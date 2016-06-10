package main

import "testing"


// start when there is no config should report that fact right away
func TestStartWithoutConfig(t *testing.T) {
	done := start("/not/notaconfig.conf")
	message := <- done
	if message == "First Message?" {
		t.Errorf("start did not report missing config")
	}
	if message != "Could not read file at /not/notaconfig.conf" {
		t.Errorf("did not report the file that was missing")
	}
}

// start when ther is a bad config should report that fact right away
func TestStartWithBadConfig(t *testing.T) {
	done := start("./test-bad.conf")
	message := <- done
	if message == "First Message?" {
		t.Errorf("start did not report bad config")
	}
	if message != "Could not decode config" {
		t.Errorf("incorrect bad config message")
	}
}

// TestCanPass proves that tests are running
func TestCanPass(t *testing.T) {
	if true != true {
		t.Errorf("true is not true, consider clojure?")
	}
}
