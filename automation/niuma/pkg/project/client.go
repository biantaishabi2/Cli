// pkg/project/client.go
// GitHub Projects V2 GraphQL 客户端：读写 Project Status 字段
package project

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

// Client GitHub Projects V2 客户端
type Client struct {
	token         string
	projectID     string            // PVT_xxx
	owner         string
	projectNumber int
	statusFieldID string
	optionIDs     map[string]string // 状态名 → option node ID
}

// NewClient 创建 Projects 客户端
func NewClient(owner string, projectNumber int, token string) *Client {
	return &Client{owner: owner, projectNumber: projectNumber, token: token, optionIDs: map[string]string{}}
}

// Init 查询 Project 元数据（尝试 user，失败则尝试 org）
func (c *Client) Init(ctx context.Context) error {
	for _, ownerType := range []string{"user", "organization"} {
		q := fmt.Sprintf(`query($login: String!, $num: Int!) {
			%s(login: $login) { projectV2(number: $num) { id
				field(name: "Status") { ... on ProjectV2SingleSelectField { id options { id name } } }
			} }
		}`, ownerType)
		resp, err := c.gql(ctx, q, map[string]interface{}{"login": c.owner, "num": c.projectNumber})
		if err != nil {
			continue
		}
		root, _ := resp["data"].(map[string]interface{})[ownerType].(map[string]interface{})
		proj, _ := root["projectV2"].(map[string]interface{})
		if proj == nil {
			continue
		}
		c.projectID, _ = proj["id"].(string)
		field, _ := proj["field"].(map[string]interface{})
		c.statusFieldID, _ = field["id"].(string)
		for _, opt := range toSlice(field["options"]) {
			o := toMap(opt)
			c.optionIDs[str(o, "name")] = str(o, "id")
		}
		return nil
	}
	return fmt.Errorf("Project #%d 未找到（owner=%s）", c.projectNumber, c.owner)
}

// items 查询模板
const itemsQuery = `query($pid: ID!) { node(id: $pid) { ... on ProjectV2 { items(first: 100) { nodes {
	id
	fieldValueByName(name: "Status") { ... on ProjectV2ItemFieldSingleSelectValue { name } }
	content { ... on Issue { number title body repository { nameWithOwner }
		closedByPullRequestsReferences(first: 3) { nodes { number merged mergeable } }
	} }
} } } } }`

// FetchByStatus 拉取指定状态的 issues
func (c *Client) FetchByStatus(ctx context.Context, status string) ([]daemon.Issue, error) {
	items, err := c.fetchItems(ctx)
	if err != nil {
		return nil, err
	}
	var result []daemon.Issue
	for _, it := range items {
		if it.Status == status {
			result = append(result, it)
		}
	}
	return result, nil
}

// SetStatus 更新 issue 状态
func (c *Client) SetStatus(ctx context.Context, itemID, status string) error {
	optID, ok := c.optionIDs[status]
	if !ok {
		return fmt.Errorf("未知状态 %q", status)
	}
	m := `mutation($pid: ID!, $iid: ID!, $fid: ID!, $oid: String!) {
		updateProjectV2ItemFieldValue(input: { projectId: $pid, itemId: $iid, fieldId: $fid,
			value: { singleSelectOptionId: $oid } }) { projectV2Item { id } }
	}`
	_, err := c.gql(ctx, m, map[string]interface{}{"pid": c.projectID, "iid": itemID, "fid": c.statusFieldID, "oid": optID})
	return err
}

// GetIssue 获取单个 issue
func (c *Client) GetIssue(ctx context.Context, itemID string) (*daemon.Issue, error) {
	items, err := c.fetchItems(ctx)
	if err != nil {
		return nil, err
	}
	for _, it := range items {
		if it.ID == itemID {
			return &it, nil
		}
	}
	return nil, fmt.Errorf("item %s 未找到", itemID)
}

// CheckDependencies 批量检查依赖状态
func (c *Client) CheckDependencies(ctx context.Context, ids []string) (map[string]string, error) {
	items, err := c.fetchItems(ctx)
	if err != nil {
		return nil, err
	}
	numStatus := map[string]string{}
	for _, it := range items {
		numStatus[strconv.Itoa(it.Number)] = it.Status
	}
	result := map[string]string{}
	for _, id := range ids {
		if s, ok := numStatus[id]; ok {
			result[id] = s
		}
	}
	return result, nil
}

// fetchItems 获取所有 items 并解析为 Issue
func (c *Client) fetchItems(ctx context.Context) ([]daemon.Issue, error) {
	resp, err := c.gql(ctx, itemsQuery, map[string]interface{}{"pid": c.projectID})
	if err != nil {
		return nil, err
	}
	nodes := toSlice(dig(resp, "data", "node", "items", "nodes"))
	var result []daemon.Issue
	for _, n := range nodes {
		item := toMap(n)
		content := toMap(item["content"])
		if content == nil {
			continue
		}
		issue := daemon.Issue{
			ID:     str(item, "id"),
			Status: str(toMap(item["fieldValueByName"]), "name"),
			Number: intVal(content, "number"),
			Title:  str(content, "title"),
			Body:   str(content, "body"),
			Repo:   str(toMap(content["repository"]), "nameWithOwner"),
		}
		for _, prn := range toSlice(dig(content, "closedByPullRequestsReferences", "nodes")) {
			pr := toMap(prn)
			issue.PRNumber = intVal(pr, "number")
			issue.PRMerged, _ = pr["merged"].(bool)
			issue.PRMergeable = str(pr, "mergeable")
			break
		}
		issue.DependsOn = daemon.ParseDependsOn(issue.Body)
		result = append(result, issue)
	}
	return result, nil
}

// AddComment 在 issue 上添加评论（REST API）
func (c *Client) AddComment(ctx context.Context, issue daemon.Issue, body string) error {
	url := fmt.Sprintf("https://api.github.com/repos/%s/issues/%d/comments", issue.Repo, issue.Number)
	b, _ := json.Marshal(map[string]string{"body": body})
	req, _ := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(b))
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, respBody)
	}
	return nil
}

// MergePR 合并 PR（REST API）
func (c *Client) MergePR(ctx context.Context, issue daemon.Issue) error {
	if issue.PRNumber == 0 {
		return fmt.Errorf("issue #%d 没有关联 PR", issue.Number)
	}
	url := fmt.Sprintf("https://api.github.com/repos/%s/pulls/%d/merge", issue.Repo, issue.PRNumber)
	b, _ := json.Marshal(map[string]string{"merge_method": "squash"})
	req, _ := http.NewRequestWithContext(ctx, "PUT", url, bytes.NewReader(b))
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("合并 PR #%d 失败 HTTP %d: %s", issue.PRNumber, resp.StatusCode, respBody)
	}
	return nil
}

// --- GraphQL + JSON 辅助 ---

func (c *Client) gql(ctx context.Context, query string, vars map[string]interface{}) (map[string]interface{}, error) {
	b, _ := json.Marshal(map[string]interface{}{"query": query, "variables": vars})
	req, _ := http.NewRequestWithContext(ctx, "POST", "https://api.github.com/graphql", bytes.NewReader(b))
	req.Header.Set("Authorization", "Bearer "+c.token)
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
