package broker

import (
	"fmt"
	"sync"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

/*************  　    以下 broker 構造体関連  　    *************/
// Broker is the interface definition
type Broker interface {
	TryDisconnect(expirationFromLastPub time.Duration, quiesce uint) bool
	IncreaseSubCnt() error
	DecreaseSubCnt() error
	GetSubCnt() uint
	UpdateLastPub()
	GetLastPub() time.Time
}

// 分散ブローカに関するデータを管理する構造体
type broker struct {
	Client    mqtt.Client
	SubCntMu  sync.RWMutex
	SubCnt    uint // 接続先分散ブローカーへ Subscribe 要求している MQTT クライアントの数
	LastPubMu sync.RWMutex
	LastPub   time.Time // 接続先分散ブローカーへ MQTT クライアントが最後に Publish 要求をした時刻
}

func NewBroker(c mqtt.Client) Broker {
	return &broker{
		Client:  c,
		SubCnt:  0,
		LastPub: time.Now(),
	}
}

func ConnectBroker(host string, port uint16) (Broker, error) {
	opts := mqtt.NewClientOptions()
	opts.AddBroker(fmt.Sprintf("tcp://%v:%v", host, port))
	c := mqtt.NewClient(opts)
	b := NewBroker(c)
	return b, nil
}

func (b *broker) TryDisconnect(expirationFromLastPub time.Duration, quiesce uint) bool {
	if b.GetSubCnt() != 0 {
		return false
	}
	if b.GetLastPub().Add(expirationFromLastPub).Before(time.Now()) {
		return false
	}
	b.Client.Disconnect(quiesce)
	return true
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

/*************  　    以上 broker 構造体関連       *************/
/*************      　以下 Error 構造体関連       *************/

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

/*************      　以上 Error 構造体関連       *************/
