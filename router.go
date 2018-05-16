package router

import (
	"context"
	"errors"

	"github.com/hashicorp/go-multierror"
)

type pub struct {
	channelID string
	obj       interface{}
	result    chan<- error
	ctx       context.Context
}

type sub struct {
	channelID string
	ch        chan Message
	done      chan<- struct{}
}

type unsub struct {
	channelID string
	ch        chan Message
	done      chan<- struct{}
}

type Router struct {
	channels  map[string][]chan Message
	pubChan   chan pub
	subChan   chan sub
	unsubChan chan unsub
	closeChan chan chan struct{}
}

type Message struct {
	Obj    interface{}
	Result chan<- error
}

var (
	ErrTimedOut     = errors.New("timed out")
	ErrNotDelivered = errors.New("not delivered")
)

func NewRouter() *Router {
	router := &Router{
		channels:  make(map[string][]chan Message),
		pubChan:   make(chan pub, 1000),
		subChan:   make(chan sub, 0),
		unsubChan: make(chan unsub, 0),
		closeChan: make(chan chan struct{}, 0),
	}

	go router.run()

	return router
}

func (router *Router) waitFor(msg pub, errChan chan error, count int) {
	defer close(errChan)

	var resErr error
	for i := 0; i < count; i++ {
		select {
		case <-msg.ctx.Done():
			msg.result <- ErrTimedOut
			return
		case err := <-errChan:
			if err != nil {
				resErr = multierror.Append(resErr, err)
			}
		}
	}

	if resErr != nil {
		msg.result <- multierror.Flatten(resErr)
	} else {
		msg.result <- nil
	}
}

func (router *Router) run() {
exit:
	for {
		select {
		case msg := <-router.pubChan:
			channel, ok := router.channels[msg.channelID]
			if ok && len(channel) > 1 {
				count := len(channel)
				errChan := make(chan error, count)
				for _, channelSub := range channel {
					channelSub <- Message{
						Obj:    msg.obj,
						Result: errChan,
					}
				}
				go router.waitFor(msg, errChan, count)
			} else if ok && len(channel) == 1 {
				channel[0] <- Message{
					Obj:    msg.obj,
					Result: msg.result,
				}
			} else {
				msg.result <- ErrNotDelivered
			}
		case msg := <-router.subChan:
			channel, _ := router.channels[msg.channelID]
			router.channels[msg.channelID] = append(channel, msg.ch)
			close(msg.done)
		case msg := <-router.unsubChan:
			channel, ok := router.channels[msg.channelID]
			if ok {
				for i, ch := range channel {
					if ch != msg.ch {
						continue
					}
					lastIdx := len(channel) - 1
					channel[i] = channel[lastIdx]
					channel[lastIdx] = nil
					router.channels[msg.channelID] = channel[:lastIdx]
					break
				}
			}
			close(msg.ch)
			close(msg.done)
		case closed := <-router.closeChan:
			defer close(closed)
			break exit
		}
	}

	// TODO: Handle pub,sub,unsub chan shutdown with a semaphore

	// Close subscribers
	for _, channel := range router.channels {
		for _, ch := range channel {
			close(ch)
		}
	}
	router.channels = nil
}

func (router *Router) Close() {
	closed := make(chan struct{})
	router.closeChan <- closed
	<-closed
}

func (router *Router) Publish(ctx context.Context, channelID string, obj interface{}) error {
	result := make(chan error, 1)
	defer close(result)

	router.pubChan <- pub{
		channelID: channelID,
		obj:       obj,
		result:    result,
		ctx:       ctx,
	}

	select {
	case err := <-result:
		return err
	case <-ctx.Done():
		return ErrTimedOut
	}
}

func (router *Router) Subscribe(channelID string) chan Message {
	subChan := make(chan Message, 10)
	done := make(chan struct{}, 0)
	router.subChan <- sub{
		channelID: channelID,
		ch:        subChan,
		done:      done,
	}
	<-done
	return subChan
}

func (router *Router) Unsubscribe(channelID string, ch chan Message) chan struct{} {
	done := make(chan struct{}, 0)
	router.unsubChan <- unsub{
		channelID: channelID,
		ch:        ch,
		done:      done,
	}

	return done
}
