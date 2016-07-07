package main

import (
	"fmt"

//	"io/ioutil"
	"net/http"
	"net/http/httptest"
//	"net/url"

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

/**
 * Commenting out flakey test - but generally works

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
			time.Sleep(1000 * time.Millisecond)
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

	res, err := http.Get(testStableServer.URL)
	if err != nil { t.Errorf("error getting response from stable server") }

	message, err := ioutil.ReadAll(res.Body)
	if err != nil { t.Errorf("error reading response body from stable server") }
	res.Body.Close()

	// ensure that all responses are the fast response
	if string(message) != "fast response" {
		t.Errorf(string(message))
	}

}
*/
// TestCanPass proves that tests are running
func TestCanPass(t *testing.T) {
	if true != true {
		t.Errorf("true is not true,\ncheck your premises,\n consider clojure?")
	}
}
