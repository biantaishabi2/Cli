// internal/testutil/testutil.go
// 测试工具函数：集成测试跳过逻辑和测试仓库常量
package testutil

import (
	"os"
	"strings"
	"testing"
)

const (
	// DefaultTestRepoFull 默认测试仓库
	DefaultTestRepoFull = "biantaishabi2/Cli-niuma-test"
)

// ResolveTestRepoFull 返回集成测试目标仓库
// 优先读取 NIUMA_TEST_REPO，未设置则回退默认测试仓。
func ResolveTestRepoFull() string {
	repo := strings.TrimSpace(os.Getenv("NIUMA_TEST_REPO"))
	if repo != "" {
		return repo
	}
	return DefaultTestRepoFull
}

// SkipIfNoToken 如果测试 token 未设置则跳过测试。
// 优先读取 NIUMA_TEST_TOKEN，未设置则回退 GITHUB_TOKEN。
func SkipIfNoToken(t *testing.T) string {
	t.Helper()
	token := strings.TrimSpace(os.Getenv("NIUMA_TEST_TOKEN"))
	if token == "" {
		token = strings.TrimSpace(os.Getenv("GITHUB_TOKEN"))
	}
	if token == "" {
		t.Skip("NIUMA_TEST_TOKEN/GITHUB_TOKEN not set, skipping integration test")
	}
	return token
}
