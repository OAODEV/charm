package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"github.com/BurntSushi/toml"
)

type Config struct {
	Upstream string
	ReqFanFactor int
	TimeoutMS int
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
		done <- fmt.Sprintf("Could not read file at %s", confPath)
		return done
	}
	// populate the config struct
	log.Print(".   . Loading config")
	var conf Config
	_, err = toml.Decode(string(tomlData), &conf)
	if err != nil {
		done <- "Could not decode config"
		return done
	}
	// report on the configuration
	log.Print("Charm is configured!")
	log.Printf(".   . Stabilizing %s", conf.Upstream)
	log.Printf(".   . with %i duplicate requests", conf.ReqFanFactor)
	log.Printf(".   . and a %i milisecond timeout.", conf.TimeoutMS)
	return done
}

func main() {
	// start Charm,
	done := start("/secret/charm.conf")
	// when Charm is done, log the message and quit.
        log.Print(<-done)
}
