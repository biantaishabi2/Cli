// internal/testutil/testutil.go
// 测试工具函数：集成测试跳过逻辑和测试仓库常量
package testutil

import (
	"os"
	"testing"
)

const (
	// TestOwner 测试仓库所有者
	TestOwner = "biantaishabi2"
	// TestRepo 测试仓库名
	TestRepo = "Cli"
	// TestRepoFull 完整仓库路径
	TestRepoFull = TestOwner + "/" + TestRepo
)

// SkipIfNoToken 如果 GITHUB_TOKEN 未设置则跳过测试
func SkipIfNoToken(t *testing.T) string {
	t.Helper()
	token := os.Getenv("GITHUB_TOKEN")
	if token == "" {
		t.Skip("GITHUB_TOKEN not set, skipping integration test")
	}
	return token
}
