package gateway

import (
	"fmt"
	"gateway/pkg/lookuptable"
	"log"
	"os"
	"os/signal"
	"regexp"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

func Gateway() {
	rootNode := &lookuptable.Node{}
	lookuptable.UpdateHost(rootNode, "/", "127.0.0.1", 5000)
	// lookuptable.UpdateHost(rootNode, "/0", "127.0.0.1", 5001)
	// lookuptable.UpdateHost(rootNode, "/1", "127.0.0.1", 5002)
	// lookuptable.UpdateHost(rootNode, "/2", "127.0.0.1", 5003)
	// lookuptable.UpdateHost(rootNode, "/3", "127.0.0.1", 5004)

	gatewayBroker := "tcp://127.0.0.1:1883"
	msgCh := make(chan mqtt.Message)
	var f mqtt.MessageHandler = func(client mqtt.Client, msg mqtt.Message) {
		msgCh <- msg
	}
	opts := mqtt.NewClientOptions()
	opts.AddBroker(gatewayBroker)
	c := mqtt.NewClient(opts)

	if token := c.Connect(); token.Wait() && token.Error() != nil {
		log.Fatalf("Mqtt error: %s", token.Error())
	}

	if subscribeToken := c.Subscribe("/api/#", 0, f); subscribeToken.Wait() && subscribeToken.Error() != nil {
		log.Fatal(subscribeToken.Error())
	}

	signalCh := make(chan os.Signal, 1)
	signal.Notify(signalCh, os.Interrupt)

	rApiRegister := regexp.MustCompile(`^/api/register$`)
	rApiUnregister := regexp.MustCompile(`^/api/unregister$`)
	rForward := regexp.MustCompile(`^/forward(/[0-3])+$`)
	for {
		select {
		case m := <-msgCh:
			fmt.Printf("topic: %v, payload: %v\n", m.Topic(), string(m.Payload()))
			// TODO: 作業ここから
			// 受信したメッセージをトピック名から、制御用メッセージと転送用メッセージに分ける。
			// その後、それぞれの処理を行う
			switch {
			case rForward.MatchString(m.Topic()):
				return
			case rApiRegister.MatchString(m.Topic()):
				return
			case rApiUnregister.MatchString(m.Topic()):
				return
			default:
				return
			}

		case <-signalCh:
			fmt.Printf("Interrupt detected.\n")
			c.Disconnect(1000)
			return
		}
	}
}
