package main

import (
	"log"
	"net/http"
	"time"
)

type stableTransport struct {
	// wrappedTransport: the transport we are stabilizing
	wrappedTransport http.RoundTripper
	// reqFanFactor: how many times to duplicate the request
	reqFanFactor int
}

// stableTransport.RoundTrip makes many round trips and returns the first
// response
func (t *stableTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	c := make(chan *http.Response)
	if t.wrappedTransport == nil {
		t.wrappedTransport = http.DefaultTransport
	}
	// fan out requests, send responses to the channel, log errors, don't
	// wait very long for someone to recieve our response
	for i := 0; i < t.reqFanFactor; i++ {
		go func () {
			resp, err := t.wrappedTransport.RoundTrip(r)
			if err != nil {
				log.Printf("transport-error: %v", err)
			} else {
				select {
				case c <- resp:
					// they were still waiting for the first
					// response and they recieved it from c
					return
				case <-time.After(1 * time.Millisecond):
					// no one was waiting to recieve from c
					// so this is not the first response
					return
				}
			}
		}()
	}

	// wait to reviece the first response
	first := <-c
	return first, nil
}
