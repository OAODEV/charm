package main

import (
	"bytes"
	"errors"
	"io/ioutil"
	"net/http"
	"net/http/httputil"
	"testing"
	"time"
)

type constCache struct {
	resp []byte
}

func (cc *constCache) Get (k string) (*Item, error) {
	return &Item{Key: k, Value: cc.resp}, nil
}

func (cc *constCache) Set (*Item) error {
	return nil
}

type constTransport struct {
	respBody string
}

func (ct *constTransport) RoundTrip (r *http.Request) (*http.Response, error) {
	return &http.Response{
		Status: "200 OK",
		StatusCode: 200,
		Body: ioutil.NopCloser(bytes.NewBufferString(ct.respBody)),
	}, nil
}

// cacheTransport returns cached results if available
func TestCacheTransportCachedResponse(t *testing.T) {
	// set up
	cachedResp := &http.Response{
		Status: "200 OK",
		StatusCode: 200,
		Body: ioutil.NopCloser(bytes.NewBufferString(
			"test cached body")),
	}
	dump, err := httputil.DumpResponse(cachedResp, true)
	if err != nil {
		t.Errorf("could not dump response:", err)
	}
	testTransport := &cacheTransport{
		wrappedTransport: &constTransport{"test upstream body"},
		cacheKey: func (r *http.Request) (string, error) {
			return "test key", nil
		},
		cache: &constCache{dump},
	}

	// run SUT
	resp, err := testTransport.RoundTrip(&http.Request{})

	// confirm assumptions
	if err != nil {
		t.Errorf("test transport error:", err)
	}
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		t.Errorf("could not read body:", err)
	}
	if string(body) != "test cached body" {
		t.Errorf("expected cached response, got", string(body))
	}
}

type emptyCache struct {
	latestItem *Item
}

func (ec *emptyCache) Get (k string) (*Item, error) {
	return nil, errors.New("empty cache has no items")
}

func (ec *emptyCache) Set (i *Item) error {
	ec.latestItem = i
	return nil
}

// cacheTransport returns upstream result if no cache is available
func TestCacheTransportFreshResponse(t *testing.T) {
	// set up
	mockCache := &emptyCache{}
	mockTransport := &constTransport{"test upstream body"}
	testTransport := &cacheTransport{
		wrappedTransport: mockTransport,
		cacheKey: func (r *http.Request) (string, error) {
			return "test key", nil
		},
		cache: mockCache,
	}

	// run SUT
	resp, err := testTransport.RoundTrip(&http.Request{})

	// confirm assumptions
	if err != nil {
		t.Errorf("test transport error:", err)
	}

	// response is the upstream response (from the wrapped transport)
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		t.Errorf("could not read body:", err)
	}
	if string(body) != "test upstream body" {
		t.Errorf("expected cached response, got", string(body))
	}

	// the cache was set to the correct item
	response, err := mockTransport.RoundTrip(&http.Request{})
	if err != nil {
		t.Errorf("mock transport RoundTrip error", err)
	}
	expectedValue, err := httputil.DumpResponse(response, true)
	if err != nil {
		t.Errorf("could not dump response:", err)
	}

	// cache is set async so we should wait some reasonable time for it.
	time.Sleep(1 * time.Millisecond)
	if !bytes.Equal(mockCache.latestItem.Value, expectedValue) {
		t.Errorf("\nexpected %v\nset to %v",
			string(expectedValue),
		        string(mockCache.latestItem.Value),
		)
	}
}
