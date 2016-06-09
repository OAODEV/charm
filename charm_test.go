package main

import "testing"

// TestCanPass proves that tests are running
func TestCanPass(t *testing.T) {
	if true != true {
		t.Errorf("true is not true, consider clojure?")
	}
}
