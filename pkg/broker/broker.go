package broker

import (
	"fmt"
	"gamma/pkg/subsctable"
	"sync"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	log "github.com/sirupsen/logrus"
)

//////////////        以下 broker 構造体関連        //////////////
// Broker is the interface definition
type Broker interface {
	Publish(topic string, retained bool, payload interface{})
	Subscribe(topic string) error
	Unsubscribe(topic string) error
	TryDisconnect(expirationFromLastPub time.Duration, quiesce uint) bool
	Disconnect(quiesce uint)
	IncreaseSubCnt() error
	DecreaseSubCnt() error
	GetSubCnt() uint
	UpdateLastPub()
	GetLastPub() time.Time
	CreateSubsetBroker(host string, port uint16, qos byte, ch chan<- mqtt.Message, topic string) (Broker, error)
	SubscribeAll()
	UnsubscribeSubsetTopics(topic string) error
}

// 分散ブローカに関するデータを管理する構造体
type broker struct {
	Client    mqtt.Client
	SubCntMu  sync.RWMutex
	SubCnt    uint // 接続先分散ブローカーへ Subscribe 要求している MQTT クライアントの数
	LastPubMu sync.RWMutex
	LastPub   time.Time // 接続先分散ブローカーへ MQTT クライアントが最後に Publish 要求をした時刻
	subTb     subsctable.Subsctable
	qos       byte
}

func NewBroker(c mqtt.Client, qos byte, ch chan<- mqtt.Message) Broker {
	return &broker{
		Client:  c,
		SubCnt:  0,
		LastPub: time.Now(),
		qos:     qos,
		subTb:   subsctable.NewSubsctable(c, qos, ch),
	}

}

func (b *broker) getQos() byte {
	return b.qos
}

func (b *broker) CreateSubsetBroker(host string, port uint16, qos byte, ch chan<- mqtt.Message, topic string) (Broker, error) {
	c, err := connectBroker(host, port, ch)
	if err != nil {
		return nil, err
	}

	subTb, err := b.subTb.GetSubsetSubsctable(c, qos, ch, topic)
	if err != nil {
		return nil, err
	}

	return &broker{
		Client:  c,
		SubCnt:  0,
		LastPub: time.Now(),
		qos:     qos,
		subTb:   subTb,
	}, nil
}

func (b *broker) SubscribeAll() {
	b.subTb.SubscribeAll()
}

func (b *broker) UnsubscribeSubsetTopics(topic string) error {
	return b.subTb.UnsubscribeSubsetTopics(topic)
}

func connectBroker(host string, port uint16, ch chan<- mqtt.Message) (mqtt.Client, error) {
	opts := mqtt.NewClientOptions()
	opts.AddBroker(fmt.Sprintf("tcp://%v:%v", host, port))
	c := mqtt.NewClient(opts)
	if token := c.Connect(); token.Wait() && token.Error() != nil {
		log.WithFields(log.Fields{"func": "ConnectBroker", "error": token.Error()}).Debug("Connect error")
		return nil, token.Error()
	}
	return c, nil
}

func ConnectBroker(host string, port uint16, qos byte, ch chan<- mqtt.Message) (Broker, error) {
	c, err := connectBroker(host, port, ch)
	if err != nil {
		return nil, err
	}
	b := NewBroker(c, qos, ch)
	return b, nil
}

func (b *broker) Publish(topic string, retained bool, payload interface{}) {
	token := b.Client.Publish(topic, b.qos, retained, payload)
	token.Wait()
	if token.Error() != nil {
		log.WithFields(log.Fields{"error": token.Error()}).Error("MQTT publish error")
	}
	b.UpdateLastPub()
}

func (b *broker) Subscribe(topic string) error {
	b.IncreaseSubCnt()
	return b.subTb.IncreaseSubscriber(topic)
}

func (b *broker) Unsubscribe(topic string) error {
	b.DecreaseSubCnt()
	return b.subTb.DecreaseSubscriber(topic)
}

func (b *broker) TryDisconnect(expirationFromLastPub time.Duration, quiesce uint) bool {
	opt := b.Client.OptionsReader()
	if b.GetSubCnt() != 0 {
		log.WithFields(log.Fields{"servers": opt.Servers()}).Debug("TryDisconnect()")
		return false
	}
	if b.GetLastPub().Add(expirationFromLastPub).After(time.Now()) {
		log.WithFields(log.Fields{"servers": opt.Servers()}).Debug("TryDisconnect()")
		return false
	}
	log.WithFields(log.Fields{"servers": opt.Servers()}).Debug("TryDisconnect()")
	b.Client.Disconnect(quiesce)
	return true
}

func (b *broker) Disconnect(quiesce uint) {
	opt := b.Client.OptionsReader()
	log.WithFields(log.Fields{"servers": opt.Servers()}).Debug("Disconnect()")
	b.Client.Disconnect(quiesce)
	b.SubCnt = 0
	b.LastPub = time.Date(1970, 1, 1, 0, 0, 0, 0, time.UTC) // Unix time の基準日
}

func (b *broker) IncreaseSubCnt() error {
	var maxSubCnt uint = 0xffffffff
	b.SubCntMu.Lock()
	defer b.SubCntMu.Unlock()

	if b.SubCnt < maxSubCnt {
		b.SubCnt++
		return nil
	}
	return MaxSubCntError{Msg: fmt.Sprintf("Already reached max SubCnt (%v)", maxSubCnt)}
}

func (b *broker) DecreaseSubCnt() error {
	b.SubCntMu.Lock()
	defer b.SubCntMu.Unlock()

	if b.SubCnt > 0 {
		b.SubCnt--
		return nil
	}
	return ZeroSubCntError{Msg: "SubCnt is already zero"}
}

func (b *broker) GetSubCnt() uint {
	b.SubCntMu.RLock()
	defer b.SubCntMu.RUnlock()
	return b.SubCnt
}

func (b *broker) UpdateLastPub() {
	b.LastPubMu.Lock()
	defer b.LastPubMu.Unlock()
	b.LastPub = time.Now()
}

func (b *broker) GetLastPub() time.Time {
	b.LastPubMu.RLock()
	defer b.LastPubMu.RUnlock()
	return b.LastPub
}

//////////////        以上 broker 構造体関連       //////////////
//////////////        以下 Error 構造体関連       //////////////

// MaxSubCntError 構造体
// 当該ブローカに設定された subscriber 上限に達した際に返される
type MaxSubCntError struct {
	Msg string
}

func (e MaxSubCntError) Error() string {
	return fmt.Sprintf("Error: %v", e.Msg)
}

// ZeroSubCntError 構造体
// 当該ブローカの subscriber が存在しないにもかかわらず、DecreaseSubCnt 関数を呼び出してしまった際に返される
type ZeroSubCntError struct {
	Msg string
}

func (e ZeroSubCntError) Error() string {
	return fmt.Sprintf("Error: %v", e.Msg)
}

//////////////        以上 Error 構造体関連       //////////////
