package subsctable

import (
	"reflect"
	"testing"
)

//////////////          以下、Subsctable 関連              //////////////
func TestValidateTopic(t *testing.T) {
	type args struct {
		topic string
	}
	type want struct {
		err error
	}
	tests := []struct {
		name string
		args args
		want want
	}{
		{
			name: "Normal scenario 01",
			args: args{
				topic: "/#",
			},
			want: want{
				err: nil,
			},
		},
		{
			name: "Normal scenario 02",
			args: args{
				topic: "/0",
			},
			want: want{
				err: nil,
			},
		},
		{
			name: "Normal scenario 03",
			args: args{
				topic: "/0/1/#",
			},
			want: want{
				err: nil,
			},
		},
		{
			name: "Normal scenario 04",
			args: args{
				topic: "/0/1/hoge",
			},
			want: want{
				err: nil,
			},
		},
		{
			name: "Normal scenario 05 (準正常系)",
			args: args{
				topic: "",
			},
			want: want{
				err: nil,
			},
		},
		{
			name: "Error scenario 01",
			args: args{
				topic: "#",
			},
			want: want{
				err: TopicNameError{},
			},
		},
		{
			name: "Error scenario 02",
			args: args{
				topic: "/#/0/1",
			},
			want: want{
				err: TopicNameError{},
			},
		},
		{
			name: "Error scenario 03",
			args: args{
				topic: "/0/1/2/hoge/#",
			},
			want: want{
				err: TopicNameError{},
			},
		},
	}
	_ = tests
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateTopic(tt.args.topic)
			if tt.want.err == nil {
				if err != nil {
					t.Errorf("Expected: %v, Result: %v", tt.want.err, err)
				}
			} else {
				if err == nil || reflect.ValueOf(err).Type() != reflect.ValueOf(tt.want.err).Type() {
					t.Errorf("Expected: %v, Result: %v", tt.want.err, err)
				}
			}
		})
	}
}

//////////////          以上、Subsctable 関連              //////////////
//////////////           以下、nodeMap 関連                //////////////

//////////////           以上、nodeMap 関連                //////////////
//////////////            以下、node 関連                  //////////////

//////////////            以上、node 関連                  //////////////
