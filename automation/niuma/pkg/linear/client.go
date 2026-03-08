// pkg/linear/client.go
// Linear GraphQL 客户端：读写 issue 状态
package linear

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"

	"github.com/biantaishabi2/Cli/automation/niuma/pkg/daemon"
)

// Client Linear API 客户端
type Client struct {
	apiKey   string
	teamKey  string            // e.g. "UNI"
	teamID   string            // 初始化时查询
	stateIDs map[string]string // 状态名 → state ID
}

// NewClient 创建 Linear 客户端
func NewClient(teamKey, apiKey string) *Client {
	return &Client{teamKey: teamKey, apiKey: apiKey, stateIDs: map[string]string{}}
}

// Init 查询 team ID 和状态列表
func (c *Client) Init(ctx context.Context) error {
	q := `query($key: String!) { team(key: $key) { id states { nodes { id name } } } }`
	resp, err := c.gql(ctx, q, map[string]interface{}{"key": c.teamKey})
	if err != nil {
		return fmt.Errorf("查询 Linear team 失败: %w", err)
	}
	team := toMap(dig(resp, "data", "team"))
	if team == nil {
		return fmt.Errorf("Linear team %q 未找到", c.teamKey)
	}
	c.teamID = str(team, "id")
	for _, n := range toSlice(dig(team, "states", "nodes")) {
		s := toMap(n)
		c.stateIDs[str(s, "name")] = str(s, "id")
	}
	return nil
}

// FetchByStatus 按状态筛选 issues
func (c *Client) FetchByStatus(ctx context.Context, status string) ([]daemon.Issue, error) {
	q := `query($teamId: String!, $stateName: String!) {
		team(id: $teamId) { issues(filter: { state: { name: { eq: $stateName } } }, first: 50) {
			nodes { id number title description state { name } }
		} }
	}`
	resp, err := c.gql(ctx, q, map[string]interface{}{"teamId": c.teamID, "stateName": status})
	if err != nil {
		return nil, err
	}
	var result []daemon.Issue
	for _, n := range toSlice(dig(resp, "data", "team", "issues", "nodes")) {
		issue := toMap(n)
		result = append(result, daemon.Issue{
			ID:        str(issue, "id"),
			Number:    intVal(issue, "number"),
			Title:     str(issue, "title"),
			Body:      str(issue, "description"),
			Status:    str(toMap(issue["state"]), "name"),
			DependsOn: daemon.ParseDependsOn(str(issue, "description")),
		})
	}
	return result, nil
}

// SetStatus 更新 issue 状态
func (c *Client) SetStatus(ctx context.Context, issueID, status string) error {
	stateID, ok := c.stateIDs[status]
	if !ok {
		return fmt.Errorf("Linear 未知状态 %q", status)
	}
	m := `mutation($id: String!, $stateId: String!) { issueUpdate(id: $id, input: { stateId: $stateId }) { success } }`
	_, err := c.gql(ctx, m, map[string]interface{}{"id": issueID, "stateId": stateID})
	return err
}

// GetIssue 获取单个 issue
func (c *Client) GetIssue(ctx context.Context, issueID string) (*daemon.Issue, error) {
	q := `query($id: String!) { issue(id: $id) { id number title description state { name } } }`
	resp, err := c.gql(ctx, q, map[string]interface{}{"id": issueID})
	if err != nil {
		return nil, err
	}
	issue := toMap(dig(resp, "data", "issue"))
	if issue == nil {
		return nil, fmt.Errorf("Linear issue %s 未找到", issueID)
	}
	return &daemon.Issue{
		ID:        str(issue, "id"),
		Number:    intVal(issue, "number"),
		Title:     str(issue, "title"),
		Body:      str(issue, "description"),
		Status:    str(toMap(issue["state"]), "name"),
		DependsOn: daemon.ParseDependsOn(str(issue, "description")),
	}, nil
}

// CheckDependencies 批量检查依赖状态（遍历 team issues 匹配编号）
func (c *Client) CheckDependencies(ctx context.Context, ids []string) (map[string]string, error) {
	q := `query($teamId: String!) { team(id: $teamId) { issues(first: 100) { nodes { number state { name } } } } }`
	resp, err := c.gql(ctx, q, map[string]interface{}{"teamId": c.teamID})
	if err != nil {
		return nil, err
	}
	numStatus := map[string]string{}
	for _, n := range toSlice(dig(resp, "data", "team", "issues", "nodes")) {
		issue := toMap(n)
		numStatus[strconv.Itoa(intVal(issue, "number"))] = str(toMap(issue["state"]), "name")
	}
	result := map[string]string{}
	for _, id := range ids {
		if s, ok := numStatus[id]; ok {
			result[id] = s
		}
	}
	return result, nil
}

// AddComment 在 issue 上添加评论
func (c *Client) AddComment(ctx context.Context, issue daemon.Issue, body string) error {
	m := `mutation($issueId: String!, $body: String!) { commentCreate(input: { issueId: $issueId, body: $body }) { success } }`
	_, err := c.gql(ctx, m, map[string]interface{}{"issueId": issue.ID, "body": body})
	return err
}

// --- GraphQL + JSON 辅助 ---

func (c *Client) gql(ctx context.Context, query string, vars map[string]interface{}) (map[string]interface{}, error) {
	b, _ := json.Marshal(map[string]interface{}{"query": query, "variables": vars})
	req, _ := http.NewRequestWithContext(ctx, "POST", "https://api.linear.app/graphql", bytes.NewReader(b))
	req.Header.Set("Authorization", c.apiKey)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, body)
	}
	var r map[string]interface{}
	json.Unmarshal(body, &r)
	if errs := toSlice(r["errors"]); len(errs) > 0 {
		return nil, fmt.Errorf("GraphQL: %s", str(toMap(errs[0]), "message"))
	}
	return r, nil
}

func dig(m interface{}, keys ...string) interface{} {
	for _, k := range keys {
		mm, _ := m.(map[string]interface{})
		if mm == nil {
			return nil
		}
		m = mm[k]
	}
	return m
}

func toMap(v interface{}) map[string]interface{}   { m, _ := v.(map[string]interface{}); return m }
func toSlice(v interface{}) []interface{}           { s, _ := v.([]interface{}); return s }
func str(m map[string]interface{}, k string) string { s, _ := m[k].(string); return s }
func intVal(m map[string]interface{}, k string) int { f, _ := m[k].(float64); return int(f) }
