package main

import (
	_ "net/http/pprof"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/ioutil"
	log "github.com/Sirupsen/logrus"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"runtime"
	"time"
	"github.com/bradfitz/gomemcache/memcache"
	"github.com/BurntSushi/toml"
)

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

type memcachedCache struct {
	client *memcache.Client
	expiration int32
}

// memcacheCache.Get simply forwards our request to the gomemcache lib
func (mc *memcachedCache) Get(key string) (*Item, error) {
	memcacheItem, err := mc.client.Get(key)
	if err != nil {
		return nil, err
	}
	item := &Item{Key: memcacheItem.Key, Value: memcacheItem.Value}
	return item, nil
}

// memcacheCache.Get adds the Expiration to the item and sets it with gomemcache
func (mc *memcachedCache) Set(item *Item) error {
	return mc.client.Set(&memcache.Item{
		Key: item.Key,
		Value: item.Value,
		Expiration: mc.expiration,
	})
}

type Config struct {
	Upstream string
	ReqFanFactor int
	TimeoutMS int
	MemcacheHosts []string
	CacheSeconds int
}

// Conf.ServeHTTP checks memcache then proxies/caches with a stable transport
func (conf Config) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	log.Debug("Config.ServeHTTP started")
	defer log.Debug("Config.ServeHTTP finished")

	upstreamURL, err := url.Parse(conf.Upstream)
	if err != nil {
		log.Fatal("error parsing Upstream URL", conf.Upstream)
	}
	proxy := httputil.NewSingleHostReverseProxy(upstreamURL)
	// TODO: make transports more elegantly composable
	stabalizedTransport := &stableTransport{
		wrappedTransport: proxy.Transport,
		reqFanFactor: conf.ReqFanFactor,
	}
	cache := &memcachedCache{
		client: memcache.New(conf.MemcacheHosts...),
		expiration: int32(conf.CacheSeconds),
	}
	proxy.Transport = &cacheTransport{
		wrappedTransport: stabalizedTransport,
		cacheKey: cacheKey,
		cache: cache,
	}
	proxy.ServeHTTP(w, r)
	log.Println(r.Method, r.URL)
}

// run
func run(conf Config, done chan string) {
	// serve the config under a timeout
	timeout := time.Duration(conf.TimeoutMS) * time.Millisecond
	// use the default serve mux so we get pprof endpoints
	timeoutHandler := http.TimeoutHandler(conf, timeout, "upstream timeout")
	http.Handle("/", timeoutHandler)
	log.Fatal(http.ListenAndServe(":8000", nil))
}

// TODO refactor this. There is potential for accidental leaks here
func snd (c chan string, s string) {
	go func () {
		log.Debug("charm.go snd started goroutine")
		defer log.Debug("charm.go snd ending gorouting")
		c <- s
	}()
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
		conf.CacheSeconds,
	)
	go run(conf, done)
	return done
}

func main() {
	// configure logging
	switch os.Getenv("CHARM_LOG_LEVEL") {
	case "Debug":
		log.SetLevel(log.DebugLevel)
	case "Info":
		log.SetLevel(log.InfoLevel)
	case "Warn":
		log.SetLevel(log.WarnLevel)
	case "Error":
		log.SetLevel(log.ErrorLevel)
	case "Fatal":
		log.SetLevel(log.FatalLevel)
	case "Panic":
		log.SetLevel(log.PanicLevel)
	default:
		log.SetLevel(log.WarnLevel)

	}

	// start Charm,
	done := start("/secret/charm.conf")
	stop := make(chan bool)
	// start some logging of the number of goroutines
	// TODO: make this goroutine logging more flexible
	go func() {
		log.Debug(
			"Charm is currently using",
			runtime.NumGoroutine(),
			"goroutines.",
		)
		for {
			select {
			case <-time.After(1 * time.Minute):
				log.Debug(
					"Charm is currently using",
					runtime.NumGoroutine(),
					"goroutines.",
				)
			case <-stop:
				return
			}
		}
	}()
	// when Charm is done, log the message and quit.
        log.Print(<-done)
	close(stop)
}
