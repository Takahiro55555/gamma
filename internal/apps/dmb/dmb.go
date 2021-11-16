package dmb

import (
	"fmt"
	"gamma/internal/apps/gateway"
	"os"
	"os/signal"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	log "github.com/sirupsen/logrus"
)

func DMB(managerMB gateway.BrokerInfo, distributedMB gateway.BrokerInfo, distributedMBTopic string) {
	// プルグラムを強制終了させるためのチャンネル
	signalCh := make(chan os.Signal, 1)
	signal.Notify(signalCh, os.Interrupt)

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

	// Managerへ分散MQTT接続情報の通知
	// mosquitto_pub -h localhost -p 1883 -t "/api/tool/distributedbroker/add" -m '{"topic":"/","broker_info":{"host":"localhost","port":1893}}'
	msg := fmt.Sprintf(`{"broker_info":{"host":"%v","port":%v},"topic":"%v"}`, distributedMB.Host, distributedMB.Port, distributedMBTopic)
	if token := managerClient.Publish("/api/tool/distributedbroker/add", 1, false, msg); token.Wait() && token.Error() != nil {
		log.WithFields(log.Fields{"error": token.Error()}).Fatal("Notify to manager")
	}
	log.WithFields(log.Fields{"msg": msg}).Info("Notified new distributed MQTT broker to manager")

	for {
		select {
		case <-signalCh:
			log.Info("Interrupt detected.\n")
			return
		}
	}
}
