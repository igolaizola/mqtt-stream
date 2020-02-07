package main

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/hex"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	ff "github.com/peterbourgon/ff/v2"
	log "github.com/sirupsen/logrus"
)

func main() {
	// generate random bytes to use them on default client id
	rnd := make([]byte, 6)
	if _, err := rand.Read(rnd); err != nil {
		log.Fatal(err)
	}

	// set configuration parameters from flags
	fs := flag.NewFlagSet("mqtt-stream", flag.ExitOnError)
	var cfg config
	var verbose bool
	fs.StringVar(&cfg.host, "host", "tcp://test.mosquitto.org:1883", "host mqtt broker (tcp or ws)")
	fs.StringVar(&cfg.from, "from", "bar", "topic to subscribe")
	fs.StringVar(&cfg.to, "to", "foo", "topic to publish")
	fs.StringVar(&cfg.clientID, "client-id", fmt.Sprintf("mqtt-stream-%x", rnd), "client id")
	fs.StringVar(&cfg.username, "username", "", "username")
	fs.StringVar(&cfg.password, "password", "", "password")
	fs.BoolVar(&cfg.hex, "hex", false, "enable hexadecimal input/output")
	fs.BoolVar(&cfg.echo, "echo", false, "enable echo data from topic \"from\" to topic \"to\"")
	fs.BoolVar(&verbose, "v", false, "enable verbose logs")
	if err := ff.Parse(fs, os.Args[1:],
		ff.WithConfigFileFlag("config"),
		ff.WithConfigFileParser(ff.PlainParser),
		ff.WithEnvVarPrefix("MQTT_STREAM"),
	); err != nil {
		log.Fatal(err)
	}

	if verbose {
		log.SetLevel(log.DebugLevel)
	}

	// create context and listen interruptions
	ctx, cancel := context.WithCancel(context.Background())
	exit := make(chan os.Signal, 2)
	signal.Notify(exit, os.Interrupt, syscall.SIGTERM)
	defer close(exit)
	go func() {
		<-exit
		cancel()
	}()

	// create data channel from stdin
	input := stdin(ctx)

	// launch stream client and reconnect on failure
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}
		log.Debugf("connecting to %s...", cfg.host)
		if err := stream(ctx, input, cfg); err != nil {
			log.Error(err)
			continue
		}
		return
	}
}

// config holds mqtt client parameters
type config struct {
	host     string
	from     string
	to       string
	clientID string
	username string
	password string
	hex      bool
	echo     bool
}

func stdin(ctx context.Context) <-chan []byte {
	c := make(chan []byte, 1)
	buf := bufio.NewReader(os.Stdin)
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			default:
			}
			data, _, err := buf.ReadLine()
			if err != nil {
				log.Error(fmt.Errorf("error reading stdin: %w", err))
			}
			c <- data
		}
	}()
	return c
}

// stream creates a mqtt client that reads from topic `from` and writes to
// topic `to`
func stream(ctx context.Context, input <-chan []byte, cfg config) error {
	// Create client options
	opts := mqtt.NewClientOptions()
	opts.AddBroker(cfg.host)
	opts.SetClientID(cfg.clientID)
	if cfg.username != "" {
		opts.SetUsername(cfg.username)
	}
	if cfg.password != "" {
		opts.SetPassword(cfg.password)
	}

	// Set connection callbacks
	connErr := make(chan error)
	defer close(connErr)
	opts.SetConnectionLostHandler(func(_ mqtt.Client, err error) {
		connErr <- fmt.Errorf("connection lost: %w", err)
	})
	opts.SetOnConnectHandler(func(_ mqtt.Client) {
		log.Debug("connected!")
	})

	// Connect client to broker
	client := mqtt.NewClient(opts)
	defer client.Disconnect(1000)
	if err := wait(ctx, client.Connect()); err != nil {
		return fmt.Errorf("connect failed: %w", err)
	}

	// Print format
	format := "%s"
	if cfg.hex {
		format = "%x"
	}

	// Create subscription
	if err := wait(ctx, client.Subscribe(cfg.from, 0, func(_ mqtt.Client, msg mqtt.Message) {
		data := msg.Payload()

		// Print data
		fmt.Fprintln(os.Stdout, fmt.Sprintf(format, data))

		// echo data
		if !cfg.echo {
			return
		}
		if err := wait(ctx, client.Publish(cfg.to, 0, false, data)); err != nil {
			log.Error(fmt.Errorf("publish failed: %w", err))
		}
	})); err != nil {
		return fmt.Errorf("subscribe failed: %w", err)
	}
	log.Debugf("subscribed to %s, publishing to %s", cfg.from, cfg.to)

	// Loop until finished
	for {
		select {
		case <-ctx.Done():
			return nil
		case err := <-connErr:
			return err
		case data := <-input:
			// Publish input data
			var err error
			if cfg.hex {
				data, err = hex.DecodeString(string(data))
				if err != nil {
					return err
				}
			}
			if err := wait(ctx, client.Publish(cfg.to, 0, false, data)); err != nil {
				log.Error(fmt.Errorf("publish failed: %w", err))
			}
		}
	}
}

func wait(ctx context.Context, token mqtt.Token) error {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	for !token.WaitTimeout(500 * time.Millisecond) {
		if err := ctx.Err(); err != nil {
			return err
		}
	}
	if err := token.Error(); err != nil {
		return err
	}
	return nil
}
