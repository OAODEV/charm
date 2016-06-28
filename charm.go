package main

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"io/ioutil"
	"log"
	"time"
	"github.com/bradfitz/gomemcache/memcache"
	"github.com/BurntSushi/toml"
)

type Config struct {
	Upstream string
	ReqFanFactor int
	TimeoutMS int
	MemcacheHosts []string
	MemcacheSeconds int
}

func snd (c chan string, s string) {
	go func () { c <- s }()
}

// start starts Charm up and returns a done channel for the done message
func start(confPath string) (chan string) {
	done := make(chan string)

	// print a welcome message
	log.Print("Charm is starting up.")
	// read the config file
	log.Printf(".   . Reading %s", confPath)
	tomlData, err := ioutil.ReadFile(confPath)
	if err != nil {
		snd(done, fmt.Sprintf("Could not read file at %s", confPath))
		return done
	}
	// populate the config struct
	log.Print(".   . Loading config")
	var conf Config
	_, err = toml.Decode(string(tomlData), &conf)
	if err != nil {
		snd(done, "Could not decode config")
		return done
	}
	// report on the configuration
	log.Print("Charm is configured!")
	log.Printf(".   . Stabilizing %v", conf.Upstream)
	log.Printf(".   . with %v duplicate requests", conf.ReqFanFactor)
	log.Printf(".   . and a %v milisecond timeout.", conf.TimeoutMS)
	log.Printf(
		".   . memcached at %v for %v seconds.",
		conf.MemcacheHosts,
		conf.MemcacheSeconds,
	)
	go run(conf, done)
	return done
}

type stableTransport struct {
	wrappedTransport http.RoundTripper
	reqFanFactor int
	cacheResponse chan *http.Response
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
	// copy the first respnse for caching
	cacheCopy := new(http.Response)
	*cacheCopy = *first
	if first.Body != nil {
		bodyBytes, err := ioutil.ReadAll(first.Body)
		if err != nil {
			log.Fatal("error reading response body:", err)
		}
		cacheBytes := make([]byte, len(bodyBytes))
		copy(cacheBytes, bodyBytes)
		first.Body = ioutil.NopCloser(bytes.NewBuffer(bodyBytes))
		cacheCopy.Body = ioutil.NopCloser(bytes.NewBuffer(cacheBytes))
	}
	// send the copy to be cached if they want it
	go func () {
		select {
		case t.cacheResponse <- cacheCopy:
			return
		case <-time.After(15 * time.Millisecond):
			return
		}
	}()
	// return the first response to the handler
	return first, nil
}

// cacheKey returns a string to be used as the cache key for a request
func cacheKey(r *http.Request) (string, error) {
	// We need to be careful here.
	// There is serious potential to accidently ignore permissions if we
	// cache requests too broadly. For example, if our cache key is the path
	// and a super-admin cache-misses on /some/restricted/path then a
	// restricted user could be given the cached result from that super
	//  admin request.

	keyStr := ""
	keyStr += r.Method
	keyStr += r.URL.Host
	keyStr += r.URL.Path
	keyStr += r.URL.RawQuery

	// TODO: extract which headers to cache on into a config option
	keyStr += r.Header["X-Forwarded-Email"][0]

	key := sha256.Sum224([]byte(keyStr))
	return hex.EncodeToString(key[:sha256.Size224]), nil
}

// Conf.ServeHTTP checks memcache then proxies/caches with a stable transport
func (conf Config) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// check memcache
	mc := memcache.New(conf.MemcacheHosts...)
	key, err := cacheKey(r)
	if err != nil {
		log.Println("cache key error for Request:", r)
	} else {
		item, err := mc.Get(key)
		if err == nil { //cache hit
			log.Println(
				"INFO: cache hit!",
				r.Method,
				r.URL.Host,
				r.URL.Path,
				r.URL.RawQuery,
				r.Header["X-Forawrded-Email"][0],
			)
			// get the cached response
			response, err := http.ReadResponse(
				bufio.NewReader(bytes.NewReader(item.Value)),
				r,
			)
			if err == nil {
				// write the response to the response writer
				response.Write(w)
				return
			}
		}
	}
	// cache miss
	log.Println(
		"INFO: cache miss!",
		r.Method,
		r.URL.Host,
		r.URL.Path,
		r.URL.RawQuery,
		r.Header["X-Forawrded-Email"][0],
	)
	upstreamURL, err := url.Parse(conf.Upstream)
	if err != nil {
		log.Fatal("error parsing Upstream URL", conf.Upstream)
	}
	responseChan := make(chan *http.Response)
	proxy := httputil.NewSingleHostReverseProxy(upstreamURL)
	proxy.Transport = &stableTransport{
		proxy.Transport,
		conf.ReqFanFactor,
		responseChan,
	}
	proxy.ServeHTTP(w, r)

	// if the transport has a response waiting on the channel, cache it if
	// we have a cache
	if cacheKey != nil {
		select {
		case resp := <- responseChan:
			log.Println("got response to cache", resp)
			dump, err := httputil.DumpResponse(resp, true)
			if err != nil {
				log.Println(
					"ERROR: couldn't dump response:",
					err,
				)
				return
			}
			item := &memcache.Item{
				Key: key,
				Value: dump,
				Expiration: int32(conf.MemcacheSeconds),
			}
			err = mc.Set(item)
			if err != nil {
				log.Println("ERROR: memcached set error:", err)
				return
			}
		case <-time.After(1 * time.Millisecond):
			return
		}
	}

}

// run
func run(conf Config, done chan string) {
	// serve the config under a timeout
	timeout := time.Duration(conf.TimeoutMS) * time.Millisecond
	log.Fatal(http.ListenAndServe(
		":8000",
		http.TimeoutHandler(conf, timeout, "upstream timeout"),
	))
}

func main() {
	// start Charm,
	done := start("/secret/charm.conf")
	// when Charm is done, log the message and quit.
        log.Print(<-done)
}
