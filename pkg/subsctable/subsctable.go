package subsctable

import (
	"fmt"
	"log"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"sync"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

//////////////          以下、Subsctable 関連              //////////////

type Subsctable interface {
}

type subsctable struct {
	client   mqtt.Client
	rootNode *node
	qos      byte
	msgCh    chan<- mqtt.Message
}

func (st *subsctable) String() string {
	return fmt.Sprintf("{\"rootNode\":%v}", st.rootNode)
}

func validateTopic(topic string) error {
	rTopic := regexp.MustCompile(`^/([0-9]+(/[0-3])*)?((/#)|(/[\w]+))?$`)
	if rTopic.MatchString(topic) {
		return nil
	}
	return TopicNameError{Msg: "Invalid topic name. Allowed topic name`s regular expressions is '^/([0-9]+(/[0-3])*)?((/#)|(/[\\w]+))?$' ."}
}

func NewSubsctable(c mqtt.Client, qos byte, ch chan<- mqtt.Message) Subsctable {
	return &subsctable{client: c, qos: qos, msgCh: ch}
}

func (st *subsctable) AddSubscriber(topic string) error {
	err := validateTopic(topic)
	if err != nil {
		return err
	}
	// トピック名の前処理
	rep := regexp.MustCompile(`^/`) // 先頭の "/" が邪魔なため、削除
	editedTopic := rep.ReplaceAllString(topic, "")
	rep = regexp.MustCompile(`/#$`) // 末尾の "#" が邪魔なため、削除
	editedTopic = rep.ReplaceAllString(editedTopic, "")

	currentNode := st.rootNode // subCnt をカウントアップするノード
	// subscNode := currentNode   // 実際にSubscribeするノード
	isUpdatedSubscNode := false
	topicSlice := strings.Split(editedTopic, "/")
	typeNotFoundErr := reflect.ValueOf(NotFoundError{}).Type()
	for _, child := range topicSlice {
		currentNode, err := currentNode.children.Load(child)

		// 子ノードが存在しない場合は、追加する
		if reflect.ValueOf(err).Type() == typeNotFoundErr {
			newNode := &node{parent: currentNode}
			currentNode.children.Store(child, newNode)
			currentNode = newNode
		} else if err != nil {
			return err
		}

		// マルチレベルワイルドカードが指定されていた場合
		// if strings.HasSuffix(currentNode.topic, "/#") {
		// 	subscNode = currentNode
		// 	isUpdatedSubscNode = true
		// }
	}
	if !isUpdatedSubscNode {
		// subscNode = currentNode

	}

	// メッセージハンドラ
	var fForwardMsg mqtt.MessageHandler = func(client mqtt.Client, msg mqtt.Message) {
		st.msgCh <- msg
	}

	// if currentNode.GetSubCnt() == 0 {
	// 	// currentNode にトピック名を設定する
	// 	currentNode.topic = topic
	// }

	// カウンターのカウントアップ
	err = currentNode.AddSubCnt()
	if err != nil {
		return err
	}

	//
	if strings.HasSuffix(topic, "/#") && !strings.HasSuffix(currentNode.topic, "/#") {
		// マルチレベルワイルドカードが指定されていが、currentNode.topic には無い場合

		if token := st.client.Subscribe(topic, st.qos, fForwardMsg); token.Wait() && token.Error() != nil {
			return token.Error()
		}

		if currentNode.topic != "" {
			if token := st.client.Unsubscribe(currentNode.topic); token.Wait() && token.Error() != nil {
				log.Fatalf("Mqtt error: %s", token.Error())
			}
		}

		// トピック名を設定する
		currentNode.topic = topic

		// 子ノードのトピックを全て Unsubscribe する
		currentNode.UnsubscribeChildrenTopics(st.client)
	}

	return nil
}

// func (st *subsctable) RemoveSubscriber(topic string) error {
// 	err := validateTopic(topic)
// 	if err != nil {
// 		return err
// 	}
// 	rep := regexp.MustCompile(`^/`)
// 	topicSlice := strings.Split(rep.ReplaceAllString(topic, ""), "/")
// 	typeNotFoundErr := reflect.ValueOf(NotFoundError{}).Type()
// 	for _, child := range topicSlice {

// 	}
// 	return nil
// }

//////////////          以上、Subsctable 関連              //////////////
//////////////           以下、nodeMap 関連                //////////////

type nodeMap struct {
	s sync.Map
}

// Store 関数
func (s *nodeMap) Store(key string, value *node) {
	s.s.Store(key, value)
}

// Load 関数
func (s *nodeMap) Load(key string) (*node, error) {
	v, ok := s.s.Load(key)
	if !ok {
		return nil, NotFoundError{Msg: fmt.Sprintf("Not found (key = %v)", key)}
	}

	t, ok := v.(*node)
	if !ok {
		return nil, StoredTypeIsInvalidError{Msg: fmt.Sprintf("Stored type is invalid (expected = %T)", &node{})}
	}

	return t, nil
}

func (s *nodeMap) Keys() []string {
	ks := []string{}
	s.s.Range(func(key, _ interface{}) bool {
		k, ok := key.(string)
		if !ok {
			log.Fatal(StoredTypeIsInvalidError{Msg: fmt.Sprintf("Key type is invalid (expected = %T)", "")})
			return true
		}
		ks = append(ks, k)
		return true
	})
	return ks
}

//////////////           以上、nodeMap 関連                //////////////
//////////////            以下、node 関連                  //////////////

type node struct {
	parent   *node
	children nodeMap
	subCntMu sync.RWMutex
	subCnt   uint
	topic    string
}

// 再帰的に node 構造体を JSON 形式の文字列に変換する
func (n *node) String() string {
	// HACK: 文字列生成処理の部分があまり効率の良くない実装になっている
	result := fmt.Sprintf("{\"topic\":\"%v\",\"subCnt\":%v,\"children\":{", n.topic, n.subCnt)
	counter := 1

	// NOTE: go の map は range でイテレーションすると、実行するたびに順序が入れ替わるためキーをソートしている
	childrenKey := n.children.Keys()
	sort.Strings(childrenKey)
	for _, key := range childrenKey {
		val, err := n.children.Load(key)
		if err != nil {
			result += fmt.Sprintf("\"%v\":null", key)
		} else {
			result += fmt.Sprintf("\"%v\":%v", key, val)
		}

		if counter != len(childrenKey) {
			result += fmt.Sprintf(",")
		}
		counter++
	}
	result += "}}"
	return result
}

// node.subCnt のカウントアップは行わない
// func (s *node) Subscribe(c mqtt.Client, topic string) error {
// 	// 本ノードの担当トピックにワイルドカードが指定されておらず、
// 	if !strings.HasSuffix(s.topic, "/#") && strings.HasSuffix(topic, "/#") {

// 	}
// }

func (s *node) HasActiveWildcardNode() bool {
	n, err := s.children.Load("#")
	if err != nil {
		return false
	}
	return n.GetSubCnt() > 0
}

func (s *node) GetSubCnt() uint {
	s.subCntMu.RLock()
	defer s.subCntMu.RUnlock()
	return s.subCnt
}

func (s *node) AddSubCnt() error {
	var maxSubCnt uint = 0xffffffff
	s.subCntMu.Lock()
	defer s.subCntMu.Unlock()

	if s.subCnt < maxSubCnt {
		s.subCnt++
		return nil
	}
	return MaxSubCntError{Msg: fmt.Sprintf("Already reached max ubCnt (%v)", maxSubCnt)}
}

func (s *node) DecreaseSubCnt() error {
	s.subCntMu.Lock()
	defer s.subCntMu.Unlock()

	if s.subCnt > 0 {
		s.subCnt--
		return nil
	}
	return ZeroSubCntError{Msg: "SubCnt is already zero"}
}

// 子ノードが Subscribe している Topic を全て Unsubscribe する
// ただし、node.subCnt はそのまま
func (s *node) UnsubscribeChildrenTopics(c mqtt.Client) {
	keys := s.children.Keys()
	for _, k := range keys {
		n, err := s.children.Load(k)
		if err != nil {
			log.Fatal(err)
			continue
		}
		// Unsubscribe する
		if n.topic != "" && n.GetSubCnt() > 0 {
			if token := c.Unsubscribe(n.topic); token.Wait() && token.Error() != nil {
				log.Fatalf("Mqtt error: %s", token.Error())
				return
			}
		}
		// 再帰処理
		n.UnsubscribeChildrenTopics(c)
	}
}

//////////////            以上、node 関連                  //////////////
//////////////           以下、エラー 関連                 //////////////

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

type TopicNameError struct {
	Msg string
}

func (e TopicNameError) Error() string {
	return fmt.Sprintf("Error: %v", e.Msg)
}

// MaxSubCntError 構造体
// 当該トピックに設定された subscriber 上限に達した際に返される
type MaxSubCntError struct {
	Msg string
}

func (e MaxSubCntError) Error() string {
	return fmt.Sprintf("Error: %v", e.Msg)
}

// ZeroSubCntError 構造体
// 当該トピックの subscriber が存在しないにもかかわらず、DecreaseSubCnt 関数を呼び出してしまった際に返される
type ZeroSubCntError struct {
	Msg string
}

func (e ZeroSubCntError) Error() string {
	return fmt.Sprintf("Error: %v", e.Msg)
}

//////////////           以上、エラー 関連                 //////////////
