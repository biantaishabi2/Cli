// pkg/github/repositories.go
// Repository 相关 API 操作
package github

import (
	"context"
	"fmt"
	"strings"
)

// RepositoryInfo 表示控制层关心的仓库元数据。
type RepositoryInfo struct {
	DefaultBranch string
}

// GetRepository 获取仓库元数据。
func (c *Client) GetRepository(ctx context.Context) (*RepositoryInfo, error) {
	repo, _, err := c.gh.Repositories.Get(ctx, c.owner, c.repo)
	if err != nil {
		return nil, fmt.Errorf("获取仓库信息失败: %w", err)
	}
	return &RepositoryInfo{
		DefaultBranch: strings.TrimSpace(repo.GetDefaultBranch()),
	}, nil
}

// GetDefaultBranch 获取仓库默认分支。
func (c *Client) GetDefaultBranch(ctx context.Context) (string, error) {
	info, err := c.GetRepository(ctx)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(info.DefaultBranch), nil
}
