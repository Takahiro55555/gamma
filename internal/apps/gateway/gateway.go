package gateway

import (
	"gateway/pkg/brokerpool"
	"gateway/pkg/brokertable"
	"gateway/pkg/metrics"
	"time"

	"os"
	"os/signal"
	"strings"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	log "github.com/sirupsen/logrus"
)

func Gateway() {
	//////////////          Managerブローカへ接続する           //////////////
	managerBroker := "tcp://127.0.0.1:1883"
	opts := mqtt.NewClientOptions()
	opts.AddBroker(managerBroker)

	// Managerブローカへ接続
	managerClient := mqtt.NewClient(opts)
	if token := managerClient.Connect(); token.Wait() && token.Error() != nil {
		log.WithFields(log.Fields{"error": token.Error()}).Fatal("MQTT connect error")
	}
	defer managerClient.Disconnect(1000)

	//////////////        ゲートウェイブローカへ接続する         //////////////
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

	// brokertable の更新情報を受け取るチャンネル
	brokertableUpdateMsgCh := make(chan mqtt.Message, 10)
	var brokertableUpdateMsgFunc mqtt.MessageHandler = func(client mqtt.Client, msg mqtt.Message) {
		brokertableUpdateMsgCh <- msg
	}
	if token := managerClient.Subscribe("/api/brokertable", 1, brokertableUpdateMsgFunc); token.Wait() && token.Error() != nil {
		log.WithFields(log.Fields{"error": token.Error()}).Fatal("MQTT subscribe error")
	}

	// Gateway の担当エリア情報を受け取るチャンネル
	gatewayAreaInfoMsgCh := make(chan mqtt.Message, 10)
	var gatewayAreaInfoMsgFunc mqtt.MessageHandler = func(client mqtt.Client, msg mqtt.Message) {
		gatewayAreaInfoMsgCh <- msg
	}
	if token := managerClient.Subscribe("/api/brokertable", 1, gatewayAreaInfoMsgFunc); token.Wait() && token.Error() != nil {
		log.WithFields(log.Fields{"error": token.Error()}).Fatal("MQTT subscribe error")
	}

	// Subscribe するトピックをリクエストするトピック
	apiRegisterMsgCh := make(chan mqtt.Message, 100)
	var apiRegisterMsgFunc mqtt.MessageHandler = func(client mqtt.Client, msg mqtt.Message) {
		apiRegisterMsgCh <- msg
	}
	if token := gatewayClient.Subscribe("/api/register", 0, apiRegisterMsgFunc); token.Wait() && token.Error() != nil {
		log.WithFields(log.Fields{"error": token.Error()}).Fatal("MQTT subscribe error")
	}

	// Subscribe 解除するためのトピック
	apiUnregisterMsgCh := make(chan mqtt.Message, 100)
	var apiUnregisterMsgFunc mqtt.MessageHandler = func(client mqtt.Client, msg mqtt.Message) {
		apiUnregisterMsgCh <- msg
	}
	if token := gatewayClient.Subscribe("/api/unregister", 0, apiUnregisterMsgFunc); token.Wait() && token.Error() != nil {
		log.WithFields(log.Fields{"error": token.Error()}).Fatal("MQTT subscribe error")
	}

	// ゲートウェイブローカ ==> このプログラム ==> 当該分散ブローカへメッセージを転送するためのトピック
	apiMsgForwardToDistributedBrokerCh := make(chan mqtt.Message, 100)
	var apiForwardMsgFunc mqtt.MessageHandler = func(client mqtt.Client, msg mqtt.Message) {
		apiMsgForwardToDistributedBrokerCh <- msg
	}
	if token := gatewayClient.Subscribe("/forward/#", 0, apiForwardMsgFunc); token.Wait() && token.Error() != nil {
		log.WithFields(log.Fields{"error": token.Error()}).Fatal("MQTT subscribe error")
	}

	// プルグラムを強制終了させるためのチャンネル
	signalCh := make(chan os.Signal, 1)
	signal.Notify(signalCh, os.Interrupt)

	////////////// 分散ブローカに関する情報を管理するオブジェクト //////////////

	// 分散ブローカ接続情報管理オブジェクト
	rootNode := &brokertable.Node{}
	brokertable.UpdateHost(rootNode, "/", "localhost", 1893)
	brokertable.UpdateHost(rootNode, "/1/2/2/3/3/2/0/0", "localhost", 1894)
	brokertable.UpdateHost(rootNode, "/1/2/2/3/3/2/0/1", "localhost", 1895)
	brokertable.UpdateHost(rootNode, "/1/2/2/3/3/2/0/2", "localhost", 1896)
	brokertable.UpdateHost(rootNode, "/1/2/2/3/3/2/0/3", "localhost", 1897)

	// 分散ブローカ ==> このプログラム ==> ゲートウェイブローカへ転送するためのチャンネル
	apiMsgForwardToGatewayBrokerCh := make(chan mqtt.Message, 100)
	bp := brokerpool.NewBrokerPool(0, apiMsgForwardToGatewayBrokerCh)
	defer bp.CloseAllBroker(100)

	// 統計データを格納する変数
	apiRegisterMsgMetrics := metrics.NewMetrics("API_register_message")
	apiUnregisterMsgMetrics := metrics.NewMetrics("API_unregister_message")
	apiMsgForwardToGatewayBrokerMetrics := metrics.NewMetrics("Forward_to_gateway_broker")
	apiMsgForwardToDistributedBrokerMetrics := metrics.NewMetrics("Forward_to_distributed_broker")
	metricsTicker := time.NewTicker(time.Second)
	metricsList := []*metrics.Metrics{
		apiRegisterMsgMetrics,
		apiUnregisterMsgMetrics,
		apiMsgForwardToGatewayBrokerMetrics,
		apiMsgForwardToDistributedBrokerMetrics,
	}
	for {
		select {
		// brokertable の更新情報を受け取る
		case m := <-brokertableUpdateMsgCh:
			log.WithFields(log.Fields{"payload": string(m.Payload())}).Trace("brokertableUpdateMsgCh")

		// Gateway の担当エリア情報を受け取る
		case m := <-gatewayAreaInfoMsgCh:
			log.WithFields(log.Fields{"payload": string(m.Payload())}).Trace("gatewayAreaInfoMsgCh")

		// Client からの Subscribe リクエストを処理する
		case m := <-apiRegisterMsgCh:
			apiRegisterMsgMetrics.Countup()
			topic := string(m.Payload())
			editedTopic := strings.Replace(topic, "/#", "", 1)
			log.WithFields(log.Fields{"topic": editedTopic}).Trace("apiRegisterMsgCh")
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
			topic := string(m.Payload())
			editedTopic := strings.Replace(topic, "/#", "", 1)
			log.WithFields(log.Fields{"topic": editedTopic}).Trace("apiUnregisterMsgCh")
			apiUnregisterMsgMetrics.Countup()
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
		case m := <-apiMsgForwardToGatewayBrokerCh:
			apiMsgForwardToGatewayBrokerMetrics.Countup()
			log.WithFields(log.Fields{"topic": m.Topic(), "payload": string(m.Payload())}).Trace("apiMsgForwardToGatewayBrokerCh")
			gatewayClient.Publish(m.Topic(), 0, false, m.Payload())

		// ゲートウェイブローカ ==> このプログラム ==> 当該分散ブローカへ転送する
		case m := <-apiMsgForwardToDistributedBrokerCh:
			log.WithFields(log.Fields{"topic": m.Topic(), "payload": string(m.Payload())}).Trace("apiMsgForwardToDistributedBrokerCh")
			apiMsgForwardToDistributedBrokerMetrics.Countup()
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

		case <-metricsTicker.C:
			log.Trace("metricsTicker.C")
			for _, m := range metricsList {
				ok, rate, name := m.GetRate()
				if ok {
					log.WithFields(log.Fields{"rate": rate, "name": name}).Info("Metrics")
				} else {
					log.WithFields(log.Fields{"rate": nil, "name": name}).Debug("Metrics cannot get")
				}
			}

		case <-signalCh:
			log.Info("Interrupt detected.\n")
			return
		}
	}
}
