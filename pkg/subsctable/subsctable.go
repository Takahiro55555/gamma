package subsctable

import (
	"fmt"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"sync"

	log "github.com/sirupsen/logrus"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

//////////////          以下、Subsctable 関連              //////////////

type Subsctable interface {
	String() string
	IncreaseSubscriber(topic string) error
	DecreaseSubscriber(topic string) error
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
	rTopic := regexp.MustCompile(`^(/[0-9]+(/[0-3])*)?((/#)|(/[\w]+))?$`)
	if rTopic.MatchString(topic) {
		return nil
	}
	return TopicNameError{Msg: fmt.Sprintf("Invalid topic name(%v). Allowed topic name`s regular expressions is '^(/[0-9]+(/[0-3])*)?((/#)|(/[\\w]+))?$' .", topic)}
}

func NewSubsctable(c mqtt.Client, qos byte, ch chan<- mqtt.Message) Subsctable {
	return &subsctable{rootNode: &node{children: nodeMap{}}, client: c, qos: qos, msgCh: ch}
}

func (st *subsctable) IncreaseSubscriber(topic string) error {
	// トピック名の前処理
	err := validateTopic(topic)
	if err != nil {
		return err
	}
	rep := regexp.MustCompile(`^/`) // 先頭の "/" が邪魔なため、削除
	editedTopic := rep.ReplaceAllString(topic, "")

	currentNode := st.rootNode
	hasActiveWildcardNode := false
	topicSlice := strings.Split(editedTopic, "/")
	typeNotFoundErr := reflect.ValueOf(NotFoundError{}).Type()
	for _, child := range topicSlice {
		// 与えられたトピックをカバーするワイルドカードトピックが既に Subscribe されていた場合
		if !hasActiveWildcardNode && currentNode.HasActiveWildcardNode() {
			hasActiveWildcardNode = true
		}

		tmpNode, err := currentNode.children.Load(child)

		// 子ノードが存在しない場合は、追加する
		if err == nil {
			currentNode = tmpNode
		} else if reflect.ValueOf(err).Type() == typeNotFoundErr {
			fmt.Println("Add children: " + child)
			fmt.Println(currentNode)
			newNode := &node{parent: currentNode, children: nodeMap{}}
			currentNode.children.Store(child, newNode)
			currentNode = newNode
		} else if err != nil {
			return err
		}
	}

	// トピック名の設定
	currentNode.topic = topic

	// カウンターのカウントアップ
	err = currentNode.AddSubCnt()
	if err != nil {
		return err
	}

	// メッセージハンドラ
	var fForwardMsg mqtt.MessageHandler = func(client mqtt.Client, msg mqtt.Message) {
		st.msgCh <- msg
	}

	// 与えられたトピックをカバーするワイルドカードトピックが Subscribe されていなかった場合
	if !hasActiveWildcardNode {
		if token := st.client.Subscribe(currentNode.topic, st.qos, fForwardMsg); token.Wait() && token.Error() != nil {
			return token.Error()
		}
		if strings.HasSuffix(topic, "/#") {
			// 子ノードのトピックを全て Unsubscribe する
			// REVIEW: 次のようなトピック名を与えられた場合の挙動が心配 : "/#"
			currentNode.parent.UnsubscribeChildrenTopics(st.client)
		}
	}
	fmt.Printf("IncreaseSubscriber() currentNode: %v", currentNode)
	return nil
}

func (st *subsctable) DecreaseSubscriber(topic string) error {
	// トピック名の前処理
	err := validateTopic(topic)
	if err != nil {
		return err
	}
	rep := regexp.MustCompile(`^/`) // 先頭の "/" が邪魔なため、削除
	editedTopic := rep.ReplaceAllString(topic, "")

	currentNode := st.rootNode
	hasActiveWildcardNode := false
	var activeWildcardNode *node
	topicSlice := strings.Split(editedTopic, "/")
	for _, child := range topicSlice {
		// 与えられたトピックをカバーするワイルドカードトピックが既に Subscribe されていた場合
		if !hasActiveWildcardNode && currentNode.HasActiveWildcardNode() {
			hasActiveWildcardNode = true
			activeWildcardNode, err = currentNode.children.Load("#")
			if err != nil {
				return err
			}
		}
		currentNode, err = currentNode.children.Load(child)
		if err != nil {
			return err
		}
	}

	// currentNode と activeWildcardNode が同じ場合
	currentNode.DecreaseSubCnt()
	if hasActiveWildcardNode && currentNode.topic == activeWildcardNode.topic && currentNode.GetSubCnt() == 0 {
		// メッセージハンドラ
		var fForwardMsg mqtt.MessageHandler = func(client mqtt.Client, msg mqtt.Message) {
			st.msgCh <- msg
		}

		// 子ノードのトピックを必要に応じて Subscribe する
		currentNode.SubscribeChildrenTopics(st.client, st.qos, fForwardMsg)

		// Unsubscribe する
		if token := st.client.Unsubscribe(currentNode.topic); token.Wait() && token.Error() != nil {
			return token.Error()
		}
	} else if !hasActiveWildcardNode && currentNode.GetSubCnt() == 0 {
		if token := st.client.Unsubscribe(currentNode.topic); token.Wait() && token.Error() != nil {
			return token.Error()
		}
	}
	fmt.Printf("DecreaseSubscriber() currentNode: %v", currentNode)
	fmt.Printf("DecreaseSubscriber() activeWildcardNode: %v", activeWildcardNode)
	return nil
}

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
			log.WithFields(log.Fields{
				"error": StoredTypeIsInvalidError{Msg: fmt.Sprintf("Key type is invalid (expected = %T)", "")},
			}).Fatal("Stored key type is invalid")
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

// 子ノードが Subscribe している Topic を必要に応じて Subscribe する
func (s *node) SubscribeChildrenTopics(c mqtt.Client, qos byte, callback mqtt.MessageHandler) {
	// 子ノードに有効なワイルドカードノードが存在する場合は、そのワイルドカードトピックのみを Subscribe し、
	// 自身や他の子ノードのトピックは Subscribe しない
	if s.HasActiveWildcardNode() {
		n, err := s.children.Load("#")
		if err != nil {
			log.WithFields(log.Fields{"error": err}).Fatal()
		}
		if token := c.Subscribe(n.topic, qos, callback); token.Wait() && token.Error() != nil {
			log.WithFields(log.Fields{"error": token.Error()}).Fatal("MQTT subscribe error")
		}
		return
	}

	// 子ノードに有効なワイルドカードノードが存在しない場合は、自身のトピックの Subscribe を試行する
	if s.GetSubCnt() > 0 {
		if token := c.Subscribe(s.topic, qos, callback); token.Wait() && token.Error() != nil {
			log.WithFields(log.Fields{"error": token.Error()}).Fatal("MQTT subscribe error")
		}
	}

	// 子ノードに対して再帰処理を行う
	keys := s.children.Keys()
	for _, k := range keys {
		n, err := s.children.Load(k)
		if err != nil {
			log.WithFields(log.Fields{"error": err}).Error("")
			continue
		}

		// 再帰関数に渡す
		n.SubscribeChildrenTopics(c, qos, callback)
	}
}

// 子ノードが Subscribe している Topic を全て Unsubscribe する
// ただし、node.subCnt はそのまま
// また、直接の子ノードになるワイルドカードトピックは Unsubscribe しない
func (s *node) UnsubscribeChildrenTopics(c mqtt.Client) {
	keys := s.children.Keys()
	for _, k := range keys {
		if k == "#" {
			continue
		}
		n, err := s.children.Load(k)
		if err != nil {
			log.WithFields(log.Fields{"error": err}).Error("")
			continue
		}
		// Unsubscribe する
		if n.GetSubCnt() > 0 {
			if token := c.Unsubscribe(n.topic); token.Wait() && token.Error() != nil {
				log.WithFields(log.Fields{"error": token.Error()}).Fatal("MQTT unsubscribe error")
			}
		}
		// 再帰関数に渡す
		n.unsubscribeAllChildrenTopics(c)
	}
}

// 子ノードが Subscribe している Topic を全て Unsubscribe する
// ただし、node.subCnt はそのまま
func (s *node) unsubscribeAllChildrenTopics(c mqtt.Client) {
	keys := s.children.Keys()
	for _, k := range keys {
		n, err := s.children.Load(k)
		if err != nil {
			log.WithFields(log.Fields{"error": err}).Error("")
			continue
		}
		// Unsubscribe する
		if n.topic != "" && n.GetSubCnt() > 0 {
			if token := c.Unsubscribe(n.topic); token.Wait() && token.Error() != nil {
				log.WithFields(log.Fields{"error": token.Error()}).Fatal("MQTT unsubscribe error")
				return
			}
		}
		// 再帰処理
		n.unsubscribeAllChildrenTopics(c)
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
