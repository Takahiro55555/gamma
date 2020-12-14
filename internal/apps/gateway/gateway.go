package gateway

import (
	"fmt"
	"gateway/pkg/brokerpool"
	"gateway/pkg/brokertable"
	"log"
	"os"
	"os/signal"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

func Gateway() {
	////// ゲートウェイブローカへ接続するための準備 //////
	gatewayBroker := "tcp://127.0.0.1:1883"
	opts := mqtt.NewClientOptions()
	opts.AddBroker(gatewayBroker)

	// ゲートウェイブローカへ接続
	gatewayClient := mqtt.NewClient(opts)
	if token := gatewayClient.Connect(); token.Wait() && token.Error() != nil {
		log.Fatalf("Mqtt error: %s", token.Error())
		return
	}

	////// メッセージハンドラの作成・登録 //////

	// Subscribe するトピックをリクエストするトピック
	apiRegisterMsgCh := make(chan mqtt.Message)
	var fRegisterMsg mqtt.MessageHandler = func(client mqtt.Client, msg mqtt.Message) {
		apiRegisterMsgCh <- msg
	}
	if subscribeToken := gatewayClient.Subscribe("/api/register", 0, fRegisterMsg); subscribeToken.Wait() && subscribeToken.Error() != nil {
		log.Fatal(subscribeToken.Error())
		return
	}

	// Subscribe 解除するためのトピック
	apiUnregisterMsgCh := make(chan mqtt.Message)
	var fUnregisterMsg mqtt.MessageHandler = func(client mqtt.Client, msg mqtt.Message) {
		apiUnregisterMsgCh <- msg
	}
	if subscribeToken := gatewayClient.Subscribe("/api/register", 0, fUnregisterMsg); subscribeToken.Wait() && subscribeToken.Error() != nil {
		log.Fatal(subscribeToken.Error())
		return
	}

	// プルグラムを強制終了させるためのチャンネル
	signalCh := make(chan os.Signal, 1)
	signal.Notify(signalCh, os.Interrupt)

	////// 分散ブローカに関する情報を管理するオブジェクト //////

	// ゲートウェイブローカへ転送するメッセージ用チャンネル
	apiForwardMsgCh := make(chan mqtt.Message)

	// 分散ブローカ接続情報管理オブジェクト
	rootNode := &brokertable.Node{}
	brokertable.UpdateHost(rootNode, "/", "localhost", 1893)
	brokertable.UpdateHost(rootNode, "/0", "localhost", 1894)
	brokertable.UpdateHost(rootNode, "/1", "localhost", 1895)
	brokertable.UpdateHost(rootNode, "/2", "localhost", 1896)
	brokertable.UpdateHost(rootNode, "/3", "localhost", 1897)

	bp := brokerpool.NewBrokerPool()
	_ = bp
	_ = apiForwardMsgCh
	for {
		select {
		case m := <-apiRegisterMsgCh:
			fmt.Printf("topic: %v, payload: %v\n", m.Topic(), string(m.Payload()))

		case m := <-apiUnregisterMsgCh:
			fmt.Printf("topic: %v, payload: %v\n", m.Topic(), string(m.Payload()))

		case <-signalCh:
			fmt.Printf("Interrupt detected.\n")
			gatewayClient.Disconnect(1000)
			return
		}
	}
}
