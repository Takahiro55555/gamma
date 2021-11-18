package dmb

import (
	"encoding/json"
	"fmt"
	"gamma/internal/apps/gateway"
	"math"
	"math/rand"
	"os"
	"os/signal"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	log "github.com/sirupsen/logrus"
)

// FIXME: 変数名、引数名、コメント等の単語・綴りの統一
func DMB(managerMB gateway.BrokerInfo, distributedMB gateway.BrokerInfo, distributedMBTopic string, baseRetransmissionIntervalMilliSeconds int,
	maxRetransmissionIntervalMilliSeconds int) {
	// プルグラムを強制終了させるためのチャンネル
	signalCh := make(chan os.Signal, 1)
	signal.Notify(signalCh, os.Interrupt)

	isRegisterd := false
	retransmissionCounter := 0
	allDistributedBrokerList := gateway.AllDistributedBrokerInfo{Version: -1, DMBs: []gateway.DistributedBrokerInfo{}}

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

	// 分散ブローカの追加リクエストを受取るチャンネル
	brokertableInfoMsgCh := make(chan mqtt.Message, 10)
	var brokertableInfoMsgFunc mqtt.MessageHandler = func(client mqtt.Client, msg mqtt.Message) {
		brokertableInfoMsgCh <- msg
	}
	if token := managerClient.Subscribe("/api/brokertable/all/info", 1, brokertableInfoMsgFunc); token.Wait() && token.Error() != nil {
		log.WithFields(log.Fields{"error": token.Error()}).Fatal("MQTT subscribe error")
	}

	// Managerへ分散MQTT接続情報の通知
	notifiNewDMBToManager(managerClient, distributedMB, distributedMBTopic)
	retransmissionTimer := time.NewTimer(time.Millisecond * time.Duration(baseRetransmissionIntervalMilliSeconds))

	for {
		select {
		// manager から分散MQTTブローカの情報を受け取るチャンネル
		case m := <-brokertableInfoMsgCh:
			log.Info("New brokertable info recieved")
			if err := json.Unmarshal(m.Payload(), &allDistributedBrokerList); err != nil {
				log.WithFields(log.Fields{"err": err}).Fatal("add distributed broker (addDistributedBrokerMsgCh)")
			}
			log.WithFields(log.Fields{"allDistributedBrokerList": string(m.Payload())}).Info("Received distributed MQTT broker info")
			for _, v := range allDistributedBrokerList.DMBs {
				if v.BrokerInfo.Host == distributedMB.Host && v.BrokerInfo.Port == distributedMB.Port {
					log.WithFields(log.Fields{"myDistributedBrokerHost": distributedMB.Host, "myDistributedBrokerPort": distributedMB.Port}).Info("My distributed broker was successfully registered to manager")
					isRegisterd = true
					break
				}
			}
			continue

		// manager に自分が受け持つ分散MQTTブローカが正常に追加されていなかった場合、再度追加リクエストを送るためのチャンネル
		case <-retransmissionTimer.C:
			log.Debug("Timer triggerd")

			// 既に自分が担当する分散MQTTブローカの登録が完了している場合、処理を終える
			if isRegisterd {
				continue
			}

			// Managerへ分散MQTT接続情報の再通知
			notifiNewDMBToManager(managerClient, distributedMB, distributedMBTopic)
			retransmissionCounter++

			// 次に追加完了確認を行うまでの時間を決める乱数の範囲を、確認回数に応じて指数関数的に増やす
			tmpMaxRetransmissionIntervalMilliSeconds := int(math.Pow(float64(baseRetransmissionIntervalMilliSeconds), float64(retransmissionCounter+1)))

			// 最大値を超えていないかを確認する
			if tmpMaxRetransmissionIntervalMilliSeconds > maxRetransmissionIntervalMilliSeconds {
				tmpMaxRetransmissionIntervalMilliSeconds = maxRetransmissionIntervalMilliSeconds
				retransmissionCounter-- // tmpMaxRetransmissionIntervalMilliSeconds のオーバーフロー対策
			}

			waittimeMilliSecnds := rand.Intn(tmpMaxRetransmissionIntervalMilliSeconds)
			log.WithFields(log.Fields{"waittimeMilliSecnds": waittimeMilliSecnds}).Debug("Will be retransmit my distributed MQTT broker info to manager")

			// タイマーを再度設定する
			retransmissionTimer = time.NewTimer(time.Millisecond * time.Duration(waittimeMilliSecnds))
			continue

		case <-signalCh:
			log.Info("Interrupt detected.\n")
			return
		}
	}
}

func notifiNewDMBToManager(managerCliet mqtt.Client, dmbInfo gateway.BrokerInfo, dmbTopic string) {
	// mosquitto_pub -h localhost -p 1883 -t "/api/tool/distributedbroker/add" -m '{"topic":"/","broker_info":{"host":"localhost","port":1893}}'
	msg := fmt.Sprintf(`{"broker_info":{"host":"%v","port":%v},"topic":"%v"}`, dmbInfo.Host, dmbInfo.Port, dmbTopic)
	if token := managerCliet.Publish("/api/tool/distributedbroker/add", 1, false, msg); token.Wait() && token.Error() != nil {
		log.WithFields(log.Fields{"error": token.Error()}).Fatal("Notify to manager")
	}
	log.WithFields(log.Fields{"msg": msg}).Info("Notified new distributed MQTT broker to manager")
}
