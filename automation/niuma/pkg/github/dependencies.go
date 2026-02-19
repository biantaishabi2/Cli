// pkg/github/dependencies.go
// Issue 依赖（展示层）相关 API 操作
package github

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"

	ghapi "github.com/google/go-github/v68/github"
)

const (
	// DependencyErrorTypeAuth 鉴权失败（token 无效）
	DependencyErrorTypeAuth = "auth"
	// DependencyErrorTypePermission 权限不足
	DependencyErrorTypePermission = "permission"
	// DependencyErrorTypeRateLimit 触发限流
	DependencyErrorTypeRateLimit = "rate_limit"
	// DependencyErrorTypeNetworkTimeout 网络/超时错误
	DependencyErrorTypeNetworkTimeout = "network_timeout"
	// DependencyErrorTypeUnsupported 仓库或 API 不支持该能力
	DependencyErrorTypeUnsupported = "unsupported"
	// DependencyErrorTypeUnknown 未分类错误
	DependencyErrorTypeUnknown = "unknown"
)

type dependencyIssue struct {
	ID     int64 `json:"id,omitempty"`
	Number int   `json:"number,omitempty"`
}

type dependencyBlockedByResponse struct {
	BlockedBy []dependencyIssue `json:"blocked_by,omitempty"`
}

// DependencyError 依赖 API 错误（带分类）
type DependencyError struct {
	Operation string
	Type      string
	Err       error
}

func (e *DependencyError) Error() string {
	return fmt.Sprintf("%s 失败(type=%s): %v", e.Operation, e.Type, e.Err)
}

func (e *DependencyError) Unwrap() error {
	return e.Err
}

// ClassifyDependencyError 将错误归类为可观测类型。
func ClassifyDependencyError(err error) string {
	if err == nil {
		return ""
	}

	var depErr *DependencyError
	if errors.As(err, &depErr) {
		return depErr.Type
	}
	var rateErr *ghapi.RateLimitError
	if errors.As(err, &rateErr) {
		return DependencyErrorTypeRateLimit
	}
	var abuseErr *ghapi.AbuseRateLimitError
	if errors.As(err, &abuseErr) {
		return DependencyErrorTypeRateLimit
	}
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		return DependencyErrorTypeNetworkTimeout
	}
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return DependencyErrorTypeNetworkTimeout
	}

	var ghErr *ghapi.ErrorResponse
	if errors.As(err, &ghErr) && ghErr.Response != nil {
		switch ghErr.Response.StatusCode {
		case http.StatusUnauthorized:
			return DependencyErrorTypeAuth
		case http.StatusForbidden:
			if isRateLimitResponse(ghErr.Response, ghErr.Message) {
				return DependencyErrorTypeRateLimit
			}
			return DependencyErrorTypePermission
		case http.StatusTooManyRequests:
			return DependencyErrorTypeRateLimit
		case http.StatusNotFound, http.StatusGone, http.StatusNotImplemented:
			return DependencyErrorTypeUnsupported
		}
	}

	return DependencyErrorTypeUnknown
}

func wrapDependencyError(operation string, err error) error {
	if err == nil {
		return nil
	}
	var depErr *DependencyError
	if errors.As(err, &depErr) {
		return err
	}
	return &DependencyError{
		Operation: operation,
		Type:      ClassifyDependencyError(err),
		Err:       err,
	}
}

func isRateLimitResponse(resp *http.Response, message string) bool {
	if resp != nil && resp.Header.Get("X-RateLimit-Remaining") == "0" {
		return true
	}
	msg := strings.ToLower(message)
	return strings.Contains(msg, "rate limit")
}

// ListIssueBlockedBy 列出 issue 当前展示层 blocked-by 依赖（返回 issue 编号集合）。
func (c *Client) ListIssueBlockedBy(ctx context.Context, issueNumber int) ([]int, error) {
	if issueNumber <= 0 {
		return nil, wrapDependencyError("list blocked_by", fmt.Errorf("issue 编号无效: %d", issueNumber))
	}

	path := fmt.Sprintf("repos/%s/%s/issues/%d/dependencies/blocked_by?per_page=100", c.owner, c.repo, issueNumber)
	req, err := c.gh.NewRequest("GET", path, nil)
	if err != nil {
		return nil, wrapDependencyError("list blocked_by", err)
	}
	req.Header.Set("Accept", "application/vnd.github+json")

	var byList []dependencyIssue
	_, err = c.gh.Do(ctx, req, &byList)
	if err != nil {
		// 兼容部分响应结构：{ "blocked_by": [...] }
		req2, reqErr := c.gh.NewRequest("GET", path, nil)
		if reqErr != nil {
			return nil, wrapDependencyError("list blocked_by", err)
		}
		req2.Header.Set("Accept", "application/vnd.github+json")
		var wrapped dependencyBlockedByResponse
		if _, err2 := c.gh.Do(ctx, req2, &wrapped); err2 != nil {
			return nil, wrapDependencyError("list blocked_by", err)
		}
		byList = wrapped.BlockedBy
	}

	seen := make(map[int]struct{}, len(byList))
	result := make([]int, 0, len(byList))
	for _, item := range byList {
		if item.Number <= 0 {
			continue
		}
		if _, ok := seen[item.Number]; ok {
			continue
		}
		seen[item.Number] = struct{}{}
		result = append(result, item.Number)
	}
	return result, nil
}

// AddIssueBlockedBy 追加一条 blocked-by 展示依赖：issueNumber 被 blockedByIssueNumber 阻塞。
func (c *Client) AddIssueBlockedBy(ctx context.Context, issueNumber int, blockedByIssueNumber int) error {
	if issueNumber <= 0 || blockedByIssueNumber <= 0 {
		return wrapDependencyError("add blocked_by", fmt.Errorf("issue 编号无效: issue=%d blocked_by=%d", issueNumber, blockedByIssueNumber))
	}

	depIssue, err := c.GetIssue(ctx, blockedByIssueNumber)
	if err != nil {
		return wrapDependencyError("add blocked_by", err)
	}
	depIssueID := depIssue.GetID()
	if depIssueID <= 0 {
		return wrapDependencyError("add blocked_by", fmt.Errorf("依赖 issue #%d 缺少 id", blockedByIssueNumber))
	}

	path := fmt.Sprintf("repos/%s/%s/issues/%d/dependencies/blocked_by", c.owner, c.repo, issueNumber)
	body := map[string]int64{
		"issue_id": depIssueID,
	}
	req, err := c.gh.NewRequest("POST", path, body)
	if err != nil {
		return wrapDependencyError("add blocked_by", err)
	}
	req.Header.Set("Accept", "application/vnd.github+json")

	_, err = c.gh.Do(ctx, req, nil)
	return wrapDependencyError("add blocked_by", err)
}

// RemoveIssueBlockedBy 删除一条 blocked-by 展示依赖：issueNumber 不再被 blockedByIssueNumber 阻塞。
func (c *Client) RemoveIssueBlockedBy(ctx context.Context, issueNumber int, blockedByIssueNumber int) error {
	if issueNumber <= 0 || blockedByIssueNumber <= 0 {
		return wrapDependencyError("remove blocked_by", fmt.Errorf("issue 编号无效: issue=%d blocked_by=%d", issueNumber, blockedByIssueNumber))
	}

	depIssue, err := c.GetIssue(ctx, blockedByIssueNumber)
	if err != nil {
		return wrapDependencyError("remove blocked_by", err)
	}
	depIssueID := depIssue.GetID()
	if depIssueID <= 0 {
		return wrapDependencyError("remove blocked_by", fmt.Errorf("依赖 issue #%d 缺少 id", blockedByIssueNumber))
	}

	path := fmt.Sprintf(
		"repos/%s/%s/issues/%d/dependencies/blocked_by/%d",
		c.owner, c.repo, issueNumber, depIssueID,
	)
	req, err := c.gh.NewRequest("DELETE", path, nil)
	if err != nil {
		return wrapDependencyError("remove blocked_by", err)
	}
	req.Header.Set("Accept", "application/vnd.github+json")

	_, err = c.gh.Do(ctx, req, nil)
	return wrapDependencyError("remove blocked_by", err)
}
