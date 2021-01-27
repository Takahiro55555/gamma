package gateway

import (
	"encoding/json"
	"fmt"
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

type BrokerInfo struct {
	Topic string `json:"topic"`
	Host  string `json:"host"`
	Port  uint16 `json:"port"`
}

func Gateway(gatewayMB, managerMB, defaultDMB BrokerInfo) {
	startTimeUnix := time.Now().Unix()
	//////////////          Managerブローカへ接続する           //////////////
	managerBroker := fmt.Sprintf("tcp://%v:%v", managerMB.Host, managerMB.Port)
	opts := mqtt.NewClientOptions()
	opts.AddBroker(managerBroker)

	// Managerブローカへ接続
	managerClient := mqtt.NewClient(opts)
	if token := managerClient.Connect(); token.Wait() && token.Error() != nil {
		log.WithFields(log.Fields{"error": token.Error()}).Fatal("MQTT connect error")
	}
	defer managerClient.Disconnect(1000)

	//////////////        ゲートウェイブローカへ接続する         //////////////
	gatewayBroker := fmt.Sprintf("tcp://%v:%v", gatewayMB.Host, gatewayMB.Port)
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
	brokertableUpdateInfoMsgCh := make(chan mqtt.Message, 10)
	var brokertableUpdateInfoMsgFunc mqtt.MessageHandler = func(client mqtt.Client, msg mqtt.Message) {
		brokertableUpdateInfoMsgCh <- msg
	}
	if token := managerClient.Subscribe("/api/brokertable/update/info", 1, brokertableUpdateInfoMsgFunc); token.Wait() && token.Error() != nil {
		log.WithFields(log.Fields{"error": token.Error()}).Fatal("MQTT subscribe error")
	}

	// brokertable の更新作業の状態を受け取るチャンネル
	brokertableUpdateStatusMsgCh := make(chan mqtt.Message, 10)
	var brokertableUpdateStatusMsgFunc mqtt.MessageHandler = func(client mqtt.Client, msg mqtt.Message) {
		brokertableUpdateStatusMsgCh <- msg
	}
	if token := managerClient.Subscribe("/api/brokertable/update/status", 1, brokertableUpdateStatusMsgFunc); token.Wait() && token.Error() != nil {
		log.WithFields(log.Fields{"error": token.Error()}).Fatal("MQTT subscribe error")
	}

	// Gateway の担当エリア情報を受け取るチャンネル
	gatewayAreaInfoMsgCh := make(chan mqtt.Message, 10)
	var gatewayAreaInfoMsgFunc mqtt.MessageHandler = func(client mqtt.Client, msg mqtt.Message) {
		gatewayAreaInfoMsgCh <- msg
	}
	if token := managerClient.Subscribe("/api/gatewayarea", 1, gatewayAreaInfoMsgFunc); token.Wait() && token.Error() != nil {
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

	// 分散ブローカ ==> このプログラム ==> ゲートウェイブローカへ転送するためのチャンネル
	apiMsgForwardToGatewayBrokerCh := make(chan mqtt.Message, 100)
	bp := brokerpool.NewBrokerPool(0, apiMsgForwardToGatewayBrokerCh)
	defer bp.CloseAllBroker(100)

	// 分散ブローカ接続情報管理オブジェクト
	rootNode := &brokertable.Node{}
	if err := brokertable.UpdateHost(rootNode, "/", defaultDMB.Host, defaultDMB.Port); err != nil {
		log.WithFields(log.Fields{"error": err}).Fatal("Brokerpool UpdateHost error")
	}
	bp.ConnectBroker(defaultDMB.Host, defaultDMB.Port)

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

	// brokertable 更新関連の変数
	var newBrokerInfo BrokerInfo
	isUpdatedBrokerInfo := false
	for {
		select {
		// brokertable の更新情報を受け取る
		case m := <-brokertableUpdateInfoMsgCh:
			log.WithFields(log.Fields{"payload": string(m.Payload())}).Trace("brokertableUpdateInfoMsgCh")
			if isUpdatedBrokerInfo {
				// NOTE: manager がバグったりしたら全ての gateway が一斉に停止する可能性が高いことに注意
				log.WithFields(log.Fields{"msg": string(m.Payload())}).Fatal("Another brokertable update request is progressing (brokertableUpdateInfoMsgCh, AddSubsetBroker)")
			}
			// NOTE: 新たなbrokerの情報は一度に1つ分しか送信されないという前提条件
			// 以下、manager から送られてくる通知情報例
			// sample := `{"topic": "/1", "host": "localhost", "port": 1894}`

			// JSONデコード
			if err := json.Unmarshal(m.Payload(), &newBrokerInfo); err != nil {
				log.Fatal(err)
			}

			// Broker を追加・接続・Subscribe
			err := bp.AddSubsetBroker(newBrokerInfo.Host, newBrokerInfo.Port, newBrokerInfo.Topic, rootNode)
			if err != nil {
				log.WithFields(log.Fields{"topic": newBrokerInfo.Topic, "error": err}).Fatal("Brokertable Update error (brokertableUpdateInfoMsgCh, AddSubsetBroker)")
			}

			msg := fmt.Sprintf(`{"GatewayInfo":{"Host":%v,"Port":%v},"Status":"%v"}`, gatewayMB.Host, gatewayMB.Port, "ok")
			if token := managerClient.Publish("/api/brokertable/update/result", 1, false, msg); token.Wait() && token.Error() != nil {
				log.WithFields(log.Fields{"topic": newBrokerInfo.Topic, "error": err}).Fatal("Brokertable Update error (brokertableUpdateInfoMsgCh, managerClient.Publish)")
			}
			isUpdatedBrokerInfo = true
			log.WithFields(log.Fields{
				"rootNode":      fmt.Sprint(rootNode),
				"newBrokerInfo": newBrokerInfo,
			}).Info("Brokerpool Update complete (brokertableUpdateInfoMsgCh)")

		// brokertable の更新作業の状態を受け取るチャンネル
		case m := <-brokertableUpdateStatusMsgCh:
			log.WithFields(log.Fields{"payload": string(m.Payload())}).Trace("brokertableUpdateStatusMsgCh")
			if !(isUpdatedBrokerInfo && string(m.Payload()) == "complete") {
				log.WithFields(log.Fields{"isUpdatedBrokerInfo": isUpdatedBrokerInfo, "message": string(m.Payload())}).Error("Brokertable Update error (brokertableUpdateStatusMsgCh)")
				continue
			}
			hosts, err := brokertable.LookupSubsetHosts(rootNode, newBrokerInfo.Topic)
			if err != nil {
				log.WithFields(log.Fields{"topic": newBrokerInfo.Topic, "error": err}).Fatal("Brokertable Lookup error (brokertableUpdateStatusMsgCh, LookupSubsetHosts)")
			}

			// Unsubscribe
			for _, h := range hosts {
				b, err := bp.GetBroker(h.Host, h.Port)
				if err != nil {
					log.WithFields(log.Fields{"error": err}).Debug("brokerpool.GetBroker() error (brokertableUpdateStatusMsgCh)")
					continue
				}
				b.UnsubscribeSubsetTopics(newBrokerInfo.Topic)
			}

			// brokertable の更新
			err = brokertable.UpdateHost(rootNode, newBrokerInfo.Topic, newBrokerInfo.Host, newBrokerInfo.Port)
			if err != nil {
				log.WithFields(log.Fields{
					"rootNode":      fmt.Sprint(rootNode),
					"newBrokerInfo": newBrokerInfo,
					"error":         err,
				}).Fatal("Brokertable Update error (brokertableUpdateStatusMsgCh, UpdateHost)")
			}
			isUpdatedBrokerInfo = false
			log.WithFields(log.Fields{
				"rootNode":      fmt.Sprint(rootNode),
				"newBrokerInfo": newBrokerInfo,
			}).Info("Brokertable Update complete (brokertableUpdateStatusMsgCh)")

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
				log.WithFields(log.Fields{"host": host, "port": port, "error": err, "broker_table": fmt.Sprint(rootNode)}).Error("Brokerpool GetOrConnectBroker error")
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
				log.WithFields(log.Fields{"host": host, "port": port, "error": err, "broker_table": fmt.Sprint(rootNode)}).Error("Brokerpool GetOrConnectBroker error")
				continue
			}
			b.Unsubscribe(topic)

		// 分散ブローカ ==> このプログラム ==> ゲートウェイブローカへ転送する
		case m := <-apiMsgForwardToGatewayBrokerCh:
			apiMsgForwardToGatewayBrokerMetrics.Countup()
			log.WithFields(log.Fields{"topic": m.Topic(), "payload": string(m.Payload())}).Trace("apiMsgForwardToGatewayBrokerCh")
			if token := gatewayClient.Publish(m.Topic(), 0, false, m.Payload()); token.Wait() && token.Error() != nil {
				log.WithFields(log.Fields{"topic": m.Topic(), "error": token.Error()}).Error("apiMsgForwardToGatewayBrokerCh")
			}

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
				log.WithFields(log.Fields{"host": host, "port": port, "error": err, "broker_table": fmt.Sprint(rootNode)}).Error("Brokerpool GetOrConnectBroker error")
				continue
			}
			b.Publish(topic, false, m.Payload())

			// brokertable の更新作業中の場合は、新たな分散ブローカへも転送する
			if isUpdatedBrokerInfo {
				if len(topic) >= len(newBrokerInfo.Topic) {
					isMatched := true
					for i, s := range newBrokerInfo.Topic {
						if string(topic[i]) != string(s) {
							isMatched = false
							break
						}
					}
					if !isMatched {
						continue
					}
				}
				b, err := bp.GetBroker(newBrokerInfo.Host, newBrokerInfo.Port)
				if err != nil {
					log.WithFields(log.Fields{"host": host, "port": port, "error": err, "broker_table": fmt.Sprint(rootNode)}).Info("Brokerpool GetBroker error")
					continue
				}
				b.Publish(topic, false, m.Payload())
			}

		case <-metricsTicker.C:
			passedTimeSecTotal := time.Now().Unix() - startTimeUnix
			passedTimeHour := passedTimeSecTotal / 3600
			passedTimeMinutes := (passedTimeSecTotal / 60) % 60
			passedTimeSeconds := passedTimeSecTotal % 60
			log.WithFields(log.Fields{
				"hour":          passedTimeHour,
				"minutes":       passedTimeMinutes,
				"seconds":       passedTimeSeconds,
				"seconds_total": passedTimeSecTotal,
			}).Info("Total run time")
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
