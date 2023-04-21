package main

import (
	"log"
	"os"
	"path/filepath"
)

const serviceName = "osc-mqtt-bridge"
const serviceDescription = "Bridges MQTT messages to OSC"
const serviceVersion = "0.1"

// App is the global application structure for communicating between servers and storing information.
type App struct {
	flags  *Flags
	config *Config
}

var app *App

func main() {
	thisPath, err := os.Executable()
	if err != nil {
		log.Panic(err)
	}
	os.Chdir(filepath.Dir(thisPath))

	app = new(App)
	app.InitFlags()
	app.ReadConfig()

	for _, relay := range app.config.Relays {
		relay.Start()
	}

	for {
	}
}
