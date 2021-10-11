<!--                  このファイルは、Markdownファイルです                 -->
<!-- VS Codeなどの、 Markdownプレビュー機能のあるエディタで見ることをお勧めします-->

![Test golang](https://github.com/Takahiro55555/location-based-mqtt-gateway/workflows/Test%20golang/badge.svg)

# location-based-mqtt-gateway

## これは何？
空間データを扱うことに特化した分散MQTTシステムの一部
## 各サブパッケージの役割
以下に各サブパッケージの役割をまとめる

### broker
任意のMQTTブローカ（１つ）に関する情報を管理する
トピックとそのトピックをSubscribeしているクライアントの数の管理は subsctable パッケージに任せている。
### brokerpool
各MQTTブローカのbrokerオブジェクトをホスト名とポート番号をキーに管理する。
### brokertable
トピック名をキーに各ブローカのホスト名とポート番号を管理する

### metrics
平均メッセージ数などの統計情報を扱う

### subsctable
任意のMQTTブローカ（１つ）において、トピックとそのトピックをSubscribeしているクライアントの数の管理を行う

## 動かしかた
### 前提条件
- `make`コマンドが使えること
- Goの環境をインストール済みであること(`go version`コマンドを実行できること)
- 先に`manager`を起動すること

### 起動
次のコマンドを実行すると起動する

```
make run
```

## 分散ブローカ情報送信例
1. 更新情報の通知
注意：トピック名が短い順にソートしてから送信すること
```
$ # Managerブローカへ送信している
$ mosquitto_pub -h localhost -p 1884 -t "/api/brokertable/all/info" -m '{"version": 1, "brokers":[{"topic":"/","broker_info":{"host":"localhost","port":1893}},{"topic":"/1","broker_info":{"host":"localhost","port":1894}}]}'
```

2. 全ての Gateway の更新作業が終わるまで待つ

3. 全ての Gateway の更新作業が終わったことを通知
```
$ # Managerブローカへ送信している
$ mosquitto_pub -h localhost -p 1884 -t "/api/brokertable/update/status" -m 'complete'
```

