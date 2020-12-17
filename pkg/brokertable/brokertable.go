package brokertable

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

//////////////        以下、Brokertable 関連              //////////////

// LookupHost 関数は、トピック名から担当している分散ブローカのホスト名とポート番号を検索する
func LookupHost(root *Node, topic string) (string, uint16, error) {
	if err := validateTopic(topic); err != nil {
		return root.Host, root.Port, err
	}
	currentNode := root
	if topic != "/" {
		rep := regexp.MustCompile(`^/`)
		topicSlice := strings.Split(rep.ReplaceAllString(topic, ""), "/")
		for _, child := range topicSlice {

			n, ok := currentNode.Children[child]
			if !ok {
				break
			}
			currentNode = n
		}
	}
	return currentNode.Host, currentNode.Port, nil
}

// FIXME: 動的な分散ブローカの追加、削除には未対応（Broker.SubCntの引継ぎを何も考えていない）
// UpdateHost 関数は、トピック名とそれに対応する分散ブローカへの接続情報を更新する
// 更新の際、当該トピックより深いレベルの分散ブローカへの接続情報は削除される。
// そのため、更新処理の順序に気を付けること
func UpdateHost(root *Node, topic string, host string, port uint16) error {
	if err := validateTopic(topic); err != nil {
		return err
	}

	if err := validateHost(host); err != nil {
		return err
	}

	currentNode := root
	if topic != "/" {
		rep := regexp.MustCompile(`^/`)
		topicSlice := strings.Split(rep.ReplaceAllString(topic, ""), "/")
		for _, child := range topicSlice {
			if currentNode.Host == "" {
				currentNode.Host = host
			}
			if currentNode.Port == 0 {
				currentNode.Port = port
			}
			// NOTE: 初期化されていない map の場合、初期化を実行する
			if currentNode.Children == nil {
				currentNode.Children = map[string]*Node{}
			}

			if val, ok := currentNode.Children[child]; ok {
				currentNode = val
			} else {
				currentNode.Children[child] = &Node{Children: map[string]*Node{}, Host: host, Port: port}
				currentNode = currentNode.Children[child]
			}
		}
	}
	// NOTE: 古い子ノードは削除する
	currentNode.Host = host
	currentNode.Port = port
	currentNode.Children = map[string]*Node{}

	return nil
}

func validateTopic(topic string) error {
	rTopic := regexp.MustCompile(`^/([0-9]+(/[0-3])*)?$`)
	if rTopic.MatchString(topic) {
		return nil
	}
	return TopicNameError{Msg: fmt.Sprintf("Invalid topic name(%v). Allowed topic name`s regular expressions is '^/([0-9]+(/[0-3])*)?$' .", topic)}
}

func validateHost(host string) error {
	rIpv4Address := regexp.MustCompile(`^(([0-9]|[1-9][0-9]|1[0-9][0-9]|2[0-4][0-9]|25[0-5])\.){3}([0-9]|[1-9][0-9]|1[0-9][0-9]|2[0-4][0-9]|25[0-5])$`)
	rDomainName := regexp.MustCompile(`^([a-zA-Z0-9][a-zA-Z0-9-]{1,61}[a-zA-Z0-9]\.)+[a-zA-Z-]{2,}$`)
	exceptionalName := []string{
		"localhost",
	}

	if rIpv4Address.MatchString(host) || rDomainName.MatchString(host) {
		return nil
	}

	for _, h := range exceptionalName {
		if h == host {
			return nil
		}
	}

	return HostError{Msg: "Invalid host. Allowed 'host' formats are IPv4 address, domain name or 'localhost'."}
}

//////////////        以上、Brokertable 関連              //////////////
//////////////            以下、Node 関連                 //////////////

type Node struct {
	Children map[string]*Node
	Host     string
	Port     uint16
}

// 再帰的に Node 構造体を JSON 形式の文字列に変換する
func (n Node) String() string {
	// HACK: 文字列生成処理の部分があまり効率の良くない実装になっている
	result := fmt.Sprintf("{\"host\":\"%v\",\"port\":%v,\"children\":{", n.Host, n.Port)
	counter := 1

	// NOTE: go の map は range でイテレーションすると、実行するたびに順序が入れ替わるためキーをソートしている
	childrenKey := keys(n.Children)
	sort.Strings(childrenKey)
	for _, key := range childrenKey {
		val := n.Children[key]
		if counter != len(n.Children) {
			result += fmt.Sprintf("\"%v\":%v,", key, val)
		} else {
			result += fmt.Sprintf("\"%v\":%v", key, val)
		}
		counter++
	}
	result += "}}"
	return result
}

func keys(m map[string]*Node) []string {
	ks := []string{}
	for k, _ := range m {
		ks = append(ks, k)
	}
	return ks
}

//////////////            以上、Node 関連                 //////////////
//////////////           以下、エラー 関連                 //////////////

type TopicNameError struct {
	Msg string
}

func (e TopicNameError) Error() string {
	return fmt.Sprintf("Error: %v", e.Msg)
}

type HostError struct {
	Msg string
}

func (e HostError) Error() string {
	return fmt.Sprintf("Error: %v", e.Msg)
}

//////////////           以上、エラー 関連                 //////////////
