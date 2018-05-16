# Go Pub Sub Channels

[![Build Status](https://travis-ci.org/hpidcock/go-pub-sub-channel.svg?branch=master)](https://travis-ci.org/hpidcock/go-pub-sub-channel)

Easily send messages between multiple go routines with error response handling.

## Setup

```go
rt := router.NewRouter()
defer rt.Close()
```

## Subscribing

```go
subChan := rt.Subscribe("channel id")
for {
    select {
    case msg, ok := <-subChan:
        if ok == false {
            // Either was closed from Unsubscribe call or
            // rt.Close()
            break
        }
    }
}
```

## Publishing

```go
err := rt.Publish(context.Background(), "channel_id", "Hello World!")
if err == router.ErrNotDelivered {
    // The message didn't get delivered to any subscribers.
} else if err == router.ErrTimedOut {
    // The message may have been delivered to some subscribers,
    // but the context timed out.
} else err != nil {
    // Errors passed back by one or more subscribers.
}
```

## Unsubscribing

```go
doneChan := rt.Unsubscribe("channel_id", subChan)
// Either wait for the doneChan to close
<-doneChan
// Or wait for the subChan to close
```
