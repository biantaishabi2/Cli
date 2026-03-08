package daemon

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseDependsOn(t *testing.T) {
	tests := []struct {
		name     string
		body     string
		expected []string
	}{
		{
			name:     "空 body",
			body:     "",
			expected: nil,
		},
		{
			name:     "无 depends-on",
			body:     "## 任务\n这是一个普通 issue",
			expected: nil,
		},
		{
			name:     "单个依赖",
			body:     "depends-on: #123",
			expected: []string{"123"},
		},
		{
			name:     "多个依赖",
			body:     "depends-on: #123, #456, #789",
			expected: []string{"123", "456", "789"},
		},
		{
			name:     "跨仓库引用",
			body:     "depends-on: owner/repo#42",
			expected: []string{"42"},
		},
		{
			name:     "混合引用",
			body:     "depends-on: #10, owner/repo#20",
			expected: []string{"10", "20"},
		},
		{
			name:     "多行多次出现",
			body:     "depends-on: #1\n其他内容\ndepends-on: #2, #3",
			expected: []string{"1", "2", "3"},
		},
		{
			name:     "去重",
			body:     "depends-on: #5, #5, #5",
			expected: []string{"5"},
		},
		{
			name:     "大小写不敏感",
			body:     "Depends-On: #99",
			expected: []string{"99"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ParseDependsOn(tt.body)
			assert.Equal(t, tt.expected, result)
		})
	}
}
