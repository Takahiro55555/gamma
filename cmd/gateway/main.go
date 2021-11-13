package main

import (
	"flag"
	"gamma/internal/apps/gateway"
	"os"

	log "github.com/sirupsen/logrus"
)

func main() {
	environment := flag.String("env", "production", "実行環境 [\"production\", \"development\"]")
	logLevel := flag.String("level", "warn", "ログレベル [\"trace\", \"debug\", \"info\", \"warn\", \"error\", \"fatal\", \"panic\"]")
	setReportCaller := flag.Bool("caller", false, "ログに行番号を表示する")
	managerMBHost := flag.String("managerHost", "localhost", "Manager MQTT broker host")
	managerMBPort := flag.Int("managerPort", 1883, "Manager MQTT broker port")
	gatewayMBHost := flag.String("gatewayHost", "localhost", "Gateway MQTT broker host")
	gatewayMBPort := flag.Int("gatewayPort", 1884, "Gateway MQTT broker port")
	flag.Parse()

	// 標準エラー出力でなく標準出力とする
	log.SetOutput(os.Stdout)

	// ログに行番号を表示する
	log.SetReportCaller(*setReportCaller)

	switch *environment {
	case "production":
		log.SetFormatter(&log.JSONFormatter{})
	case "development":
		log.SetFormatter(&log.TextFormatter{
			FullTimestamp: true,
		})
	default:
		log.SetFormatter(&log.TextFormatter{
			FullTimestamp: true,
		})
		log.WithFields(log.Fields{"environment": *environment}).Fatal("Undefined environment")
	}

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
	log.WithFields(log.Fields{"environment": *environment}).Info()
	log.WithFields(log.Fields{"level": *logLevel}).Info()
	log.WithFields(log.Fields{"host": *managerMBHost, "port": uint16(*managerMBPort)}).Info("Manager MQTT broker")
	log.WithFields(log.Fields{"host": *gatewayMBHost, "port": uint16(*gatewayMBPort)}).Info("Gateway MQTT broker")

	managerMB := gateway.BrokerInfo{Host: *managerMBHost, Port: uint16(*managerMBPort)}
	gatewayMB := gateway.BrokerInfo{Host: *gatewayMBHost, Port: uint16(*gatewayMBPort)}
	gateway.Gateway(gatewayMB, managerMB)
}
