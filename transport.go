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
	// channel to send the fisrt good response
	rc := make(chan *http.Response)
	// channel to send and collect bad (error) responses
	ec := make(chan *http.Response)
	// use the default transport from http if not specified
	if t.wrappedTransport == nil {
		t.wrappedTransport = http.DefaultTransport
	}
	// fan out requests, send responses to the channel, log errors, don't
	// wait very long for someone to receive our response
	for i := 0; i < t.reqFanFactor; i++ {
		go func () {
			resp, err := t.wrappedTransport.RoundTrip(r)
			if err != nil {
				log.Printf("transport-error: %v", err)
				return
			}
			// don't send anything if the response is not good
			if resp.StatusCode != 200 {
				ec <- resp
				return
			}
			select {
			case rc <- resp:
				// they were still waiting for the first
				// response and they received it from c
				return
			case <-time.After(1 * time.Millisecond):
				// no one was waiting to receive from c
				// so this is not the first response
				return
			}
		}()
	}

	// wait to receive the first good response but gather error responses
	// start with a slice of responses to gather bad responses. If this has
	// as many responses in it as requests we have made, we know there is no
	// good response coming back so can respond with a summary of the bad
	// responses.
	errorResponses := make([]*http.Response, 0)
	for {
		select {
		case firstGoodResponse := <-rc:
			return firstGoodResponse, nil
		case errorResponse := <- ec:
			errorResponses = append(errorResponses, errorResponse)
			// if the length of that slice grows to ReqFanFactor
			if len(errorResponses) >= t.reqFanFactor {
				// respond with a sample of the errors.
				// TODO summarize the errors into a new response
				return errorResponses[0], nil
			}
		}
	}
}
