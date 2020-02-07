[![GitHub release](https://img.shields.io/github/release/igolaizola/mqtt-stream.svg)](https://github.com/igolaizola/mqtt-stream/releases)
[![Go Report Card](https://goreportcard.com/badge/igolaizola/mqtt-stream)](http://goreportcard.com/report/igolaizola/mqtt-stream)
[![license](https://img.shields.io/github/license/igolaizola/mqtt-stream.svg)](https://github.com/igolaizola/mqtt-stream/blob/master/LICENSE.md)

# mqtt-stream

MQTT client that creates a data stream using two topics

 - Data from topic `from` goes to `stdout`.
 - Data from `stdin` goes topic `to`.
 - If `echo` is enabled: data from topic `from` goes to topic `to`.

## Usage

```
mqtt-stream --host tcp://test.mosquitto.org:1883 --from bar --to foo
```

Launch `mqtt-stream --help` to see all available parameters.

## Echo example using mosquitto

Create a subscription to `foo` topic using `mosquitto_sub`

```
mosquitto_sub -h test.mosquitto.org -t bar
```

Launch `mqtt-stream` with echo enabled

```
mqtt-stream --host tcp://test.mosquitto.org:1883 --from bar --to foo --echo
```

Publish some data to topic `bar` using `mosquitto_pub`

```
mosquitto_pub -h test.mosquitto.org -t bar -m "hello world"
```

Output

```
$ mosquitto_sub -h test.mosquitto.org -t foo
hello world
```

```
$ mqtt-stream --host tcp://test.mosquitto.org:1883 --from bar --to foo
2020/02/05 13:51:45 connecting to tcp://test.mosquitto.org:1883...
2020/02/05 13:51:45 connected!
2020/02/05 13:51:45 subscribed to bar, publishing to foo
2020/02/05 13:51:46 hello world
```