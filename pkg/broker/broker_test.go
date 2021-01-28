package broker_test

import (
	"testing"
)

// テストケース分類
//   正常系
//   準正常系（仕様に無いテストケースなど?）
//   異常系（エラーが発生するテストケース）
//   境界値

func TestIncreaseSubCnt(t *testing.T) {
	type args struct {
	}
	type want struct {
	}
	tests := []struct {
		name string
		args args
		want want
	}{
		{
			name: "Normal scenario 01",
			args: args{},
			want: want{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
		})
	}
}
