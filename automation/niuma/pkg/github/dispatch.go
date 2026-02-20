// pkg/github/dispatch.go
// Repository Dispatch API 封装
package github

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/go-github/v68/github"
)

// CreateRepositoryDispatch 发送 repository_dispatch 事件
func (c *Client) CreateRepositoryDispatch(ctx context.Context, eventType string, clientPayload map[string]interface{}) error {
	payloadBytes, err := json.Marshal(clientPayload)
	if err != nil {
		return fmt.Errorf("序列化 payload 失败: %w", err)
	}

	rawPayload := json.RawMessage(payloadBytes)
	_, _, err = c.gh.Repositories.Dispatch(ctx, c.owner, c.repo, github.DispatchRequestOptions{
		EventType:     eventType,
		ClientPayload: &rawPayload,
	})
	if err != nil {
		return fmt.Errorf("发送 repository dispatch 失败: %w", err)
	}
	return nil
}
