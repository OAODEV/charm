package main

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// start when there is no config should report that fact right away
func TestStartWithoutConfig(t *testing.T) {
	done := start("/not/notaconfig.conf")
	message := <- done
	if message != "Could not read file at /not/notaconfig.conf" {
		t.Errorf("did not report the file that was missing")
	}
}

// start when there is a bad config should report that fact right away
func TestStartWithBadConfig(t *testing.T) {
	done := start("./test_bad.conf")
	message := <- done
	if message != "Could not decode config" {
		t.Errorf("incorrect bad config mesage\n\"%s\"", message)
	}
}

// start with a valid file should not be done right away
func TestStartWithGoodConfig(t *testing.T) {
	done := start("./test_good.conf")
	select {
	case message := <-done:
		t.Errorf("done with message\n \"%s\"", message)
	case <-time.After(time.Millisecond * 50):
		fmt.Print("Stayed up with good config\n")
	}
}

// stableTransport returns a bad response if all responses are bad
func Test500IfAll500(t *testing.T) {
	// set up
	// make a servier that always responds with an error
	ts := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "It's always something", 500)
		},
	))
	// make a stableTransport fanning out to two requests
	transport := &stableTransport{
		wrappedTransport: nil,
		reqFanFactor: 2,
	}
	// run SUT
	req, err := http.NewRequest("GET", string(ts.URL), nil)
	if err != nil {
		t.Errorf("could not create request", err)
	}
	resp, err := transport.RoundTrip(req)
	if err != nil {
		t.Errorf("could not make round trip", err)
	}
	// confirm that the we get a bad response
	if resp.StatusCode != 500 {
		t.Error("expected 500 error but got", resp.StatusCode)
	}
}

// stableTransport waits for a non 5xx response
func TestNo500Responses(t *testing.T) {
	// set up
	// make a server that errors once, then responds after a delay
	i := 0
	ts := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			if i == 0 {
				http.Error(w, "fast server error", 500)
				i = 1
			} else {
				time.Sleep(1 * time.Millisecond)
				fmt.Fprintf(w, "delayed good response")
			}
		},
	))
	// make a stableTransport fanning out to two requests
	transport := &stableTransport{
		wrappedTransport: nil,
		reqFanFactor: 2,
	}
	// run SUT
	// send request to that server through that stableTransport
	req, err := http.NewRequest("GET", string(ts.URL), nil)
	if err != nil {
		t.Errorf("error making new request", err)
	}
	resp, err := transport.RoundTrip(req)
	if err != nil {
		t.Errorf("transport error", err)
	}
	// confirm that the delayed but good response is returned
	if resp.StatusCode != 200 {
		t.Errorf("expected response status 200, got: %v", resp.Status)
	}
	body, err := ioutil.ReadAll(resp.Body)
	bodyStr := string(body)
	if err != nil {
		t.Errorf("body read error", err)
	}
	if bodyStr != "delayed good response" {
		t.Errorf("expected 'delayed good response' got '%v'", bodyStr)
	}
}

// make sure Config.ServeHTTP serves http
func TestConfigIsHandler(t *testing.T) {
	// set up
	// make a test server
	ts := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprintln(w, "Hello, client")
		},
	))
	defer ts.Close()
	// make a test cache
	tcs := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			http.Error(
				w,
				"mock cache miss",
				http.StatusInternalServerError,
			)
		},
	))
	defer tcs.Close()
	// make a config with that upstream
	testConfig := &Config{
		Upstream: string(ts.URL),
		ReqFanFactor: 2,
		TimeoutMS: 1,
		MemcacheHosts: []string{string(tcs.URL)},
		CacheSeconds: 0,
	}
	// make a response writer and a request
	w := httptest.NewRecorder()
	r, err := http.NewRequest("GET", "http//upstream/api/v1/object", nil)
	if err != nil {
		t.Errorf("could not create mock request", err)
	}
	r.Header.Add("X-Forwarded-Email", "mock@email.com")
	// run SUT
	testConfig.ServeHTTP(w, r)
	// confirm upstream response is written to the response writer
	if w.Code != 200 {
		t.Errorf("expected code 200 but got", w.Code)
	}
	bodyString := w.Body.String()
	if bodyString != "Hello, client\n" {
		t.Errorf("expected 'Hello, client\n', got '%v'", bodyString)
	}
}

// TestCanPass proves that tests are running
func TestCanPass(t *testing.T) {
	if true != true {
		t.Errorf("true is not true,\ncheck your premises,\n consider clojure?")
	}
}
