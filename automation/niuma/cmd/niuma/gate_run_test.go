package main

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func captureStderr(t *testing.T, fn func()) string {
	t.Helper()
	old := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stderr = w

	fn()

	w.Close()
	os.Stderr = old

	buf := make([]byte, 4096)
	n, _ := r.Read(buf)
	r.Close()
	return string(buf[:n])
}

func TestParseMaxRetries_NormalInput(t *testing.T) {
	stderr := captureStderr(t, func() {
		result := parseMaxRetries("3")
		assert.Equal(t, 3, result)
	})
	assert.Empty(t, stderr, "正常输入不应输出 warning")
}

func TestParseMaxRetries_NonNumericInput(t *testing.T) {
	stderr := captureStderr(t, func() {
		result := parseMaxRetries("abc")
		assert.Equal(t, 2, result, "非数字输入应回退默认值 2")
	})
	assert.True(t, strings.Contains(stderr, "WARNING"), "非数字输入应输出 WARNING")
	assert.True(t, strings.Contains(stderr, "abc"), "WARNING 应包含原始输入值")
}

func TestParseMaxRetries_EmptyInput(t *testing.T) {
	stderr := captureStderr(t, func() {
		result := parseMaxRetries("")
		assert.Equal(t, 2, result, "空值输入应回退默认值 2")
	})
	assert.True(t, strings.Contains(stderr, "WARNING"), "空值输入应输出 WARNING")
}

func TestParseMaxRetries_ZeroInput(t *testing.T) {
	stderr := captureStderr(t, func() {
		result := parseMaxRetries("0")
		assert.Equal(t, 0, result, "零值输入应保留为 0（禁用自动重试）")
	})
	assert.Empty(t, stderr, "零值输入不应输出 warning")
}

func TestParseMaxRetries_NegativeInput(t *testing.T) {
	stderr := captureStderr(t, func() {
		result := parseMaxRetries("-1")
		assert.Equal(t, 0, result, "负数输入应 clamp 到 0")
	})
	assert.True(t, strings.Contains(stderr, "WARNING"), "负数输入应输出 WARNING")
	assert.True(t, strings.Contains(stderr, "clamp"), "WARNING 应提示 clamp")
}
