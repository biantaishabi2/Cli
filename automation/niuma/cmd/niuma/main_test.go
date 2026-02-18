package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestResolveDiscussMaxRounds_Priority(t *testing.T) {
	tests := []struct {
		name         string
		cliRounds    int
		configRounds int
		expected     int
	}{
		{
			name:         "CLI 优先于配置",
			cliRounds:    9,
			configRounds: 7,
			expected:     9,
		},
		{
			name:         "配置优先于默认值",
			cliRounds:    0,
			configRounds: 7,
			expected:     7,
		},
		{
			name:         "配置缺省时回退默认值",
			cliRounds:    0,
			configRounds: 0,
			expected:     5,
		},
		{
			name:         "CLI 上边界 20 生效",
			cliRounds:    20,
			configRounds: 7,
			expected:     20,
		},
		{
			name:         "配置上边界 20 生效",
			cliRounds:    0,
			configRounds: 20,
			expected:     20,
		},
		{
			name:         "配置超过上界回退默认值",
			cliRounds:    0,
			configRounds: 21,
			expected:     5,
		},
		{
			name:         "配置小于下界回退默认值",
			cliRounds:    0,
			configRounds: -1,
			expected:     5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, resolveDiscussMaxRounds(tt.cliRounds, tt.configRounds))
		})
	}
}

func TestValidateMaxDiscussionRounds(t *testing.T) {
	tests := []struct {
		name      string
		rounds    int
		wantError bool
	}{
		{name: "默认值 0 合法", rounds: 0, wantError: false},
		{name: "下界 1 合法", rounds: 1, wantError: false},
		{name: "上界 20 合法", rounds: 20, wantError: false},
		{name: "小于下界非法", rounds: -1, wantError: true},
		{name: "大于上界非法", rounds: 21, wantError: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateMaxDiscussionRounds(tt.rounds)
			if tt.wantError {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
		})
	}
}

func TestResolveIntegrationGateMaxRetries_Priority(t *testing.T) {
	tests := []struct {
		name     string
		flag     int
		env      string
		expected int
		wantErr  bool
	}{
		{
			name:     "flag 优先",
			flag:     5,
			env:      "2",
			expected: 5,
		},
		{
			name:     "env 生效",
			flag:     -1,
			env:      "4",
			expected: 4,
		},
		{
			name:    "env 非法时返回错误",
			flag:    -1,
			env:     "bad",
			wantErr: true,
		},
		{
			name:    "env 负数时返回错误",
			flag:    -1,
			env:     "-1",
			wantErr: true,
		},
		{
			name:     "全缺省回退默认",
			flag:     -1,
			env:      "",
			expected: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := resolveIntegrationGateMaxRetries(tt.flag, tt.env)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tt.expected, got)
		})
	}
}
