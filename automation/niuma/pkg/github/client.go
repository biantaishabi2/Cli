// pkg/github/client.go
// GitHub API 客户端封装
package github

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/google/go-github/v68/github"
	"golang.org/x/oauth2"
)

// Client 封装 go-github 客户端
type Client struct {
	gh    *github.Client
	owner string
	repo  string
}

// NewClient 创建 Client，需要 token 和 "owner/repo" 格式的仓库路径
func NewClient(token, repoFullName string) (*Client, error) {
	owner, repo, err := parseRepo(repoFullName)
	if err != nil {
		return nil, err
	}

	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: token})
	tc := oauth2.NewClient(context.Background(), ts)
	gh := github.NewClient(tc)

	return &Client{gh: gh, owner: owner, repo: repo}, nil
}

// NewClientFromEnv 从 GITHUB_TOKEN 环境变量创建 Client
func NewClientFromEnv(repoFullName string) (*Client, error) {
	token := os.Getenv("GITHUB_TOKEN")
	if token == "" {
		return nil, fmt.Errorf("GITHUB_TOKEN environment variable not set")
	}
	return NewClient(token, repoFullName)
}

// Owner 返回仓库所有者
func (c *Client) Owner() string { return c.owner }

// Repo 返回仓库名
func (c *Client) Repo() string { return c.repo }

// GitHub 返回底层 go-github 客户端（用于高级操作）
func (c *Client) GitHub() *github.Client { return c.gh }

func parseRepo(fullName string) (owner, repo string, err error) {
	parts := strings.SplitN(fullName, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", fmt.Errorf("invalid repo format %q, expected owner/repo", fullName)
	}
	return parts[0], parts[1], nil
}
