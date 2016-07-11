package main

import (
	"bufio"
	"bytes"
	"io/ioutil"
	log "github.com/Sirupsen/logrus"
	"net/http"
	"net/http/httputil"
	"net/url"
)

// Item, the cached item
type Item struct {
	Key string
	Value []byte
}

// Cacher, something that can store and retrive to and from a cache
type Cacher interface {
	Get(string) (*Item, error)
	Set(*Item) error
}

type cacheTransport struct {
	// wrappedTransport: the transport we are caching
	wrappedTransport http.RoundTripper
	// cacheKey: function returning the key to cache on
	cacheKey func(*http.Request) (string, error)
	// cache: the Cache
	cache Cacher
}

func (ct *cacheTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	log.Debug("cacheTransport.RoundTrip starting")
	defer log.Debug("cacheTransport.RoundTrip finished")

	// first check the cache
	key, err := ct.cacheKey(r)
	if err != nil {
		log.Fatal("cache key error:", err)
	}
	item, err := ct.cache.Get(key)
	if err != nil {
		// there may not be a url in the request
		if r.URL == nil {
			r.URL = &url.URL{}
		}
		log.Println(
			"cache miss:",
			err,
			key,
			r.Method,
			r.URL.Host,
			r.URL.Path,
			r.URL.RawQuery,
			r.Header,
		)
	}

	// if we have an item, return that as the response
	if item != nil {
		reader := bufio.NewReader(bytes.NewReader(item.Value))
		response, err := http.ReadResponse(reader, r)
		if err != nil {
			log.Println("error reading response from cache:", err)
		} else {
			return response, nil
		}
	}

	// cache miss or cache error, gotta actually do the request
	response, err := ct.wrappedTransport.RoundTrip(r)
	if err != nil {
		log.Fatal("cacheTransport upstream error:", err)
	}
	// and make sure we cache a copy of the response but don't wait to
	// return the response
	cacheCopy := new(http.Response)
	*cacheCopy = *response
	if response.Body != nil {
		// TODO: extract this to a helper function
		bodyBytes, err := ioutil.ReadAll(response.Body)
		if err != nil {
			log.Fatal("error reading response body:", err)
		}
		cacheBytes := make([]byte, len(bodyBytes))
		copy(cacheBytes, bodyBytes)
		response.Body = ioutil.NopCloser(bytes.NewBuffer(bodyBytes))
		cacheCopy.Body = ioutil.NopCloser(bytes.NewBuffer(cacheBytes))
	}

	go func () {
		log.Debug("cacheTransport.RoundTrip setting response in cache")
		defer log.Debug("cacheTransport.RoundTrip cache set finished")
		dump, err := httputil.DumpResponse(cacheCopy, true)
		if err != nil {
			log.Fatal("could not dump response", err)
		}
		ct.cache.Set(&Item{Key: key, Value: dump})
	}()

	return response, nil
}
