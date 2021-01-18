package gateway

import (
	"fmt"
	"gateway/pkg/brokerpool"
	"gateway/pkg/brokertable"

	"os"
	"os/signal"
	"regexp"
	"strings"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	log "github.com/sirupsen/logrus"
)

func Entrypoint() {
	//////////////        APIブローカへ接続するための準備        //////////////
	apiBroker := "tcp://127.0.0.1:1883"
	opts := mqtt.NewClientOptions()
	opts.AddBroker(apiBroker)

	// APIブローカへ接続
	apiClient := mqtt.NewClient(opts)
	if token := apiClient.Connect(); token.Wait() && token.Error() != nil {
		log.WithFields(log.Fields{"error": token.Error()}).Fatal("MQTT connect error")
	}
	defer apiClient.Disconnect(1000)

	//////////////    ゲートウェイブローカへ接続するための準備    //////////////
	gatewayBroker := "tcp://127.0.0.1:1884"
	opts = mqtt.NewClientOptions()
	opts.AddBroker(gatewayBroker)

	// ゲートウェイブローカへ接続
	gatewayClient := mqtt.NewClient(opts)
	if token := gatewayClient.Connect(); token.Wait() && token.Error() != nil {
		log.WithFields(log.Fields{"error": token.Error()}).Fatal("MQTT connect error")
	}
	defer gatewayClient.Disconnect(1000)

	//////////////        メッセージハンドラの作成・登録         //////////////

	// Subscribe するトピックをリクエストするトピック
	apiRegisterMsgCh := make(chan mqtt.Message)
	var fRegisterMsg mqtt.MessageHandler = func(client mqtt.Client, msg mqtt.Message) {
		apiRegisterMsgCh <- msg
	}
	if token := gatewayClient.Subscribe("/api/register", 0, fRegisterMsg); token.Wait() && token.Error() != nil {
		log.WithFields(log.Fields{"error": token.Error()}).Fatal("MQTT subscribe error")
	}

	// Subscribe 解除するためのトピック
	apiUnregisterMsgCh := make(chan mqtt.Message)
	var fUnregisterMsg mqtt.MessageHandler = func(client mqtt.Client, msg mqtt.Message) {
		apiUnregisterMsgCh <- msg
	}
	if token := gatewayClient.Subscribe("/api/unregister", 0, fUnregisterMsg); token.Wait() && token.Error() != nil {
		log.WithFields(log.Fields{"error": token.Error()}).Fatal("MQTT subscribe error")
	}

	// ゲートウェイブローカ ==> このプログラム ==> 当該分散ブローカへメッセージを転送するためのトピック
	apiMsgChForwardToDistributedBroker := make(chan mqtt.Message, 10)
	var fForwardMsg mqtt.MessageHandler = func(client mqtt.Client, msg mqtt.Message) {
		apiMsgChForwardToDistributedBroker <- msg
	}
	if token := gatewayClient.Subscribe("/forward/#", 0, fForwardMsg); token.Wait() && token.Error() != nil {
		log.WithFields(log.Fields{"error": token.Error()}).Fatal("MQTT subscribe error")
	}

	// プルグラムを強制終了させるためのチャンネル
	signalCh := make(chan os.Signal, 1)
	signal.Notify(signalCh, os.Interrupt)

	////////////// 分散ブローカに関する情報を管理するオブジェクト //////////////

	// 分散ブローカ接続情報管理オブジェクト
	rootNode := &brokertable.Node{}
	brokertable.UpdateHost(rootNode, "/", "localhost", 1893)
	brokertable.UpdateHost(rootNode, "/0", "localhost", 1894)
	brokertable.UpdateHost(rootNode, "/1", "localhost", 1895)
	brokertable.UpdateHost(rootNode, "/2", "localhost", 1896)
	brokertable.UpdateHost(rootNode, "/3", "localhost", 1897)

	// 分散ブローカ ==> このプログラム ==> ゲートウェイブローカへ転送するためのチャンネル
	apiMsgChForwardToGatewayBroker := make(chan mqtt.Message, 10)
	bp := brokerpool.NewBrokerPool(0, apiMsgChForwardToGatewayBroker)
	defer bp.CloseAllBroker(100)
	for {
		select {
		// Client からの Subscribe リクエストを処理する
		case m := <-apiRegisterMsgCh:
			fmt.Println("apiRegisterMsgCh")
			topic := string(m.Payload())
			rep := regexp.MustCompile(`/#$`)
			editedTopic := rep.ReplaceAllString(topic, "")
			fmt.Println(editedTopic)
			host, port, err := brokertable.LookupHost(rootNode, editedTopic)
			if err != nil {
				log.WithFields(log.Fields{"topic": topic, "error": err}).Error("Brokertable LookupHost error")
				continue
			}
			b, err := bp.GetOrConnectBroker(host, port)
			if err != nil {
				log.WithFields(log.Fields{"host": host, "port": port, "error": err}).Error("Brokerpool GetOrConnectBroker error")
				continue
			}
			b.Subscribe(topic)

		// Client からの Unsubscribe リクエストを処理する
		case m := <-apiUnregisterMsgCh:
			fmt.Println("apiUnregisterMsgCh")
			topic := string(m.Payload())
			rep := regexp.MustCompile(`/#$`)
			editedTopic := rep.ReplaceAllString(topic, "")
			fmt.Println(editedTopic)
			host, port, err := brokertable.LookupHost(rootNode, editedTopic)
			if err != nil {
				log.WithFields(log.Fields{"topic": topic, "error": err}).Error("Brokertable LookupHost error")
				continue
			}
			b, err := bp.GetOrConnectBroker(host, port)
			if err != nil {
				log.WithFields(log.Fields{"host": host, "port": port, "error": err}).Error("Brokerpool GetOrConnectBroker error")
				continue
			}
			b.Unsubscribe(topic)

		// 分散ブローカ ==> このプログラム ==> ゲートウェイブローカへ転送する
		case m := <-apiMsgChForwardToGatewayBroker:
			fmt.Printf("topic: %v, msg: %v\n", m.Topic(), string(m.Payload()))
			gatewayClient.Publish(m.Topic(), 0, false, m.Payload())

		// ゲートウェイブローカ ==> このプログラム ==> 当該分散ブローカへ転送する
		case m := <-apiMsgChForwardToDistributedBroker:
			fmt.Printf("topic: %v, msg: %v\n", m.Topic(), string(m.Payload()))
			topic := strings.Replace(m.Topic(), "/forward", "", 1)
			host, port, err := brokertable.LookupHost(rootNode, topic)
			if err != nil {
				log.WithFields(log.Fields{"topic": topic, "error": err}).Error("Brokertable LookupHost error")
				continue
			}
			b, err := bp.GetOrConnectBroker(host, port)
			if err != nil {
				log.WithFields(log.Fields{"host": host, "port": port, "error": err}).Error("Brokerpool GetOrConnectBroker error")
				continue
			}
			b.Publish(topic, false, m.Payload())

		case <-signalCh:
			log.Info("Interrupt detected.\n")
			return
		}
	}
}
