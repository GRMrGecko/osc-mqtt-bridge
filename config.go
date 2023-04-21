package main

import (
	"log"
	"os"
	"os/user"
	"path/filepath"

	"gopkg.in/yaml.v2"
)

// Configuration Structure
type Config struct {
	// Relays: Different relays available.
	Relays []*Relay `yaml:"relays"`
}

// ReadConfig Read the configuration file
func (a *App) ReadConfig() {
	usr, err := user.Current()
	if err != nil {
		log.Fatal(err)
	}

	// Configuration paths.
	localConfig, _ := filepath.Abs("./config.yaml")
	homeDirConfig := usr.HomeDir + "/.config/mqtt-osc-bridge/config.yaml"
	etcConfig := "/etc/mqtt-osc-bridge/config.yaml"

	// Determine which configuration to use.
	var configFile string
	if _, err := os.Stat(app.flags.ConfigPath); err == nil && app.flags.ConfigPath != "" {
		configFile = app.flags.ConfigPath
	} else if _, err := os.Stat(localConfig); err == nil {
		configFile = localConfig
	} else if _, err := os.Stat(homeDirConfig); err == nil {
		configFile = homeDirConfig
	} else if _, err := os.Stat(etcConfig); err == nil {
		configFile = etcConfig
	} else {
		log.Fatal("Unable to find a configuration file.")
	}

	app.config = new(Config)

	yamlFile, err := os.ReadFile(configFile)
	if err != nil {
		log.Fatalf("Error reading YAML file: %s\n", err)
	}

	err = yaml.Unmarshal(yamlFile, &app.config)
	if err != nil {
		log.Fatalf("Error parsing YAML file: %s\n", err)
	}

	if len(app.config.Relays) == 0 {
		log.Fatal("No relays defined in the configuration file.")
	}

	for _, relay := range app.config.Relays {
		if relay.OscBindAddr != "" && relay.OscBindPort == 0 {
			relay.OscBindPort = relay.OscPort
		}
	}

	for i, relay := range app.config.Relays {
		if relay.MqttHost == "" || relay.MqttPort == 0 {
			log.Fatalf("Relay %d: MQTT host and port are required configurations.", i)
		}
		if relay.MqttTopic == "" {
			log.Fatalf("Relay %d: MQTT topic is a required configuration.", i)
		}
		if relay.OscBindAddr == "" && relay.OscHost == "" {
			log.Fatalf("Relay %d: You must define either a bind address or an OSC host in the configuration.", i)
		}
		for b, relay2 := range app.config.Relays {
			if b != i {
				if relay.MqttTopic == relay2.MqttTopic {
					log.Fatalf("Relay %d: MQTT topic cannot exist on 2 different relays.", i)
				}
				if relay.OscBindPort == relay2.OscBindPort {
					log.Fatalf("Relay %d: Cannot use the same OSC bind port on 2 different relays.", i)
				}
			}
		}
	}
}
