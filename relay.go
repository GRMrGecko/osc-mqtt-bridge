package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"strings"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/hypebeast/go-osc/osc"
)

// LogLevel Definition
type LogLevel int

const (
	// ErrorLog Logs only errors.
	ErrorLog LogLevel = iota
	// ReceiveLog MQTT and OSC receive logging.
	ReceiveLog
	// SendLog MQTT and OSC send logging.
	SendLog
	// DebugLog Debug messages.
	DebugLog
)

// String: Provides a string value for a log level.
func (l LogLevel) String() string {
	return [...]string{"Error", "Receive", "Send", "Debug"}[l]
}

// Relay command definition
type RelayCommand struct {
	// Command: The command path to send.
	Command string `yaml:"command" json:"command"`
	// MqttTopic: Absolute MQTT topic to subscribe.
	MqttTopic string `yaml:"mqtt_topic" json:"mqtt_topic"`
	// MqttSubTopic: Sub topic off relay MQTT topic to subscribe.
	// osc/example/$SUB_TOPIC
	MqttSubTopic string `yaml:"mqtt_sub_topic" json:"mqtt_sub_topic"`
	// DisallowPayload: Rather or not to disallow payload to be relayed.
	DisallowPayload bool `yaml:"disallow_payload" json:"disallow_payload"`
	// DefaultPayload: Payload to send if no payload is provided via MQTT or if DisallowPayload is true.
	DefaultPayload []interface{} `yaml:"default_payload" json:"default_payload"`
}

// Relay OSC subscription
type RelayOscSubscription struct {
	// Command: The command to send every interval.
	Command string `yaml:"command" json:"command"`
	// Payload: Payload to send.
	Payload []interface{} `yaml:"payload" json:"payload"`
	// Interval: How often to call the command.
	Interval time.Duration `yaml:"interval" json:"interval"`
}

// Relay configurations
type Relay struct {
	// MqttHost: Hostname of the MQTT broker.
	MqttHost string `yaml:"mqtt_host" json:"mqtt_host"`
	// MqttPort: Port of the MQTT broker.
	MqttPort int `yaml:"mqtt_port" json:"mqtt_port"`
	// MqttClientId: MQTT client ID of this relay.
	MqttClientId string `yaml:"mqtt_client_id" json:"mqtt_client_id"`
	// MqttUser: User name used for MQTT authentication.
	MqttUser string `yaml:"mqtt_user" json:"mqtt_user"`
	// MqttPassword: Password used for MQTT authentication.
	MqttPassword string `yaml:"mqtt_password" json:"mqtt_password"`
	// MqttTopic: Topic where MQTT messages are pushed and received.
	// Set topic to `osc/example` and the following topics will be setup.
	// osc/example/cmd/$OSC_CMD - Any commands received on OSC will publish here.
	// osc/example/send/$OSC_CMD - Any commands pushed via MQTT will be forwarded to OSC.
	// osc/example/bundle - OSC Bundle messages.
	// osc/example/bundle/send - Send OSC Bundle messages.
	// osc/example/status - Configuration is published on startup.
	// osc/example/status/check - Request status.
	MqttTopic string `yaml:"mqtt_topic" json:"mqtt_topic"`
	// MqttDisableConfigSend: Disables the config send.
	MqttDisableConfigSend bool `yaml:"mqtt_disable_config_send" json:"mqtt_disable_config_send"`

	// OscHost: Hostname for OSC client connection.
	OscHost string `yaml:"osc_host" json:"osc_host"`
	// OscPort: Port for OSC client connection.
	OscPort int `yaml:"osc_port" json:"osc_port"`
	// OscBindAddr: Bind address of the OSC server.
	// To have bidirectional mode, you must specify at least this, OscHost, and OscPort defined.
	// You must specify the unicast IP address, cannot be 0.0.0.0.
	OscBindAddr string `yaml:"osc_bind_addr" json:"osc_bind_addr"`
	// OscBindPort: Port of the OSC server. Defaults to OscPort if specified.
	OscBindPort int `yaml:"osc_bind_port" json:"osc_bind_port"`
	// OscDisallowArbritaryCommand: Disallows pushing to arbritary commands to the cmd topic.
	OscDisallowArbritaryCommand bool `yaml:"osc_disallow_arbritary_command" json:"osc_disallow_arbritary_command"`

	// RelayCommands: Pre-defined commands to relay.
	Commands []RelayCommand `yaml:"relay_commands" json:"commands"`
	// RelayOscSubscriptions: OSC Comamnds to send at regular intervals. Useful for OSC servers that offers data subscriptions.
	OscSubscriptions []RelayOscSubscription `yaml:"osc_subscriptions" json:"osc_subscriptions"`

	// LogLevel: How much logging.
	// 0 - Errors
	// 1 - MQTT and OSC receive logging.
	// 2 - MQTT and OSC send logging.
	// 3 - Debug
	LogLevel LogLevel `yaml:"log_level" json:"log_level"`

	// MqttClient: The client connection to MQTT.
	MqttClient mqtt.Client `yaml:"-" json:"-"`
	// OscClient: The client connection to OSC.
	OscClient *osc.Client `yaml:"-" json:"-"`
	// OscServer: OSC Server.
	OscServer *osc.Server `yaml:"-" json:"-"`
	// OscServerConn: Server connection.
	// The OSC software is limited in bidirectional support, so I do my own connection here.
	OscServerConn net.PacketConn `yaml:"-" json:"-"`
}

// OscMessage: Used for json encode/decode to/from MQTT for bundles.
type OscMessage struct {
	Address   string        `json:"address"`
	Arguments []interface{} `json:"arguments"`
}

// OscBundle: Used for json encode/decode to/from MQTT.
type OscBundle struct {
	Timetag  time.Time     `json:"timetag"`
	Messages []*OscMessage `json:"messages"`
	Bundles  []*OscBundle  `json:"bundles"`
}

// OscDispatcher: Handles OSC messages.
type OscDispatcher struct {
	r *Relay
}

// makeBundle: Makes an OscBundle from an osc.Bundle.
func (d OscDispatcher) makeBundle(bundle *osc.Bundle) *OscBundle {
	b := new(OscBundle)
	b.Timetag = bundle.Timetag.Time()
	for _, message := range bundle.Messages {
		m := new(OscMessage)
		m.Address = message.Address
		m.Arguments = message.Arguments
		b.Messages = append(b.Messages, m)
	}

	for _, sbundle := range bundle.Bundles {
		subBundle := d.makeBundle(sbundle)
		b.Bundles = append(b.Bundles, subBundle)
	}
	return b
}

// Dispatch: Handle OSC packet.
func (d OscDispatcher) Dispatch(packet osc.Packet) {
	// Determine packet type and process.
	if packet != nil {
		switch packet.(type) {
		default:
			d.r.Log(ErrorLog, "Unknown OSC packet received.")

		// Message packets can just go to /cmd/$OSC_CMD and arguments encoded to JSON.
		case *osc.Message:
			message := packet.(*osc.Message)
			d.r.Log(ReceiveLog, "<- [OSC] %s: %s", message.Address, message.Arguments)
			topic := d.r.MqttTopic + "/cmd" + message.Address
			data, err := json.Marshal(message.Arguments)
			if err != nil {
				d.r.Log(ErrorLog, "Json Encode: %s", err)
				return
			}
			d.r.MqttClient.Publish(topic, 0, true, data)
			d.r.Log(SendLog, "-> [MQTT] %s: %s", topic, data)

		// Bundle packets are capable of having multiple messages and bundles embeded in it,
		//  so I translate to my own bundle structure that is JSON aware.
		case *osc.Bundle:
			b := d.makeBundle(packet.(*osc.Bundle))
			d.r.Log(ReceiveLog, "<- [OSC] Bundle %s", b.Timetag)
			topic := d.r.MqttTopic + "/bundle"
			data, err := json.Marshal(b)
			if err != nil {
				d.r.Log(ErrorLog, "Json Encode: %s", err)
				return
			}
			d.r.MqttClient.Publish(topic, 0, true, data)
			d.r.Log(SendLog, "-> [MQTT] %s: %s", topic, data)
		}
	}
}

// OscSend: Sends an OSC packet. I use my own function to allow bidirectional communication.
func (r *Relay) OscSend(packet osc.Packet) error {
	// Do not send nil packets.
	if packet == nil {
		return nil
	}

	// Log send request.
	switch packet.(type) {
	default:
	case *osc.Message:
		message := packet.(*osc.Message)
		r.Log(SendLog, "-> [OSC] %s: %s", message.Address, message.Arguments)
	case *osc.Bundle:
		bundle := packet.(*osc.Bundle)
		r.Log(SendLog, "-> [OSC] Bundle %s", bundle.Timetag.Time())
	}

	// Hosts can be DNS names, or IP addresses, so we need to resolve.
	var err error
	addr, err := net.ResolveUDPAddr("udp", fmt.Sprintf("%s:%d", r.OscHost, r.OscPort))
	if err != nil {
		return err
	}

	// Convert packet to OSC bytes.
	data, err := packet.MarshalBinary()
	if err != nil {
		return err
	}
	if r.LogLevel >= DebugLog {
		r.Log(DebugLog, "-> [OSC] Binary %s", bytes.ReplaceAll(data, []byte{byte(0)}, []byte("~")))
	}

	// If we have an OSC Server defined, we use its connection to write the data for bidirectional support.
	if r.OscServer != nil {
		_, err = r.OscServerConn.WriteTo(data, addr)
	} else {
		// Otherwise, we dial the address with a unused source port.
		// Specifying a manual source port could end up with conflicts.
		conn, err := net.DialUDP("udp", nil, addr)
		if err != nil {
			return err
		}
		defer conn.Close()

		_, err = conn.Write(data)
	}

	return err
}

// SendStatus: Send config to MQTT status.
func (r *Relay) SendStatus() {
	// If disabled, ignore.
	if r.MqttDisableConfigSend {
		return
	}

	// Make JSON dump.
	config, err := json.Marshal(&r)
	if err != nil {
		r.Log(ErrorLog, "Json Error: %s", err)
	}

	// Send config.
	r.MqttClient.Publish(r.MqttTopic+"/status", 0, true, config)
}

// MakeOSCBundle: Makes an osc.Bundle. from an OscBundle.
func (r *Relay) MakeOSCBundle(bundle *OscBundle) *osc.Bundle {
	b := osc.NewBundle(bundle.Timetag)

	// Add attached messages.
	for _, message := range bundle.Messages {
		m := osc.NewMessage(message.Address)
		m.Arguments = message.Arguments
		b.Append(m)
	}

	// Add sub bundles.
	for _, sbundle := range bundle.Bundles {
		subBundle := r.MakeOSCBundle(sbundle)
		b.Append(subBundle)
	}
	return b
}

// MqttOnEvent: Handle MQTT events.
func (r *Relay) MqttOnEvent(client mqtt.Client, message mqtt.Message) {
	r.Log(ReceiveLog, "<- [MQTT] %s: %s\n", message.Topic(), message.Payload())

	// Check commands to see if one matches this topic.
	for _, cmd := range r.Commands {
		if message.Topic() == cmd.MqttTopic ||
			(cmd.MqttSubTopic != "" && message.Topic() == r.MqttTopic+"/"+cmd.MqttSubTopic) {
			// Configure OSC message.
			oscMessage := osc.NewMessage(cmd.Command)

			// If arguments allowed and provided, parse, otherwise use default payload.
			var arguments []interface{}
			if !cmd.DisallowPayload && len(message.Payload()) != 0 {
				err := json.Unmarshal(message.Payload(), &arguments)
				if err != nil {
					r.Log(ErrorLog, "Json Error: %s", err)
					return
				}
			} else if len(cmd.DefaultPayload) != 0 {
				arguments = cmd.DefaultPayload
			}
			oscMessage.Arguments = arguments

			// Send OSC message.
			err := r.OscSend(oscMessage)
			if err != nil {
				r.Log(ErrorLog, "Send Error: %s", err)
			}
		}
	}

	// If standard send topic.
	if strings.HasPrefix(message.Topic(), r.MqttTopic+"/send") {
		// Verify arbritary commands can be sent.
		if r.OscDisallowArbritaryCommand {
			r.Log(ErrorLog, "Arbritary commands are disabled on this relay.")
			return
		}

		// Get the command from topic.
		cmd := strings.Replace(message.Topic(), r.MqttTopic+"/send", "", 1)
		if cmd == "" {
			cmd = "/"
		}

		// Parse the arguments.
		var arguments []interface{}
		if len(message.Payload()) != 0 {
			err := json.Unmarshal(message.Payload(), &arguments)
			if err != nil {
				r.Log(ErrorLog, "Json Error: %s", err)
				return
			}
		}

		// Create OSC message.
		oscMessage := osc.NewMessage(cmd)
		oscMessage.Arguments = arguments

		// Send OSC message.
		err := r.OscSend(oscMessage)
		if err != nil {
			r.Log(ErrorLog, "Send Error: %s", err)
		}
	} else if message.Topic() == r.MqttTopic+"/bundle/send" {
		// Verify arbritary commands can be sent.
		if r.OscDisallowArbritaryCommand {
			r.Log(ErrorLog, "Arbritary commands are disabled on this relay.")
			return
		}

		// Create bundle.
		bundle := new(OscBundle)
		err := json.Unmarshal(message.Payload(), bundle)
		if err != nil {
			r.Log(ErrorLog, "Json Error: %s", err)
			return
		}

		// Make the OSC bundle based on received bundle.
		b := r.MakeOSCBundle(bundle)

		// Send OSC bundle.
		err = r.OscSend(b)
		if err != nil {
			r.Log(ErrorLog, "Send Error: %s", err)
		}
	} else if message.Topic() == r.MqttTopic+"/status/check" {
		r.SendStatus()
	}
}

// MqttSubscribe: Subscribe to MQTT Topic.
func (r *Relay) MqttSubscribe(topic string) {
	r.Log(DebugLog, "Subscribing MQTT: %s", topic)
	if t := r.MqttClient.Subscribe(topic, 0, r.MqttOnEvent); t.Wait() && t.Error() != nil {
		r.Log(ErrorLog, "MQTT Subscribe Error: %s", t.Error())
	}
}

// Log: Logging function to allow log levels.
func (r *Relay) Log(level LogLevel, format string, args ...interface{}) {
	if level <= r.LogLevel {
		log.Println(fmt.Sprintf(format, args...))
	}
}

// Start: Start the relay.
func (r *Relay) Start() {
	// Connect to MQTT.
	mqtt_opts := mqtt.NewClientOptions()
	mqtt_opts.AddBroker(fmt.Sprintf("tcp://%s:%d", r.MqttHost, r.MqttPort))
	mqtt_opts.SetClientID(r.MqttClientId)
	mqtt_opts.SetUsername(r.MqttUser)
	mqtt_opts.SetPassword(r.MqttPassword)
	r.MqttClient = mqtt.NewClient(mqtt_opts)

	// Connect and failures are fatal exiting service.
	r.Log(DebugLog, "Connecting to MQTT")
	if t := r.MqttClient.Connect(); t.Wait() && t.Error() != nil {
		log.Fatalf("MQTT error: %s", t.Error())
		return
	}

	// Subscribe to MQTT topics.
	r.MqttSubscribe(r.MqttTopic + "/send/#")
	r.MqttSubscribe(r.MqttTopic + "/bundle/send")
	r.MqttSubscribe(r.MqttTopic + "/status/check")
	// Subscribe to command topics configured.
	for _, cmd := range r.Commands {
		if cmd.MqttTopic != "" {
			r.MqttSubscribe(cmd.MqttTopic)
		}
		if cmd.MqttSubTopic != "" {
			r.MqttSubscribe(r.MqttTopic + "/" + cmd.MqttSubTopic)
		}
	}

	// If an OSC client configuration is provided, setup client.
	if r.OscHost != "" && r.OscPort != 0 {
		r.OscClient = osc.NewClient(r.OscHost, r.OscPort)
	}

	// If OSC server configured, setup server.
	if r.OscBindAddr != "" && r.OscBindPort != 0 {
		r.OscServer = &osc.Server{Addr: fmt.Sprintf("%s:%d", r.OscBindAddr, r.OscBindPort), Dispatcher: OscDispatcher{r: r}}

		// Run server in thread.
		go func() {
			r.Log(DebugLog, "Starting OSC Server")
			var err error
			// I setup our own UDP connection to overcome a limit in go-osc
			//  where bidirectional isn't built in.
			r.OscServerConn, err = net.ListenPacket("udp", r.OscServer.Addr)
			if err != nil {
				log.Fatal(err)
			}

			// Close connection when function ends.
			defer r.OscServerConn.Close()

			// Have Go-OSC handle OSC traffic on this connection.
			if err = r.OscServer.Serve(r.OscServerConn); err != nil {
				log.Fatal(err)
			}
		}()
	}

	// Setup subscriptions.
	for _, subcription := range r.OscSubscriptions {
		// Each subscription runs in its own thread.
		go func(subcription RelayOscSubscription) {
			r.Log(DebugLog, "Started subscription: %s", subcription.Command)
			ticker := time.NewTicker(subcription.Interval)
			for range ticker.C {
				// Send OSC message as configured.
				r.Log(DebugLog, "Running subscription: %s", subcription.Command)
				oscMessage := osc.NewMessage(subcription.Command)
				oscMessage.Arguments = subcription.Payload
				err := r.OscSend(oscMessage)
				if err != nil {
					r.Log(ErrorLog, "Send Error: %s", err)
				}
			}
		}(subcription)
	}

	// Send current config to MQTT.
	r.SendStatus()
}
