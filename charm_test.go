package main

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"net/url"
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

// Stabilizer.ServeHTTP should return the first good response
func TestStabilizerReturnsFirstResponse(t *testing.T) {
	// mock handler that first errors, then takes a long time then returns a
	// a good response
	reqCount := 0
	mockHandler := func(w http.ResponseWriter, r *http.Request) {
		reqCount++
		if reqCount % 3 == 1 {
			http.Error(w, "test error", 1234567890)
		}
		if reqCount % 3 == 2 {
			time.Sleep(100 * time.Millisecond)
			fmt.Fprintf(w, "slow response")
		}
		if reqCount % 3 == 0 {
			fmt.Fprintf(w, "fast response")
		}
	}

	mockUnstableBackend := httptest.NewServer(http.HandlerFunc(mockHandler))
	defer mockUnstableBackend.Close()
	u, err := url.Parse(mockUnstableBackend.URL)
	if err != nil { t.Errorf("error parsing backend url test broken") }
	testStabilizer := &Stabilizer{u, 4}
	testStableServer := httptest.NewServer(
		http.TimeoutHandler(testStabilizer, 5 * time.Second, "timeout"),
	)
	defer testStableServer.Close()

	// make many requests and make sure they are all the fast response
	for i := 0; i < 50; i++ {
		res, err := http.Get(testStableServer.URL)
		if err != nil { t.Errorf("error response from stable server") }

		message, err := ioutil.ReadAll(res.Body)
		if err != nil { t.Errorf("error reading response body from stable server") }
		res.Body.Close()

		// ensure that all responses are the fast response
		if string(message) != "fast response" {
			t.Errorf(string(message))
		}
	}
}

// TestCanPass proves that tests are running
func TestCanPass(t *testing.T) {
	if true != true {
		t.Errorf("true is not true,\ncheck your premises,\n consider clojure?")
	}
}
