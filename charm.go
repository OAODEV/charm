package main

import (
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"io/ioutil"
	"log"
	"time"
	"github.com/BurntSushi/toml"
)

type Config struct {
	Upstream string
	ReqFanFactor int
	TimeoutMS int
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
	go run(conf, done)
	return done
}

func First() {}

type stableTransport struct {
	wrappedTransport http.RoundTripper
	reqFanFactor int
}

// stableTransport.RoundTrip makes many round trips and returns the first
// response
func (t *stableTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	c := make(chan *http.Response)
	if t.wrappedTransport == nil {
		t.wrappedTransport = http.DefaultTransport
	}
	// fan out requests, send responses to the channel, log errors
	for i := 0; i < t.reqFanFactor; i++ {
		time.Sleep(5)
		go func () {
			resp, err := t.wrappedTransport.RoundTrip(r)
			if err != nil {
				// TODO: figure out how much logging we want
			} else {
				c <- resp
			}
		}()
	}

	// return the first good response
	return <-c, nil
}

type Stabilizer struct {
	upstreamURL *url.URL
	reqFanFactor int
}

// Stabilizer.ServeHTTP proxies with a stable transport
func (st *Stabilizer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	proxy := httputil.NewSingleHostReverseProxy(st.upstreamURL)
	proxy.Transport = &stableTransport{proxy.Transport, st.reqFanFactor}
	proxy.ServeHTTP(w, r)
}

// run
func run(conf Config, done chan string) {
	// make a stabilizer
	upstreamURL, err := url.Parse(conf.Upstream)
	if err != nil {
		log.Fatal("error parsing Upstream URL", conf.Upstream)
	}
	stabilizer := &Stabilizer{upstreamURL, conf.ReqFanFactor}

	// serve that stabilizer under a timeout
	timeout := time.Duration(conf.TimeoutMS) * time.Millisecond
	log.Fatal(http.ListenAndServe(
		":8000",
		http.TimeoutHandler(stabilizer, timeout, "upstream timeout"),
	))
}

func main() {
	// start Charm,
	done := start("/secret/charm.conf")
	// when Charm is done, log the message and quit.
        log.Print(<-done)
}
