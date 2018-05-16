package router

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/hashicorp/go-multierror"

	"github.com/stretchr/testify/assert"
)

var (
	errSentinal0 = errors.New("sentinal 0")
	errSentinal1 = errors.New("sentinal 1")
)

func TestOne(t *testing.T) {
	done := make(chan struct{}, 0)
	defer close(done)

	router := NewRouter()
	defer router.Close()

	err := router.Publish(context.Background(), "a", "wow")
	assert.Equal(t, ErrNotDelivered, err)

	recv := router.Subscribe("a")
	assert.NotNil(t, recv)

	go func() {
		msg := <-recv
		assert.NotNil(t, msg)
		assert.Equal(t, msg.Obj, "wow")
		assert.NotNil(t, msg.Result)
		msg.Result <- errSentinal0
	}()

	err = router.Publish(context.Background(), "a", "wow")
	assert.Equal(t, errSentinal0, err)

	<-router.Unsubscribe("a", recv)

	err = router.Publish(context.Background(), "a", "wow")
	assert.Equal(t, ErrNotDelivered, err)

	recvX := router.Subscribe("a")
	assert.NotNil(t, recv)

	go func() {
		msg := <-recvX
		assert.NotNil(t, msg)
		assert.Equal(t, msg.Obj, "wow")
		assert.NotNil(t, msg.Result)
		msg.Result <- errSentinal0

		select {
		case _, ok := <-recvX:
			assert.False(t, ok, "channel not closed")
		case <-done:
			assert.Fail(t, "channel not closed")
		}
	}()

	recvY := router.Subscribe("a")
	assert.NotNil(t, recv)

	go func() {
		msg := <-recvY
		assert.NotNil(t, msg)
		assert.Equal(t, msg.Obj, "wow")
		assert.NotNil(t, msg.Result)
		msg.Result <- errSentinal1

		select {
		case _, ok := <-recvY:
			assert.False(t, ok, "channel not closed")
		case <-done:
			assert.Fail(t, "channel not closed")
		}
	}()

	err = router.Publish(context.Background(), "a", "wow")
	if err.Error() != multierror.Append(nil, errSentinal0, errSentinal1).Error() &&
		err.Error() != multierror.Append(nil, errSentinal1, errSentinal0).Error() {
		assert.Fail(t, "bad error")
	}

	// Out of order unsub
	<-router.Unsubscribe("a", recvY)
	<-router.Unsubscribe("a", recvX)

	recvZ := router.Subscribe("a")
	assert.NotNil(t, recv)

	go func() {
		msg := <-recvZ
		time.Sleep(100 * time.Millisecond)
		assert.NotNil(t, msg)
		assert.Equal(t, msg.Obj, "wow")
		assert.NotNil(t, msg.Result)
		msg.Result <- errSentinal1

		select {
		case _, ok := <-recvZ:
			assert.False(t, ok, "channel not closed")
		case <-done:
			assert.Fail(t, "channel not closed")
		}
	}()

	timeoutCtx, cancelFunc := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancelFunc()

	err = router.Publish(timeoutCtx, "a", "wow")
	assert.Equal(t, ErrTimedOut, err)

	// Don't unsub on shutdown
}
