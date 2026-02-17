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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, resolveDiscussMaxRounds(tt.cliRounds, tt.configRounds))
		})
	}
}
