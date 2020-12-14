package brokerpool

import (
	"fmt"
	"reflect"
	"sync"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

/*************  　  以下 brokerpool 構造体関連  　  *************/

// Brokerpool is the interface definition
type Brokerpool interface {
	GetBroker(host string, port uint16) (Broker, error)
	ConnectBroker(host string, port uint16) error
	GetOrConnectBroker(host string, port uint16) (Broker, error)
	TryDisconnectBroker(host string, port uint16, expirationFromLastPub time.Duration, quiesce uint) bool
	IncreaseSubCnt(host string, port uint16) error
	DecreaseSubCnt(host string, port uint16) error
	GetSubCnt(host string, port uint16) (uint, error)
	GetLastPub(host string, port uint16) (time.Time, error)
	UpdateLastPub(host string, port uint16) error
}

type brokerpool struct {
	bt BrokersTableByHost
}

// NewBrokerPool は blokerpool 構造体を生成し、Brokerpool interface を返す
func NewBrokerPool() Brokerpool {
	return &brokerpool{}
}

func (p *brokerpool) GetBroker(host string, port uint16) (Broker, error) {
	bt, err := p.bt.Load(host)
	if err != nil {
		return nil, err
	}

	b, err := bt.Load(port)
	return b, err
}

func (p *brokerpool) ConnectBroker(host string, port uint16) error {
	b, err := p.GetBroker(host, port)

	// 重複接続を防ぐための確認
	if err == nil {
		return AlreadyConnectedError{Msg: fmt.Sprintf("This broker is already connected (tcp://%v:%v).", host, port)}
	}

	// エラーが NotFoundError であることを確認する
	typeNotFoundErr := reflect.ValueOf(NotFoundError{}).Type()
	if reflect.ValueOf(err).Type() != typeNotFoundErr {
		return err
	}

	// ブローカへの接続を試みる
	b, err = connectBroker(host, port)
	if err != nil {
		return err
	}

	// brokerpool へ Broker インターフェースを登録する
	bt, err := p.bt.Load(host)
	if err != nil && reflect.ValueOf(err).Type() == typeNotFoundErr {
		bt = &BrokerTableByPort{}
		p.bt.Store(host, bt)
	}

	bt.Store(port, b)

	return nil
}

func (p *brokerpool) GetOrConnectBroker(host string, port uint16) (Broker, error) {
	b, err := p.GetBroker(host, port)
	if err == nil {
		return b, err
	}

	err = p.ConnectBroker(host, port)
	if err != nil {
		return nil, err
	}

	return p.GetBroker(host, port)
}

func (p *brokerpool) TryDisconnectBroker(host string, port uint16, expirationFromLastPub time.Duration, quiesce uint) bool {
	b, err := p.GetBroker(host, port)
	if err != nil {
		return false
	}
	return b.TryDisconnect(expirationFromLastPub, quiesce)
}

func (p *brokerpool) IncreaseSubCnt(host string, port uint16) error {
	b, err := p.GetBroker(host, port)
	if err != nil {
		return err
	}

	return b.IncreaseSubCnt()
}

func (p *brokerpool) DecreaseSubCnt(host string, port uint16) error {
	b, err := p.GetBroker(host, port)
	if err != nil {
		return err
	}

	return b.DecreaseSubCnt()
}

func (p *brokerpool) GetSubCnt(host string, port uint16) (uint, error) {
	b, err := p.GetBroker(host, port)
	if err != nil {
		return 0, err
	}

	return b.GetSubCnt(), nil
}

func (p *brokerpool) GetLastPub(host string, port uint16) (time.Time, error) {
	b, err := p.GetBroker(host, port)
	if err != nil {
		return time.Now(), err
	}
	return b.GetLastPub(), nil
}

func (p *brokerpool) UpdateLastPub(host string, port uint16) error {
	b, err := p.GetBroker(host, port)
	if err != nil {
		return err
	}

	b.UpdateLastPub()
	return nil
}

/*************  　  以上 brokerpool 構造体関連  　  *************/
/*************  　    以下 broker 構造体関連  　    *************/

// HACK: broker 構造体関連は別パッケージにしたほうが良さげ

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

func newBroker(c mqtt.Client) Broker {
	return &broker{
		Client:  c,
		SubCnt:  0,
		LastPub: time.Now(),
	}
}

func connectBroker(host string, port uint16) (Broker, error) {
	opts := mqtt.NewClientOptions()
	opts.AddBroker(fmt.Sprintf("tcp://%v:%v", host, port))
	c := mqtt.NewClient(opts)
	b := newBroker(c)
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
/*************　以下 BrokersTableByHost 構造体関連 *************/

// BrokersTableByHost 構造体を管理する構造体 (map)
type BrokersTableByHost struct {
	t sync.Map // BrokerTableByPort
}

func newBrokersTableByHost() *BrokersTableByHost {
	return &BrokersTableByHost{}
}

// Store 関数
func (s *BrokersTableByHost) Store(key string, value *BrokerTableByPort) {
	s.t.Store(key, value)
}

// Load 関数
func (s *BrokersTableByHost) Load(key string) (*BrokerTableByPort, error) {
	v, ok := s.t.Load(key)
	if !ok {
		return nil, NotFoundError{Msg: fmt.Sprintf("Not found (key = %v)", key)}
	}

	t, ok := v.(*BrokerTableByPort)
	if !ok {
		return nil, StoredTypeIsInvalidError{Msg: fmt.Sprintf("Stored type is invalid (expected = %T, result = %T)", BrokerTableByPort{}, v)}
	}

	return t, nil
}

/*************　以上 BrokersTableByHost 構造体関連 *************/
/*************　以下 BrokerTableByPort 構造体関連  *************/

// BrokerTableByPort 構造体は broker interface を管理する構造体 (Map)
type BrokerTableByPort struct {
	t sync.Map
}

func newBrokerTableByPort() *BrokerTableByPort {
	return &BrokerTableByPort{}
}

// Store 関数
func (s *BrokerTableByPort) Store(key uint16, value Broker) {
	s.t.Store(key, value)
}

// Load 関数
func (s *BrokerTableByPort) Load(key uint16) (Broker, error) {
	v, ok := s.t.Load(key)
	if !ok {
		return nil, NotFoundError{Msg: fmt.Sprintf("Not found (key = %v)", key)}
	}

	t, ok := v.(Broker)
	if !ok {
		return nil, StoredTypeIsInvalidError{Msg: fmt.Sprintf("Stored type is invalid (expected = %T, result = %T)", broker{}, v)}
	}

	return t, nil
}

/*************　以上 BrokerTableByPort 構造体関連 *************/
/*************      　以下 Error 構造体関連       *************/

// AlreadyConnectedError 構造体
// 同一ブローカへの２重接続が発生した際に使用する
type AlreadyConnectedError struct {
	Msg string
}

func (e AlreadyConnectedError) Error() string {
	return fmt.Sprintf("Error: %v", e.Msg)
}

// NotFoundError 構造体
// 主に Load 関数で使用する
type NotFoundError struct {
	Msg string
}

func (e NotFoundError) Error() string {
	return fmt.Sprintf("Error: %v", e.Msg)
}

// StoredTypeIsInvalidError 構造体
// 主に Store 関数で使用する
type StoredTypeIsInvalidError struct {
	Msg string
}

func (e StoredTypeIsInvalidError) Error() string {
	return fmt.Sprintf("Error: %v", e.Msg)
}

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
