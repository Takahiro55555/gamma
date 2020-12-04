package lookuptable_test

import (
	"fmt"
	"gateway/pkg/lookuptable"
	"reflect"
	"testing"
)

func TestLookupHost(t *testing.T) {
	type args struct {
		node  *lookuptable.Node
		topic string
	}
	type want struct {
		host string
		port uint16
		err  error
	}
	tests := []struct {
		name string
		args args
		want want
	}{
		{
			name: "Success basic 01",
			args: args{
				node: &lookuptable.Node{
					Children: map[string]*lookuptable.Node{},
					Host:     "127.0.0.1",
					Port:     5000,
				},
				topic: "/",
			},
			want: want{
				host: "127.0.0.1",
				port: 5000,
				err:  nil,
			},
		},
		{
			name: "Success basic 02",
			args: args{
				node: &lookuptable.Node{
					Children: map[string]*lookuptable.Node{
						"0": {
							Children: map[string]*lookuptable.Node{
								"1": {
									Children: map[string]*lookuptable.Node{
										"2": {
											Children: map[string]*lookuptable.Node{},
											Host:     "localhost",
											Port:     5003,
										},
									},
									Host: "localhost",
									Port: 5002,
								},
							},
							Host: "localhost",
							Port: 5001,
						},
					},
					Host: "localhost",
					Port: 5000,
				},
				topic: "/1",
			},
			want: want{
				host: "localhost",
				port: 5000,
				err:  nil,
			},
		},
		{
			name: "Success basic 03",
			args: args{
				node: &lookuptable.Node{
					Children: map[string]*lookuptable.Node{
						"0": {
							Children: map[string]*lookuptable.Node{
								"1": {
									Children: map[string]*lookuptable.Node{
										"2": {
											Children: map[string]*lookuptable.Node{},
											Host:     "localhost",
											Port:     5003,
										},
									},
									Host: "localhost",
									Port: 5002,
								},
							},
							Host: "localhost",
							Port: 5001,
						},
					},
					Host: "localhost",
					Port: 5000,
				},
				topic: "/0/1/2",
			},
			want: want{
				host: "localhost",
				port: 5003,
				err:  nil,
			},
		},
		{
			name: "Invalid topic 01",
			args: args{
				node: &lookuptable.Node{
					Children: map[string]*lookuptable.Node{},
					Host:     "example.com",
					Port:     5000,
				},
				topic: "/hoge",
			},
			want: want{
				host: "example.com",
				port: 5000,
				err:  lookuptable.TopicNameError{},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			host, port, err := lookuptable.LookupHost(tt.args.node, tt.args.topic)
			if tt.want.err == nil {
				/** エラーを期待しないテストケース **/
				if err != tt.want.err {
					t.Errorf("LookupHost() = %v (Type: %T), expected %v (Type: %T)", err, err, tt.want.err, tt.want.err)
				}
			} else {
				/** エラーを期待するテストケース **/
				if err == nil || reflect.ValueOf(err).Type() != reflect.ValueOf(tt.want.err).Type() {
					t.Errorf("LookupHost() = %v (Type: %T), expected %v (Type: %T)", err, err, tt.want.err, tt.want.err)
				}
			}
			/** host の確認 **/
			if fmt.Sprint(host) != fmt.Sprint(tt.want.host) {
				t.Errorf("LookupHost(); host = %v, expected %v", host, tt.want.host)
			}
			/** port の確認 **/
			if fmt.Sprint(port) != fmt.Sprint(tt.want.port) {
				t.Errorf("LookupHost(); port = %v, expected %v", port, tt.want.port)
			}
		})
	}
}

func TestUpdateHost(t *testing.T) {
	type args struct {
		node  *lookuptable.Node
		topic string
		host  string
		port  uint16
	}
	tests := []struct {
		name string
		args args
		want *lookuptable.Node
		err  error
	}{
		{
			name: "Success basic 01",
			args: args{node: &lookuptable.Node{}, topic: "/", host: "127.0.0.1", port: 5000},
			want: &lookuptable.Node{
				Children: map[string]*lookuptable.Node{},
				Host:     "127.0.0.1",
				Port:     5000,
			},
			err: nil,
		},
		{
			name: "Success basic 02 (add node)",
			args: args{node: &lookuptable.Node{}, topic: "/1234567890", host: "127.0.0.1", port: 5000},
			want: &lookuptable.Node{
				Children: map[string]*lookuptable.Node{
					"1234567890": {
						Children: map[string]*lookuptable.Node{},
						Host:     "127.0.0.1",
						Port:     5000,
					},
				},
				Host: "127.0.0.1",
				Port: 5000,
			},
			err: nil,
		},
		{
			name: "Success basic 03 (add node)",
			args: args{node: &lookuptable.Node{}, topic: "/1234567890/1", host: "127.0.0.1", port: 5000},
			want: &lookuptable.Node{
				Children: map[string]*lookuptable.Node{
					"1234567890": {
						Children: map[string]*lookuptable.Node{
							"1": {
								Children: map[string]*lookuptable.Node{},
								Host:     "127.0.0.1",
								Port:     5000,
							},
						},
						Host: "127.0.0.1",
						Port: 5000,
					},
				},
				Host: "127.0.0.1",
				Port: 5000,
			},
			err: nil,
		},
		{
			name: "Success basic 04 (add node)",
			args: args{node: &lookuptable.Node{}, topic: "/0/0/3", host: "localhost", port: 5000},
			want: &lookuptable.Node{
				Children: map[string]*lookuptable.Node{
					"0": {
						Children: map[string]*lookuptable.Node{
							"0": {
								Children: map[string]*lookuptable.Node{
									"3": {
										Children: map[string]*lookuptable.Node{},
										Host:     "localhost",
										Port:     5000,
									},
								},
								Host: "localhost",
								Port: 5000,
							},
						},
						Host: "localhost",
						Port: 5000,
					},
				},
				Host: "localhost",
				Port: 5000,
			},
			err: nil,
		},
		{
			name: "Success basic 05 (delete node)",
			args: args{node: &lookuptable.Node{
				Children: map[string]*lookuptable.Node{
					"0": {
						Children: map[string]*lookuptable.Node{},
						Host:     "localhost",
						Port:     5000,
					},
				},
				Host: "localhost",
				Port: 5000,
			},
				topic: "/",
				host:  "localhost",
				port:  5000,
			},
			want: &lookuptable.Node{
				Children: map[string]*lookuptable.Node{},
				Host:     "localhost",
				Port:     5000,
			},
			err: nil,
		},
		{
			name: "Success basic 06 (update node)",
			args: args{node: &lookuptable.Node{
				Children: map[string]*lookuptable.Node{
					"0": {
						Children: map[string]*lookuptable.Node{},
						Host:     "localhost",
						Port:     5000,
					},
					"1": {
						Children: map[string]*lookuptable.Node{},
						Host:     "localhost",
						Port:     5001,
					},
				},
				Host: "localhost",
				Port: 5000,
			},
				topic: "/0",
				host:  "127.0.0.1",
				port:  5002,
			},
			want: &lookuptable.Node{
				Children: map[string]*lookuptable.Node{
					"0": {
						Children: map[string]*lookuptable.Node{},
						Host:     "127.0.0.1",
						Port:     5002,
					},
					"1": {
						Children: map[string]*lookuptable.Node{},
						Host:     "localhost",
						Port:     5001,
					},
				},
				Host: "localhost",
				Port: 5000,
			},
			err: nil,
		},
		{
			name: "Topic name error 01 (boundary value)",
			args: args{node: &lookuptable.Node{}, topic: "/1234567890/0/1/2/3/4", host: "127.0.0.1", port: 5000},
			want: &lookuptable.Node{},
			err:  lookuptable.TopicNameError{},
		},
		{
			name: "Topic name error 02 (boundary value, not allowed charactor)",
			args: args{node: &lookuptable.Node{}, topic: "/1234567890/0/1/2/-1", host: "127.0.0.1", port: 5000},
			want: &lookuptable.Node{},
			err:  lookuptable.TopicNameError{},
		},
		{
			name: "Topic name error 03 (not allowed charactor)",
			args: args{node: &lookuptable.Node{}, topic: "/1234567890/0/1/2/#", host: "127.0.0.1", port: 5000},
			want: &lookuptable.Node{},
			err:  lookuptable.TopicNameError{},
		},
		{
			name: "Topic name error 04 (not allowed charactor)",
			args: args{node: &lookuptable.Node{}, topic: "/1234567890/0/1/2/+", host: "127.0.0.1", port: 5000},
			want: &lookuptable.Node{},
			err:  lookuptable.TopicNameError{},
		},
		{
			name: "Topic name error 05 (not allowed charactor)",
			args: args{node: &lookuptable.Node{}, topic: "/1234567890/0/1/2/A", host: "127.0.0.1", port: 5000},
			want: &lookuptable.Node{},
			err:  lookuptable.TopicNameError{},
		},
		{
			name: "Topic name error 06 (not allowed format)",
			args: args{node: &lookuptable.Node{}, topic: "/1234567890/01/1/2/3", host: "127.0.0.1", port: 5000},
			want: &lookuptable.Node{},
			err:  lookuptable.TopicNameError{},
		},
		{
			name: "Topic name error 07 (not allowed format)",
			args: args{node: &lookuptable.Node{}, topic: "/1234567890/10/1/2/3", host: "127.0.0.1", port: 5000},
			want: &lookuptable.Node{},
			err:  lookuptable.TopicNameError{},
		},
		{
			name: "Topic name error 08 (not allowed format)",
			args: args{node: &lookuptable.Node{}, topic: "1234567890/0/1/2/3", host: "127.0.0.1", port: 5000},
			want: &lookuptable.Node{},
			err:  lookuptable.TopicNameError{},
		},
		{
			name: "Topic name error 09 (not allowed format)",
			args: args{node: &lookuptable.Node{}, topic: "/1234567890/0/1/2/3/", host: "127.0.0.1", port: 5000},
			want: &lookuptable.Node{},
			err:  lookuptable.TopicNameError{},
		},
		{
			name: "Topic name error 10 (not allowed format)",
			args: args{node: &lookuptable.Node{}, topic: "/1234567890/0/1/2/3//", host: "127.0.0.1", port: 5000},
			want: &lookuptable.Node{},
			err:  lookuptable.TopicNameError{},
		},
		{
			name: "Topic name error 01 (not allowed format)",
			args: args{node: &lookuptable.Node{}, topic: "/1234567890/0/1/2/3", host: "256.0.0.1", port: 5000},
			want: &lookuptable.Node{},
			err:  lookuptable.HostError{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := lookuptable.UpdateHost(tt.args.node, tt.args.topic, tt.args.host, tt.args.port)
			if tt.err == nil {
				/** エラーを期待しないテストケース **/
				if got != tt.err {
					t.Errorf("UpdateHost() = %v (Type: %T), expected %v (Type: %T)", got, got, tt.err, tt.err)
				}
			} else {
				/** エラーを期待するテストケース **/
				if got == nil || reflect.ValueOf(got).Type() != reflect.ValueOf(tt.err).Type() {
					t.Errorf("UpdateHost() = %v (Type: %T), expected %v (Type: %T)", got, got, tt.err, tt.err)
				}
			}
			/** Node の確認 **/
			if fmt.Sprint(tt.args.node) != fmt.Sprint(tt.want) {
				t.Errorf("UpdateHost(); node = %v, expected %v", tt.args.node, tt.want)
			}
		})
	}
}

// NOTE: go の map は range でイテレーションすると、実行するたびに順序が入れ替わる
//       Node 構造体を文字列に変換する関数がきちんとのことを考慮しているか確認するためのテスト
func TestString(t *testing.T) {
	type args struct {
		node  *lookuptable.Node
	}
	tests := []struct {
		name string
		args args
		time int  // 試行回数
	}{
		{
			name: "Success 01",
			args: args{
				node: &lookuptable.Node{
					Children: map[string]*lookuptable.Node{
						"0": {
							Children: map[string]*lookuptable.Node{},
							Host:     "localhost",
							Port:     5000,
						},
						"1": {
							Children: map[string]*lookuptable.Node{},
							Host:     "localhost",
							Port:     5001,
						},
					},
					Host: "localhost",
					Port: 5000,
				},
			},
			time: 1000,
		},
	}
	for _, tt := range tests {
		want := fmt.Sprint(tt.args.node)
		isFailed := false
		for i:=0; i < tt.time; i++ {
			t.Run(tt.name, func(t *testing.T) {
				if fmt.Sprint(tt.args.node) != want {
					t.Errorf("UpdateHost(); node = %v, expected %v", tt.args.node, want)
					isFailed = true
				}
			})
			if isFailed {
				break
			}
		}
	}
}
