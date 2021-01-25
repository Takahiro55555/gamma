package broker

import (
	"reflect"
	"testing"
)

// テストケース分類
//   正常系
//   準正常系（使用に無いテストケースなど?）
//   異常系（エラーが発生するテストケース）
//   境界値

func TestIncreaseSubCnt(t *testing.T) {
	type args struct {
		b broker
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
				b: broker{
					SubCnt: 0,
				},
			},
			want: want{
				err: nil,
			},
		},
		{
			name: "Normal scenario 02 (境界値)",
			args: args{
				b: broker{
					SubCnt: 0xfffffffe,
				},
			},
			want: want{
				err: nil,
			},
		},
		{
			name: "Error scenario 01 (境界値)",
			args: args{
				b: broker{
					SubCnt: 0xffffffff,
				},
			},
			want: want{
				err: MaxSubCntError{},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.args.b.IncreaseSubCnt()
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

func TestDecreaseSubCnt(t *testing.T) {
	type args struct {
		b broker
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
				b: broker{
					SubCnt: 0xffff0000,
				},
			},
			want: want{
				err: nil,
			},
		},
		{
			name: "Normal scenario 02 (境界値)",
			args: args{
				b: broker{
					SubCnt: 1,
				},
			},
			want: want{
				err: nil,
			},
		},
		{
			name: "Error scenario 01 (境界値)",
			args: args{
				b: broker{
					SubCnt: 0,
				},
			},
			want: want{
				err: ZeroSubCntError{},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.args.b.DecreaseSubCnt()
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

func TestGetSubCnt(t *testing.T) {
	type args struct {
		b broker
	}
	type want struct {
		result uint
	}
	tests := []struct {
		name string
		args args
		want want
	}{
		{
			name: "Normal scenario 01",
			args: args{
				b: broker{
					SubCnt: 0xffff0000,
				},
			},
			want: want{
				result: 0xffff0000,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.args.b.GetSubCnt()
			if tt.want.result != result {
				t.Errorf("Expected: %v, Result: %v", tt.want.result, result)
			}
		})
	}
}

func TestUpdateLastPub(t *testing.T) {
	// NOTE: このテストに意味があるのかは微妙...
	type args struct {
		b broker
	}
	tests := []struct {
		name string
		args args
	}{
		{
			name: "Normal scenario 01",
			args: args{
				b: broker{
					SubCnt: 0xffff0000,
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.args.b.GetLastPub()
			tt.args.b.UpdateLastPub()
			if tt.args.b.LastPub == result {
				t.Errorf("Did not update broker.LastPub...")
			}
		})
	}
}
