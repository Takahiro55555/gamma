package manager

import (
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"sort"
	"time"

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

type GatewayBrokerInfoSingleTopic struct {
	Topic      string     `json:"topic"`
	BrokerInfo BrokerInfo `json:"broker_info"`
}
type GatewayBrokerInfo struct {
	Topics     []string   `json:"topic"`
	BrokerInfo BrokerInfo `json:"broker_info"`
}

type GatewayBrokerStatus struct {
	Status     string     `json:"status"`
	Version    int        `json:"version"`
	BrokerInfo BrokerInfo `json:"broker_info"`
}

func Manager(client mqtt.Client) {
	startTimeUnix := time.Now().Unix()

	// プルグラムを強制通知を受け取るためのチャンネル
	signalCh := make(chan os.Signal, 1)
	signal.Notify(signalCh, os.Interrupt)

	// 1秒おきに統計情報などをログに出力するためのタイマ
	// metricsTicker := time.NewTicker(time.Second)

	// Gatewayの分散ブローカ情報追加結果を受け取るチャンネル
	// brokertableUpdateResultMsgCh := make(chan mqtt.Message, 10)
	// var brokertableUpdateResultMsgFunc mqtt.MessageHandler = func(client mqtt.Client, msg mqtt.Message) {
	// 	brokertableUpdateResultMsgCh <- msg
	// }
	// if token := client.Subscribe("/api/brokertable/update/result", 2, brokertableUpdateResultMsgFunc); token.Wait() && token.Error() != nil {
	// 	log.WithFields(log.Fields{"error": token.Error()}).Fatal("MQTT subscribe error")
	// }

	// Gatewayの状態通知を受取るチャンネル
	gatewayNotifyMsgCh := make(chan mqtt.Message, 10)
	var gatewayNotifyMsgFunc mqtt.MessageHandler = func(client mqtt.Client, msg mqtt.Message) {
		gatewayNotifyMsgCh <- msg
	}
	if token := client.Subscribe("/api/notice/gatewaybroker", 2, gatewayNotifyMsgFunc); token.Wait() && token.Error() != nil {
		log.WithFields(log.Fields{"error": token.Error()}).Fatal("MQTT subscribe error")
	}

	// ゲートウェイブローカの担当エリア設定リクエストを受取るチャンネル
	setGatewayBrokerMsgCh := make(chan mqtt.Message, 10)
	var setGatewayBrokerMsgFunc mqtt.MessageHandler = func(client mqtt.Client, msg mqtt.Message) {
		setGatewayBrokerMsgCh <- msg
	}
	if token := client.Subscribe("/api/tool/gatewaybroker/set", 1, setGatewayBrokerMsgFunc); token.Wait() && token.Error() != nil {
		log.WithFields(log.Fields{"error": token.Error()}).Fatal("MQTT subscribe error")
	}

	// 分散ブローカの追加リクエストを受取るチャンネル
	addDistributedBrokerMsgCh := make(chan mqtt.Message, 10)
	var addDistributedBrokerMsgFunc mqtt.MessageHandler = func(client mqtt.Client, msg mqtt.Message) {
		addDistributedBrokerMsgCh <- msg
	}
	if token := client.Subscribe("/api/tool/distributedbroker/add", 1, addDistributedBrokerMsgFunc); token.Wait() && token.Error() != nil {
		log.WithFields(log.Fields{"error": token.Error()}).Fatal("MQTT subscribe error")
	}

	// 分散ブローカ情報更新の際に使用する
	gatewayCoverAreaInfo := map[string]*GatewayBrokerInfo{}
	gatewayStatusMap := map[string]GatewayBrokerStatus{}
	allDistributedBrokerList := AllDistributedBrokerInfo{Version: -1, DMBs: []DistributedBrokerInfo{}}
	isUpdatingDistributedBrokerList := false
	metricsTrigger := make(chan bool, 10)
	for {
		select {
		// Gatewayの状態通知を受取るチャンネル
		case m := <-gatewayNotifyMsgCh:
			metricsTrigger <- true
			log.WithFields(log.Fields{"payload": string(m.Payload())}).Trace("gatewayNotifyMsgCh")
			// JSONデコード
			var gatewayStatus GatewayBrokerStatus
			if err := json.Unmarshal(m.Payload(), &gatewayStatus); err != nil {
				log.WithFields(log.Fields{"err": err}).Fatal("add gateway broker (gatewayNotifyMsgCh)")
			}
			key := fmt.Sprintf("%v-%v", gatewayStatus.BrokerInfo.Host, gatewayStatus.BrokerInfo.Port)
			if _, ok := gatewayCoverAreaInfo[key]; !ok {
				gatewayCoverAreaInfo[key] = &GatewayBrokerInfo{Topics: []string{"/"}, BrokerInfo: gatewayStatus.BrokerInfo}
			}
			gatewayStatusMap[key] = gatewayStatus
			if gatewayStatus.Status == "up" {
				var payload []GatewayBrokerInfoSingleTopic
				for _, v := range gatewayCoverAreaInfo {
					for _, t := range v.Topics {
						var gatewayCoverArea = GatewayBrokerInfoSingleTopic{Topic: t, BrokerInfo: BrokerInfo{Host: v.BrokerInfo.Host, Port: v.BrokerInfo.Port}}
						payload = append(payload, gatewayCoverArea)
					}
				}
				msg, err := json.Marshal(payload)
				if err != nil {
					log.WithFields(log.Fields{"err": err}).Fatal("add gateway broker (gatewayNotifyMsgCh)")
				}
				if token := client.Publish("/api/gateway/info/all", 2, true, msg); token.Wait() && token.Error() != nil {
					log.WithFields(log.Fields{"error": token.Error()}).Fatal("Notify to manager")
				}
			}

			// 全ての Gateway の状態が completeかどうかを確かめる
			for _, info := range gatewayStatusMap {
				isUpdatingDistributedBrokerList = false
				if info.Version != allDistributedBrokerList.Version {
					isUpdatingDistributedBrokerList = true
					break
				}
			}
			if !isUpdatingDistributedBrokerList {
				if token := client.Publish("/api/brokertable/update/status", 2, false, "complete"); token.Wait() && token.Error() != nil {
					log.WithFields(log.Fields{"error": token.Error()}).Fatal("MQTT publish error")
				}
				log.Info("Distributed broker`s info update complete by gateway")
			}

		// ユーザによるゲートウェイ担当エリアの設定
		case m := <-setGatewayBrokerMsgCh:
			metricsTrigger <- true
			log.WithFields(log.Fields{"payload": string(m.Payload())}).Trace("setGatewayBrokerMsgCh")
			// JSONデコード
			var gatewayCoverArea GatewayBrokerInfoSingleTopic
			if err := json.Unmarshal(m.Payload(), &gatewayCoverArea); err != nil {
				log.WithFields(log.Fields{"err": err}).Fatal("add gateway broker (gatewayNotifyMsgCh)")
			}
			key := fmt.Sprintf("%v-%v", gatewayCoverArea.BrokerInfo.Host, gatewayCoverArea.BrokerInfo.Port)
			if _, ok := gatewayCoverAreaInfo[key]; !ok {
				log.WithFields(log.Fields{
					"Host": gatewayCoverArea.BrokerInfo.Host,
					"Port": gatewayCoverArea.BrokerInfo.Port,
				}).Error("This gateway is not exists...")
				continue
			}
			// ToDo: この登録処理を変更して、単一のGatewayに複数の担当エリアを設定できるように変更する
			// gatewayCoverAreaInfo[key] = gatewayCoverArea
			var isDuplicateTopic = false
			for _, t := range gatewayCoverAreaInfo[key].Topics {
				isDuplicateTopic = t == gatewayCoverArea.Topic
				if isDuplicateTopic {
					break
				}
			}
			if !isDuplicateTopic {
				gatewayCoverAreaInfo[key].Topics = append(gatewayCoverAreaInfo[key].Topics, gatewayCoverArea.Topic)
			}

			var payload []GatewayBrokerInfoSingleTopic
			for _, v := range gatewayCoverAreaInfo {
				for _, t := range v.Topics {
					gatewayCoverArea = GatewayBrokerInfoSingleTopic{Topic: t, BrokerInfo: BrokerInfo{Host: v.BrokerInfo.Host, Port: v.BrokerInfo.Port}}
					payload = append(payload, gatewayCoverArea)
				}
			}
			msg, err := json.Marshal(payload)
			if err != nil {
				log.WithFields(log.Fields{"err": err}).Fatal("add gateway broker (gatewayNotifyMsgCh)")
			}
			if token := client.Publish("/api/gateway/info/all", 2, true, msg); token.Wait() && token.Error() != nil {
				log.WithFields(log.Fields{"error": token.Error()}).Fatal("Notify to manager")
			}
			log.WithFields(log.Fields{
				"Host": gatewayCoverArea.BrokerInfo.Host,
				"Port": gatewayCoverArea.BrokerInfo.Port,
			}).Info("Updated cover area info")

		// ユーザによる分散ブローカの登録（「出来なかったらやり直せばいいでしょ」の方針）
		case m := <-addDistributedBrokerMsgCh:
			metricsTrigger <- true
			log.WithFields(log.Fields{"payload": string(m.Payload())}).Trace("addDistributedBrokerMsgCh")
			if isUpdatingDistributedBrokerList {
				log.WithFields(log.Fields{
					"isUpdatingDistributedBrokerList": isUpdatingDistributedBrokerList,
					"gatewayStatusMap":                gatewayStatusMap,
				}).Error("Could not add distributed broker...")
				continue
			}

			isUpdatingDistributedBrokerList = true
			allDistributedBrokerList.Version++
			// JSONデコード
			var newDistributedBrokerInfo DistributedBrokerInfo
			if err := json.Unmarshal(m.Payload(), &newDistributedBrokerInfo); err != nil {
				log.WithFields(log.Fields{"err": err}).Fatal("add distributed broker (addDistributedBrokerMsgCh)")
			}
			// バリデーションを行う
			for _, info := range allDistributedBrokerList.DMBs {
				if info.BrokerInfo.Host == newDistributedBrokerInfo.BrokerInfo.Host && info.BrokerInfo.Port == newDistributedBrokerInfo.BrokerInfo.Port {
					isUpdatingDistributedBrokerList = false
					log.WithFields(log.Fields{
						"Host": newDistributedBrokerInfo.BrokerInfo.Host,
						"Port": newDistributedBrokerInfo.BrokerInfo.Port,
					}).Error("This broker is already exists (addDistributedBrokerMsgCh)")
					break
				}
			}
			allDistributedBrokerList.DMBs = append(allDistributedBrokerList.DMBs, newDistributedBrokerInfo)
			// 非安定ソート
			sort.Slice(allDistributedBrokerList.DMBs, func(i, j int) bool {
				return len(allDistributedBrokerList.DMBs[i].Topic) < len(allDistributedBrokerList.DMBs[j].Topic)
			})

			// JSONエンコード
			msg, err := json.Marshal(allDistributedBrokerList)
			if err != nil {
				log.WithFields(log.Fields{"err": err}).Fatal("add gateway broker (gatewayNotifyMsgCh)")
			}
			if token := client.Publish("/api/brokertable/all/info", 2, true, msg); token.Wait() && token.Error() != nil {
				log.WithFields(log.Fields{"error": token.Error()}).Fatal("Notify to manager")
			}

		case <-metricsTrigger:
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

		case <-signalCh:
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
			log.Info("Interrupt detected.\n")
			return
		}
	}
}
