# osc-mqtt-bridge
A bridge between [Open Sound Control](https://en.wikipedia.org/wiki/Open_Sound_Control) (OSC) and MQTT, allowind bidirectional communication. The main purpose of this tool is to provide a way to talk to devices that support OSC via MQTT messages for automation.

## Example configuration
```yaml
relays:
  - mqtt_host: 10.0.0.2
    mqtt_port: 1883
    mqtt_client_id: osc_mqtt_bridge
    mqtt_user: mqtt
    mqtt_password: PASSWORD
    mqtt_topic: osc/behringer_wing
    osc_host: 10.0.0.3
    osc_port: 2223
    osc_bind_addr: 0.0.0.0
    log_level: 2
```

## Configuration specification

### Relay
- `mqtt_host`: Hostname of the MQTT broker.
- `mqtt_port`: Port of the MQTT broker.
- `mqtt_client_id`: MQTT client ID of this relay.
- `mqtt_user`: User name used for MQTT authentication.
- `mqtt_password`: Password used for MQTT authentication.
- `mqtt_topic`: Topic where MQTT messages are pushed and received.

    Set topic to `osc/example` and the following topics will be setup.
    - `osc/example/cmd/$OSC_CMD` - Any commands received on OSC will publish here.
    - `osc/example/send/$OSC_CMD` - Any commands pushed via MQTT will be forwarded to OSC.
    - `osc/example/bundle` - OSC Bundle messages.
    - `osc/example/bundle/send` - Send OSC Bundle messages.
    - `osc/example/status` - Configuration is published on startup.
    - `osc/example/status/check` - Request status.
<br/><br/>

- `mqtt_disable_config_send`: Disables the config send.
- `osc_host`: Hostname for OSC client connection.
- `osc_port`: Port for OSC client connection.
- `osc_bind_addr`: Bind address of the OSC server.

    To have bidirectional mode, you must specify at least this, OscHost, and OscPort defined.

- `osc_bind_port`: Port of the OSC server. Defaults to OscPort if specified.
- `osc_disallow_arbritary_command`: Disallows pushing to arbritary commands to the cmd topic.
- `commands`: Pre-defined commands to relay.

    This is an array with the following variables.

    - `command`: The command path to send.
    - `mqtt_topic`: Absolute MQTT topic to subscribe.
    - `mqtt_sub_topic`: Sub topic off relay MQTT topic to subscribe.
        osc/example/$SUB_TOPIC
    - `disallow_payload`: Rather or not to disallow payload to be relayed.
    - `default_payload`: Payload to send if no payload is provided via MQTT or if DisallowPayload is true. This is an array of strings/integers/timestamps/bools.
<br/><br/>

- `osc_subscriptions`: OSC Comamnds to send at regular intervals. Useful for OSC servers that offers data subscriptions.

    This is an array with the following variables.

    - `command`: The command to send every interval.
    - `payload`: Payload to send. This is an array of strings/integers/timestamps/bools.
    - `interval`: How often to call the command.
<br/><br/>

- `log_level`: How much logging.

    - 0 - Errors
    - 1 - MQTT and OSC receive logging.
    - 2 - MQTT and OSC send logging.
    - 3 - Debug

## MQTT Message Example

**Mute Behringer Wing channel 1**<br/>
Topic: osc/behringer_wing/send/ch/1/mute<br/>
Payload: `["1"]`

**Behringer Wing get info**<br/>
Topic: osc/behringer_wing/send/?<br/>
Payload:

## Build
```bash
go build
```

[Golang](https://go.dev/) 1.19 and below are known to have issues, 1.20 works.

## Config file location
Same directory as the binary, in your home directory at `~/.config/osc-mqtt-bridge/config.yaml`, or under etc at `/etc/osc-mqtt-bridge/config.yaml`.

## Docker
I have made docker images for this product as I use docker for home assistant in my environment and wanted to keep with the existing scheme for services that are used with home assistant.

### Build Image
```bash
docker build --tag osc-mqtt-bridge .
```

### Run
```bash
docker run --volume ./config:/etc/osc-mqtt-bridge --publish 2223:2223/udp osc-mqtt-bridge
```

### Docker compose
```yaml
version: '2.3'

services:
  osc-mqtt-bridge:
    image: grmrgecko/osc-mqtt-bridge:latest
    restart: unless-stopped
    volumes:
      - ./config:/etc/osc-mqtt-bridge
    ports:
      - "2223:2223/udp"
```
