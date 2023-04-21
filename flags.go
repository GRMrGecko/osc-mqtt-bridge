package main

import (
	"flag"
	"fmt"
	"os"
)

// Flags Configuration options for cli execution
type Flags struct {
	ConfigPath string
}

// InitFlags Parses configuration options
func (a *App) InitFlags() {
	app.flags = new(Flags)
	flag.Usage = func() {
		fmt.Printf(serviceName + ": " + serviceDescription + ".\n\nUsage:\n")
		flag.PrintDefaults()
	}

	var printVersion bool
	flag.BoolVar(&printVersion, "v", false, "Print version")

	usage := "Load configuration from `FILE`"
	flag.StringVar(&app.flags.ConfigPath, "config", "", usage)
	flag.StringVar(&app.flags.ConfigPath, "c", "", usage+" (shorthand)")

	flag.Parse()

	if printVersion {
		fmt.Println(serviceName + ": " + serviceVersion)
		os.Exit(0)
	}
}
