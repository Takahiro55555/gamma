package brokertable

import (
	"reflect"
	"testing"
)

func TestValidateTopicName(t *testing.T) {
	type args struct {
		topic string
	}
	tests := []struct {
		name string
		args args
		want error
	}{
		{
			name: "Valid topic name 01",
			args: args{topic: "/1234567890/0/1/2/3"},
			want: nil,
		},
		{
			name: "Valid topic name 02",
			args: args{topic: "/"},
			want: nil,
		},
		{
			name: "Invalid topic name 01 (boundary value)",
			args: args{topic: "/1234567890/0/1/2/3/4"},
			want: TopicNameError{},
		},
		{
			name: "Invalid topic name 02 (boundary value, not allowed charactor)",
			args: args{topic: "/1234567890/0/1/2/-1"},
			want: TopicNameError{},
		},
		{
			name: "Invalid topic name 03 (not allowed charactor)",
			args: args{topic: "/1234567890/0/1/2/#"},
			want: TopicNameError{},
		},
		{
			name: "Invalid topic name 04 (not allowed charactor)",
			args: args{topic: "/1234567890/0/1/2/+"},
			want: TopicNameError{},
		},
		{
			name: "Invalid topic name 05 (not allowed charactor)",
			args: args{topic: "/1234567890/0/1/2/A"},
			want: TopicNameError{},
		},
		{
			name: "Invalid topic name 06 (not allowed format)",
			args: args{topic: "/1234567890/01/1/2/3"},
			want: TopicNameError{},
		},
		{
			name: "Invalid topic name 07 (not allowed format)",
			args: args{topic: "/1234567890/10/1/2/3"},
			want: TopicNameError{},
		},
		{
			name: "Invalid topic name 08 (not allowed format)",
			args: args{topic: "1234567890/0/1/2/3"},
			want: TopicNameError{},
		},
		{
			name: "Invalid topic name 09 (not allowed format)",
			args: args{topic: "/1234567890/0/1/2/3/"},
			want: TopicNameError{},
		},
		{
			name: "Invalid topic name 10 (not allowed format)",
			args: args{topic: "/1234567890/0/1/2/3//"},
			want: TopicNameError{},
		},
		{
			name: "Invalid topic name 11 (not allowed charactor)",
			args: args{topic: "/hoge/0/1"},
			want: TopicNameError{},
		},
		{
			name: "Invalid topic name 12 (not allowed charactor)",
			args: args{topic: "/0-1=2"},
			want: TopicNameError{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := validateTopic(tt.args.topic)
			if tt.want == nil {
				/** エラーを期待しないテストケース **/
				if got != tt.want {
					t.Errorf("UpdateHost() = %v (Type: %T), expected %v (Type: %T)", got, got, tt.want, tt.want)
				}
			} else {
				/** エラーを期待するテストケース **/
				if got == nil || reflect.ValueOf(got).Type() != reflect.ValueOf(tt.want).Type() {
					t.Errorf("UpdateHost() = %v (Type: %T), expected %v (Type: %T)", got, got, tt.want, tt.want)
				}
			}
		})
	}
}

func TestValidateHost(t *testing.T) {
	type args struct {
		host string
	}
	tests := []struct {
		name string
		args args
		want error
	}{
		{
			name: "Valid IPv4 address 01 (boundary value)",
			args: args{host: "0.0.0.0"},
			want: nil,
		},
		{
			name: "Valid IPv4 address 02 (boundary value)",
			args: args{host: "255.255.255.255"},
			want: nil,
		},
		{
			name: "Valid IPv4 address 03",
			args: args{host: "127.0.0.1"},
			want: nil,
		},
		{
			name: "Invalid IPv4 address 01 (boundary value)",
			args: args{host: "255.255.255.256"},
			want: HostError{},
		},
		{
			name: "Invalid IPv4 address 02 (boundary value)",
			args: args{host: "256.255.255.255"},
			want: HostError{},
		},
		{
			name: "Invalid IPv4 address 03 (format)",
			args: args{host: "0.1.2.3.4"},
			want: HostError{},
		},
		{
			name: "Invalid IPv4 address 04 (escape)",
			args: args{host: "123-4#5$6"},
			want: HostError{},
		},
		{
			name: "Valid domain name 01",
			args: args{host: "golang.org"},
			want: nil,
		},
		{
			name: "Valid domain name 02",
			args: args{host: "www.Golang.org"},
			want: nil,
		},
		{
			name: "Valid domain name 03",
			args: args{host: "WWW.golang.org"},
			want: nil,
		},
		{
			name: "Valid domain name 04",
			args: args{host: "not-exist-may-be.golang.org"},
			want: nil,
		},
		{
			name: "Invalid domain name 01 (format)",
			args: args{host: "randomstringHOGHEODJSLFJDSLFELDFJDSL"},
			want: HostError{},
		},
		{
			name: "Valid exceptional name 01 (localhost)",
			args: args{host: "localhost"},
			want: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := validateHost(tt.args.host)
			if tt.want == nil {
				/** エラーを期待しないテストケース **/
				if got != tt.want {
					t.Errorf("UpdateHost() = %v (Type: %T), expected %v (Type: %T)", got, got, tt.want, tt.want)
				}
			} else {
				/** エラーを期待するテストケース **/
				if got == nil || reflect.ValueOf(got).Type() != reflect.ValueOf(tt.want).Type() {
					t.Errorf("UpdateHost() = %v (Type: %T), expected %v (Type: %T)", got, got, tt.want, tt.want)
				}
			}
		})
	}
}
