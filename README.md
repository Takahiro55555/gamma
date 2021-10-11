<!--                  このファイルは、Markdownファイルです                 -->
<!-- VS Codeなどの、 Markdownプレビュー機能のあるエディタで見ることをお勧めします-->

![Test golang](https://github.com/Takahiro55555/location-based-mqtt-manager/workflows/Test%20golang/badge.svg)

# location-based-mqtt-manager

## 動かしかた
### 前提条件
- `make`コマンドが使えること
- Goの環境をインストール済みであること(`go version`コマンドを実行できること)
- 1台以上の分散MQTTブローカを起動しておくこと

### 起動
次のコマンドを実行すると起動する(ローカルで実行する場合)

```
make docker
make run
```

## 分散ブローカの登録例

```
$ mosquitto_pub -h localhost -p 1883 -t "/api/tool/distributedbroker/add" -m '{"topic":"/","broker_info":{"host":"localhost","port":1893}}'
$ mosquitto_pub -h localhost -p 1883 -t "/api/tool/distributedbroker/add" -m '{"topic":"/1","broker_info":{"host":"localhost","port":1894}}'
$ mosquitto_pub -h localhost -p 1883 -t "/api/tool/distributedbroker/add" -m '{"topic":"/2/3","broker_info":{"host":"localhost","port":1895}}'
$ mosquitto_pub -h localhost -p 1883 -t "/api/tool/distributedbroker/add" -m '{"topic":"/2/3","broker_info":{"host":"localhost","port":1896}}'
$ mosquitto_pub -h localhost -p 1883 -t "/api/tool/distributedbroker/add" -m '{"topic":"/2/3","broker_info":{"host":"localhost","port":1897}}'
$ mosquitto_pub -h localhost -p 1883 -t "/api/tool/distributedbroker/add" -m '{"topic":"/2/3","broker_info":{"host":"localhost","port":1898}}'
$ mosquitto_pub -h localhost -p 1883 -t "/api/tool/distributedbroker/add" -m '{"topic":"/2/3","broker_info":{"host":"localhost","port":1899}}'
$ mosquitto_pub -h localhost -p 1883 -t "/api/tool/distributedbroker/add" -m '{"topic":"/2/3","broker_info":{"host":"localhost","port":1900}}'
```

## ゲートウェイブローカの担当エリアの登録例
```
$ mosquitto_pub -h localhost -p 1883 -t "/api/tool/gatewaybroker/set" -m '{"topic":"/","broker_info":{"host":"localhost","port":1884}}'
```

## 動作確認

### メッセージの送信
```
$  msg="Message at `date`"; mosquitto_pub -h localhost -p 1884 -t "/forward/2/1/2/3" -m "${msg}"; echo $msg
```