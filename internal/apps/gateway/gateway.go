package gateway

import (
	"encoding/json"
	"fmt"
	"gateway/pkg/brokerpool"
	"gateway/pkg/brokertable"
	"gateway/pkg/metrics"
	"reflect"
	"time"

	"os"
	"os/signal"
	"strings"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	log "github.com/sirupsen/logrus"
)

type BrokerInfo struct {
	Host string `json:"host"`
	Port uint16 `json:"port"`
}

type DistributedBrokerInfo struct {
	Topic      string     `json:"topic"`
	BrokerInfo BrokerInfo `json:"broker_info"`
}

type AllDistributedBrokerInfo struct {
	Version int                     `json:"version"`
	DMBs    []DistributedBrokerInfo `json:"brokers"`
}

func Gateway(gatewayMB, managerMB BrokerInfo) {
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
	log.WithFields(log.Fields{"host": managerMB.Host, "port": managerMB.Port}).Info("Connected manager broker")

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
	log.WithFields(log.Fields{"host": gatewayMB.Host, "port": gatewayMB.Port}).Info("Connected gateway broker")

	//////////////        メッセージハンドラの作成・登録         //////////////

	// brokertable の全ての情報を受け取るチャンネル
	brokertableAllInfoMsgCh := make(chan mqtt.Message, 10)
	var brokertableAllInfoMsgFunc mqtt.MessageHandler = func(client mqtt.Client, msg mqtt.Message) {
		brokertableAllInfoMsgCh <- msg
	}
	if token := managerClient.Subscribe("/api/brokertable/all/info", 2, brokertableAllInfoMsgFunc); token.Wait() && token.Error() != nil {
		log.WithFields(log.Fields{"error": token.Error()}).Fatal("MQTT subscribe error")
	}

	// // brokertable の更新情報を受け取るチャンネル
	// brokertableUpdateInfoMsgCh := make(chan mqtt.Message, 10)
	// var brokertableUpdateInfoMsgFunc mqtt.MessageHandler = func(client mqtt.Client, msg mqtt.Message) {
	// 	brokertableUpdateInfoMsgCh <- msg
	// }
	// if token := managerClient.Subscribe("/api/brokertable/update/info", 2, brokertableUpdateInfoMsgFunc); token.Wait() && token.Error() != nil {
	// 	log.WithFields(log.Fields{"error": token.Error()}).Fatal("MQTT subscribe error")
	// }

	// brokertable の更新作業の状態を受け取るチャンネル
	brokertableUpdateStatusMsgCh := make(chan mqtt.Message, 10)
	var brokertableUpdateStatusMsgFunc mqtt.MessageHandler = func(client mqtt.Client, msg mqtt.Message) {
		brokertableUpdateStatusMsgCh <- msg
	}
	if token := managerClient.Subscribe("/api/brokertable/update/status", 2, brokertableUpdateStatusMsgFunc); token.Wait() && token.Error() != nil {
		log.WithFields(log.Fields{"error": token.Error()}).Fatal("MQTT subscribe error")
	}

	// Gateway の担当エリア情報を受け取るチャンネル
	// gatewayAreaInfoMsgCh := make(chan mqtt.Message, 10)
	// var gatewayAreaInfoMsgFunc mqtt.MessageHandler = func(client mqtt.Client, msg mqtt.Message) {
	// 	gatewayAreaInfoMsgCh <- msg
	// }
	// if token := managerClient.Subscribe("/api/gatewayarea", 2, gatewayAreaInfoMsgFunc); token.Wait() && token.Error() != nil {
	// 	log.WithFields(log.Fields{"error": token.Error()}).Fatal("MQTT subscribe error")
	// }

	// Subscribe するトピックをリクエストするトピック
	apiRegisterMsgCh := make(chan mqtt.Message, 100)
	var apiRegisterMsgFunc mqtt.MessageHandler = func(client mqtt.Client, msg mqtt.Message) {
		apiRegisterMsgCh <- msg
	}
	if token := gatewayClient.Subscribe("/api/register", 1, apiRegisterMsgFunc); token.Wait() && token.Error() != nil {
		log.WithFields(log.Fields{"error": token.Error()}).Fatal("MQTT subscribe error")
	}

	// Subscribe 解除するためのトピック
	apiUnregisterMsgCh := make(chan mqtt.Message, 100)
	var apiUnregisterMsgFunc mqtt.MessageHandler = func(client mqtt.Client, msg mqtt.Message) {
		apiUnregisterMsgCh <- msg
	}
	if token := gatewayClient.Subscribe("/api/unregister", 1, apiUnregisterMsgFunc); token.Wait() && token.Error() != nil {
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

	// 統計データを格納する変数
	apiRegisterMsgMetrics := metrics.NewMetrics("API_register_message")
	apiUnregisterMsgMetrics := metrics.NewMetrics("API_unregister_message")
	apiMsgForwardToGatewayBrokerMetrics := metrics.NewMetrics("Forward_to_gateway_broker")
	apiMsgForwardToDistributedBrokerMetrics := metrics.NewMetrics("Forward_to_distributed_broker")
	metricsTicker := time.NewTicker(time.Minute)
	metricsList := []*metrics.Metrics{
		apiRegisterMsgMetrics,
		apiUnregisterMsgMetrics,
		apiMsgForwardToGatewayBrokerMetrics,
		apiMsgForwardToDistributedBrokerMetrics,
	}

	// brokertable 更新関連の変数
	brokertableVersion := -1
	var newDistributedBrokerInfo DistributedBrokerInfo
	isUpdatedBrokerInfo := false
	isStarted := false // 分散ブローカ情報の取得が完了したかどうか
	// Manager へ自分の情報を通知する
	// TODO: 自分が死んだとき用のメッセージの設定をする(will)
	msg := fmt.Sprintf(`{"broker_info":{"host":"%v","port":%v},"status":"%v", "version": %v}`, gatewayMB.Host, gatewayMB.Port, "up", brokertableVersion)
	if token := managerClient.Publish("/api/notice/gatewaybroker", 1, false, msg); token.Wait() && token.Error() != nil {
		log.WithFields(log.Fields{"error": token.Error()}).Fatal("Notify to manager")
	}
	for {
		select {
		// brokertable の全ての情報を受け取るチャンネル
		case m := <-brokertableAllInfoMsgCh:
			// JSONデコード
			var allDistributedBroker AllDistributedBrokerInfo
			if err := json.Unmarshal(m.Payload(), &allDistributedBroker); err != nil {
				log.WithFields(log.Fields{"err": err}).Fatal("Init gateway (brokertableAllInfoMsgCh)")
			}
			brokertableInfo := allDistributedBroker.DMBs
			brokertableVersion = allDistributedBroker.Version

			if isStarted {
				for _, info := range brokertableInfo {
					_, err := bp.GetBroker(info.BrokerInfo.Host, info.BrokerInfo.Port)
					if err != nil && reflect.ValueOf(err).Type() == reflect.ValueOf(brokerpool.NotFoundError{}).Type() {
						newDistributedBrokerInfo = info
						isUpdatedBrokerInfo = true
						break
					} else if err != nil {
						log.WithFields(log.Fields{"err": err}).Fatal("Init gateway (brokertableAllInfoMsgCh)")
					}
				}
				if !isUpdatedBrokerInfo {
					continue
				}
				// Broker を追加・接続・Subscribe
				err := bp.AddSubsetBroker(newDistributedBrokerInfo.BrokerInfo.Host, newDistributedBrokerInfo.BrokerInfo.Port, newDistributedBrokerInfo.Topic, rootNode)
				if err != nil {
					log.WithFields(log.Fields{"topic": newDistributedBrokerInfo.Topic, "error": err}).Fatal("Brokertable Update error (brokertableUpdateInfoMsgCh, AddSubsetBroker)")
				}
				msg := fmt.Sprintf(`{"broker_info":{"host":"%v","port":%v},"status":"%v", "version": %v}`, gatewayMB.Host, gatewayMB.Port, "complete", brokertableVersion)
				if token := managerClient.Publish("/api/notice/gatewaybroker", 1, false, msg); token.Wait() && token.Error() != nil {
					log.WithFields(log.Fields{"error": token.Error()}).Fatal("Notify to manager")
				}
				log.WithFields(log.Fields{
					"rootNode":                 fmt.Sprint(rootNode),
					"newDistributedBrokerInfo": newDistributedBrokerInfo,
				}).Info("Brokerpool Update complete (brokertableUpdateInfoMsgCh)")
				continue
			}

			// NOTE: マネージャーから送られてくるデータは完全に不都合のないものという前提
			if len(brokertableInfo) == 0 {
				log.WithFields(log.Fields{"msg": string(m.Payload())}).Error("Invalid message")
				continue
			}
			info := brokertableInfo[0]
			err := brokertable.UpdateHost(rootNode, info.Topic, info.BrokerInfo.Host, info.BrokerInfo.Port)
			if err != nil {
				log.WithFields(log.Fields{"topic": info.Topic, "error": err}).Fatal("Brokertable Update error (brokertableAllInfoMsgCh, Add root distributed broker)")
			}
			bp.ConnectBroker(info.BrokerInfo.Host, info.BrokerInfo.Port)

			// brokertable, brokerpool の更新作業を一度に行う
			for _, info := range brokertableInfo[1:] {
				// Broker を追加・接続・Subscribe
				err := bp.AddSubsetBroker(info.BrokerInfo.Host, info.BrokerInfo.Port, info.Topic, rootNode)
				if err != nil {
					log.WithFields(log.Fields{"topic": info.Topic, "error": err}).Fatal("Brokertable Update error (brokertableAllInfoMsgCh, AddSubsetBroker)")
				}
				hosts, err := brokertable.LookupSubsetHosts(rootNode, info.Topic)
				if err != nil {
					log.WithFields(log.Fields{"topic": info.Topic, "error": err}).Fatal("Brokertable Lookup error (brokertableAllInfoMsgCh, LookupSubsetHosts)")
				}
				// Unsubscribe
				for _, h := range hosts {
					b, err := bp.GetBroker(h.Host, h.Port)
					if err != nil {
						log.WithFields(log.Fields{"error": err}).Debug("brokerpool.GetBroker() error (brokertableAllInfoMsgCh)")
						continue
					}
					b.UnsubscribeSubsetTopics(info.Topic)
				}
				// brokertable の更新
				err = brokertable.UpdateHost(rootNode, info.Topic, info.BrokerInfo.Host, info.BrokerInfo.Port)
				if err != nil {
					log.WithFields(log.Fields{
						"rootNode": fmt.Sprint(rootNode),
						"info":     info,
						"error":    err,
					}).Fatal("Brokertable Update error (brokertableAllInfoMsgCh, UpdateHost)")
				}
			}
			msg := fmt.Sprintf(`{"broker_info":{"host":"%v","port":%v},"status":"%v", "version": %v}`, gatewayMB.Host, gatewayMB.Port, "complete", brokertableVersion)
			if token := managerClient.Publish("/api/notice/gatewaybroker", 1, false, msg); token.Wait() && token.Error() != nil {
				log.WithFields(log.Fields{"error": token.Error()}).Fatal("Notify to manager")
			}
			isStarted = true
			log.Info("Gateway started!!!!")

		// brokertable の更新作業の状態を受け取るチャンネル
		case m := <-brokertableUpdateStatusMsgCh:
			log.WithFields(log.Fields{"payload": string(m.Payload())}).Trace("brokertableUpdateStatusMsgCh")
			if !isStarted {
				log.WithFields(log.Fields{"payload": string(m.Payload())}).Info("Ignored this message (not complete start sequence)")
				continue
			}
			if !isUpdatedBrokerInfo {
				log.WithFields(log.Fields{"isUpdatedBrokerInfo": isUpdatedBrokerInfo, "message": string(m.Payload())}).Info("There is no updating info...")
				continue
			}
			if string(m.Payload()) != "complete" {
				log.WithFields(log.Fields{"isUpdatedBrokerInfo": isUpdatedBrokerInfo, "message": string(m.Payload())}).Error("Brokertable Update error (brokertableUpdateStatusMsgCh)")
				continue
			}
			hosts, err := brokertable.LookupSubsetHosts(rootNode, newDistributedBrokerInfo.Topic)
			if err != nil {
				log.WithFields(log.Fields{"topic": newDistributedBrokerInfo.Topic, "error": err}).Fatal("Brokertable Lookup error (brokertableUpdateStatusMsgCh, LookupSubsetHosts)")
			}

			// Unsubscribe
			for _, h := range hosts {
				b, err := bp.GetBroker(h.Host, h.Port)
				if err != nil {
					log.WithFields(log.Fields{"error": err}).Debug("brokerpool.GetBroker() error (brokertableUpdateStatusMsgCh)")
					continue
				}
				b.UnsubscribeSubsetTopics(newDistributedBrokerInfo.Topic)
			}

			// brokertable の更新
			err = brokertable.UpdateHost(rootNode, newDistributedBrokerInfo.Topic, newDistributedBrokerInfo.BrokerInfo.Host, newDistributedBrokerInfo.BrokerInfo.Port)
			if err != nil {
				log.WithFields(log.Fields{
					"rootNode":                 fmt.Sprint(rootNode),
					"newDistributedBrokerInfo": newDistributedBrokerInfo,
					"error":                    err,
				}).Fatal("Brokertable Update error (brokertableUpdateStatusMsgCh, UpdateHost)")
			}
			isUpdatedBrokerInfo = false
			log.WithFields(log.Fields{
				"rootNode":                 fmt.Sprint(rootNode),
				"newDistributedBrokerInfo": newDistributedBrokerInfo,
			}).Info("Brokertable Update complete (brokertableUpdateStatusMsgCh)")

		// // Gateway の担当エリア情報を受け取る
		// case m := <-gatewayAreaInfoMsgCh:
		// 	log.WithFields(log.Fields{"payload": string(m.Payload())}).Trace("gatewayAreaInfoMsgCh")

		// Client からの Subscribe リクエストを処理する
		case m := <-apiRegisterMsgCh:
			apiRegisterMsgMetrics.Countup()
			if !isStarted {
				log.WithFields(log.Fields{"payload": string(m.Payload())}).Info("Ignored this message (not complete start sequence)")
				continue
			}
			topic := string(m.Payload())
			editedTopic := strings.Replace(topic, "/#", "", 1)
			log.WithFields(log.Fields{"topic": editedTopic}).Trace("apiRegisterMsgCh")
			host, port, err := brokertable.LookupHost(rootNode, editedTopic)
			if err != nil {
				log.WithFields(log.Fields{"topic": topic, "error": err}).Error("Brokertable LookupHost error")
				continue
			}
			b, err := bp.GetBroker(host, port)
			if err != nil {
				log.WithFields(log.Fields{"host": host, "port": port, "error": err, "broker_table": fmt.Sprint(rootNode)}).Error("Brokerpool GetBroker error")
				continue
			}
			b.Subscribe(topic)

		// Client からの Unsubscribe リクエストを処理する
		case m := <-apiUnregisterMsgCh:
			topic := string(m.Payload())
			if !isStarted {
				log.WithFields(log.Fields{"payload": string(m.Payload())}).Info("Ignored this message (not complete start sequence)")
				continue
			}
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
			if !isStarted {
				log.WithFields(log.Fields{"payload": string(m.Payload())}).Info("Ignored this message (not complete start sequence)")
				continue
			}
			log.WithFields(log.Fields{"topic": m.Topic(), "payload": string(m.Payload())}).Trace("apiMsgForwardToGatewayBrokerCh")
			if token := gatewayClient.Publish(m.Topic(), 0, false, m.Payload()); token.Wait() && token.Error() != nil {
				log.WithFields(log.Fields{"topic": m.Topic(), "error": token.Error()}).Error("apiMsgForwardToGatewayBrokerCh")
			}

		// ゲートウェイブローカ ==> このプログラム ==> 当該分散ブローカへ転送する
		case m := <-apiMsgForwardToDistributedBrokerCh:
			apiMsgForwardToDistributedBrokerMetrics.Countup()
			if !isStarted {
				log.WithFields(log.Fields{"payload": string(m.Payload())}).Info("Ignored this message (not complete start sequence)")
				continue
			}
			log.WithFields(log.Fields{"topic": m.Topic(), "payload": string(m.Payload())}).Trace("apiMsgForwardToDistributedBrokerCh")
			topic := strings.Replace(m.Topic(), "/forward", "", 1)
			host, port, err := brokertable.LookupHost(rootNode, topic)
			if err != nil {
				log.WithFields(log.Fields{"topic": topic, "error": err}).Error("Brokertable LookupHost error")
				continue
			}
			b, err := bp.GetBroker(host, port)
			if err != nil {
				log.WithFields(log.Fields{"host": host, "port": port, "error": err, "broker_table": fmt.Sprint(rootNode)}).Error("Brokerpool GetBroker error")
				continue
			}
			b.Publish(topic, false, m.Payload())

			// brokertable の更新作業中の場合は、新たな分散ブローカへも転送する
			if isUpdatedBrokerInfo {
				if len(topic) >= len(newDistributedBrokerInfo.Topic) {
					isMatched := true
					for i, s := range newDistributedBrokerInfo.Topic {
						if string(topic[i]) != string(s) {
							isMatched = false
							break
						}
					}
					if !isMatched {
						continue
					}
				}
				b, err := bp.GetBroker(newDistributedBrokerInfo.BrokerInfo.Host, newDistributedBrokerInfo.BrokerInfo.Port)
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
