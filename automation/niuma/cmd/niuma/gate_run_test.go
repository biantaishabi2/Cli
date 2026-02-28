package main

import (
	"os"
	"strconv"
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
		result, err := parseMaxRetries("3")
		assert.NoError(t, err)
		assert.Equal(t, 3, result)
	})
	assert.Empty(t, stderr, "正常输入不应输出 warning")
}

func TestParseMaxRetries_NonNumericInput(t *testing.T) {
	stderr := captureStderr(t, func() {
		result, err := parseMaxRetries("abc")
		assert.NoError(t, err)
		assert.Equal(t, defaultGateMaxRetries, result, "非数字输入应回退默认值")
	})
	assert.True(t, strings.Contains(stderr, "WARNING"), "非数字输入应输出 WARNING")
	assert.True(t, strings.Contains(stderr, "abc"), "WARNING 应包含原始输入值")
}

func TestParseMaxRetries_EmptyInput(t *testing.T) {
	stderr := captureStderr(t, func() {
		result, err := parseMaxRetries("")
		assert.NoError(t, err)
		assert.Equal(t, defaultGateMaxRetries, result, "空值输入应回退默认值")
	})
	assert.True(t, strings.Contains(stderr, "WARNING"), "空值输入应输出 WARNING")
}

func TestParseMaxRetries_DefaultValueUnchanged(t *testing.T) {
	stderr := captureStderr(t, func() {
		result, err := parseMaxRetries(strconv.Itoa(defaultGateMaxRetries))
		assert.NoError(t, err)
		assert.Equal(t, defaultGateMaxRetries, result, "默认值语义应保持不变")
	})
	assert.Empty(t, stderr, "默认值输入不应输出 warning")
}

func TestParseMaxRetries_ZeroInput(t *testing.T) {
	stderr := captureStderr(t, func() {
		result, err := parseMaxRetries("0")
		assert.NoError(t, err)
		assert.Equal(t, 0, result, "max-retries=0 应表示仅首轮执行，不自动重试")
	})
	assert.Empty(t, stderr, "max-retries=0 不应输出 warning")
}

func TestParseMaxRetries_WhitespaceInput(t *testing.T) {
	stderr := captureStderr(t, func() {
		result, err := parseMaxRetries(" 0 ")
		assert.NoError(t, err)
		assert.Equal(t, 0, result, "max-retries 应允许前后空白并保持语义")
	})
	assert.Empty(t, stderr, "合法输入带空白不应输出 warning")
}

func TestParseMaxRetries_NegativeInput(t *testing.T) {
	stderr := captureStderr(t, func() {
		result, err := parseMaxRetries("-1")
		assert.Equal(t, 0, result)
		assert.Error(t, err, "负数输入应视为参数错误")
		assert.True(t, strings.Contains(err.Error(), "--max-retries"))
	})
	assert.Empty(t, stderr, "负数输入应直接返回错误，不走 warning 回退")
}

func TestRetryCountMapping_OnlyCountsAutoRetries(t *testing.T) {
	attemptKey := "issue-488-pr-100-run-1-attempt-1"

	assert.Equal(t, 0, retryCountForStorage(1, attemptKey), "首轮失败不应计入 retry_count")
	assert.Equal(t, 1, retryCountForStorage(2, attemptKey), "第 1 次自动重试后失败应计为 retry_count=1")
	assert.Equal(t, 2, retryCountForStorage(3, attemptKey), "第 2 次自动重试后失败应计为 retry_count=2")
	assert.Equal(t, 1, retryCountFromStorage(0, attemptKey), "读取 marker 时应恢复到内部失败计数")
	assert.Equal(t, 2, retryCountFromStorage(1, attemptKey), "读取 marker 时应恢复到内部失败计数")
	assert.Equal(t, 0, retryCountForDisplay(1, attemptKey), "输出给用户的 retry_count 应保持自动重试语义")
}

func TestRetryCountMapping_EscalationKeepsAutoRetrySemantics(t *testing.T) {
	attemptKey := "issue-488-pr-100-run-1-attempt-1"

	assert.Equal(t, 2, retryCountForStorage(3, attemptKey), "超限 escalation 场景下，外部 retry_count 仍只统计自动重试次数")
	assert.Equal(t, 2, retryCountForDisplay(3, attemptKey), "输出给用户的 retry_count 在 escalation 场景下不应额外增长")
}
