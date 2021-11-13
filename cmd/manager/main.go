package main

import (
	"flag"
	"fmt"
	"manager/internal/apps/manager"
	"os"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	log "github.com/sirupsen/logrus"
)

func main() {
	environment := flag.String("env", "production", "実行環境 [\"production\", \"dev\"]")
	logLevel := flag.String("level", "warn", "ログレベル [\"trace\", \"debug\", \"info\", \"warn\", \"error\", \"fatal\", \"panic\"]")
	setReportCaller := flag.Bool("caller", false, "ログに行番号を表示する")
	host := flag.String("host", "127.0.0.1", "Manager Broker のホスト名")
	port := flag.Uint("port", 1883, "Manager Broker のポート番号")
	flag.Parse()

	// 標準エラー出力でなく標準出力とする
	log.SetOutput(os.Stdout)

	// ログに行番号を表示する
	log.SetReportCaller(*setReportCaller)

	switch *environment {
	case "production":
		log.SetFormatter(&log.JSONFormatter{})
	case "dev":
		log.SetFormatter(&log.TextFormatter{
			FullTimestamp: true,
		})
	default:
		log.SetFormatter(&log.TextFormatter{
			FullTimestamp: true,
		})
		log.WithFields(log.Fields{"environment": *environment}).Fatal("Undefined environment")
	}
	log.WithFields(log.Fields{"environment": *environment}).Info()

	switch *logLevel {
	case "trace":
		log.SetLevel(log.TraceLevel)
	case "debug":
		log.SetLevel(log.DebugLevel)
	case "info":
		log.SetLevel(log.InfoLevel)
	case "warn":
		log.SetLevel(log.WarnLevel)
	case "error":
		log.SetLevel(log.ErrorLevel)
	case "fatal":
		log.SetLevel(log.FatalLevel)
	case "panic":
		log.SetLevel(log.PanicLevel)
	default:
		log.SetLevel(log.DebugLevel)
		log.WithFields(log.Fields{"level": *logLevel}).Fatal("Undefined log level")
	}
	log.WithFields(log.Fields{"level": *logLevel}).Info()

	//////////////        APIブローカへ接続するための準備        //////////////
	apiBroker := fmt.Sprintf("tcp://%v:%v", *host, *port)
	opts := mqtt.NewClientOptions()
	opts.AddBroker(apiBroker)

	// APIブローカへ接続
	apiClient := mqtt.NewClient(opts)
	if token := apiClient.Connect(); token.Wait() && token.Error() != nil {
		log.WithFields(log.Fields{"error": token.Error()}).Fatal("MQTT connect error")
	}
	log.WithFields(log.Fields{"host": *host, "port": *port}).Info("MQTT connected broker")
	defer apiClient.Disconnect(1000)

	manager.Manager(apiClient)
}
