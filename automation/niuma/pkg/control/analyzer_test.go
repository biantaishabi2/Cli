package control

import (
	"context"
	"fmt"
	"testing"

	"github.com/biantaishabi2/Cli/automation/niuma/pkg/ai"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseDependsOn_Single(t *testing.T) {
	deps := parseDependsOn("depends-on: #40")
	assert.Equal(t, []int{40}, deps)
}

func TestParseDependsOn_Multiple(t *testing.T) {
	deps := parseDependsOn("some text\ndepends-on: #40, #41\nmore text")
	assert.Equal(t, []int{40, 41}, deps)
}

func TestParseDependsOn_NoDeps(t *testing.T) {
	deps := parseDependsOn("no dependencies here")
	assert.Nil(t, deps)
}

func TestParseDependsOn_CaseInsensitive(t *testing.T) {
	deps := parseDependsOn("Depends-On: #42")
	assert.Equal(t, []int{42}, deps)
}

func TestParseDependsOn_WithSpaces(t *testing.T) {
	deps := parseDependsOn("depends on: #40 #41")
	assert.Equal(t, []int{40, 41}, deps)
}

func TestAnalyze_DependsOnOnly(t *testing.T) {
	analyzer := NewDependencyAnalyzer(nil) // 无 AI provider

	issues := []IssueInfo{
		{Number: 40, Title: "Auth", Body: "fix auth"},
		{Number: 41, Title: "Payment", Body: "fix payment"},
		{Number: 42, Title: "Tests", Body: "depends-on: #40, #41"},
	}

	result, err := analyzer.Analyze(context.Background(), issues)
	require.NoError(t, err)
	assert.Equal(t, []int{40, 41}, result.Dependencies[42])
	assert.Empty(t, result.Dependencies[40])
	assert.Empty(t, result.Dependencies[41])
}

func TestAnalyze_AIAnalysis(t *testing.T) {
	mockAI := ai.NewMockProvider(`{"dependencies": {"42": [40]}, "potential_conflicts": [[41, 42]]}`)
	analyzer := NewDependencyAnalyzer(mockAI)

	issues := []IssueInfo{
		{Number: 40, Title: "Auth", Body: "fix auth"},
		{Number: 41, Title: "Payment", Body: "fix payment"},
		{Number: 42, Title: "Tests", Body: "add tests"},
	}

	result, err := analyzer.Analyze(context.Background(), issues)
	require.NoError(t, err)
	assert.Equal(t, []int{40}, result.Dependencies[42])
	assert.Equal(t, [][]int{{41, 42}}, result.PotentialConflicts)
}

func TestAnalyze_AIFailureFallback(t *testing.T) {
	mockAI := ai.NewMockProviderWithError(fmt.Errorf("API timeout"))
	analyzer := NewDependencyAnalyzer(mockAI)

	issues := []IssueInfo{
		{Number: 40, Title: "Auth", Body: "fix auth"},
		{Number: 41, Title: "Payment", Body: "fix payment"},
	}

	// AI 失败不返回 error，而是 fallback 到全部独立
	result, err := analyzer.Analyze(context.Background(), issues)
	require.NoError(t, err)
	assert.Empty(t, result.Dependencies)
}

func TestAnalyze_ManualOverridesAI(t *testing.T) {
	// AI 认为 42 依赖 41，但人工声明 42 依赖 40
	mockAI := ai.NewMockProvider(`{"dependencies": {"42": [41]}, "potential_conflicts": []}`)
	analyzer := NewDependencyAnalyzer(mockAI)

	issues := []IssueInfo{
		{Number: 40, Title: "Auth", Body: "fix auth"},
		{Number: 41, Title: "Payment", Body: "fix payment"},
		{Number: 42, Title: "Tests", Body: "depends-on: #40"},
	}

	result, err := analyzer.Analyze(context.Background(), issues)
	require.NoError(t, err)
	// 人工声明优先：42 依赖 40，不是 41
	assert.Equal(t, []int{40}, result.Dependencies[42])
}

func TestAnalyze_FilterInvalidDeps(t *testing.T) {
	analyzer := NewDependencyAnalyzer(nil)

	issues := []IssueInfo{
		{Number: 40, Title: "Auth", Body: "fix auth"},
		{Number: 42, Title: "Tests", Body: "depends-on: #40, #99"}, // #99 不在集合中
	}

	result, err := analyzer.Analyze(context.Background(), issues)
	require.NoError(t, err)
	assert.Equal(t, []int{40}, result.Dependencies[42])
}

func TestAnalyze_SelfDependencyFiltered(t *testing.T) {
	analyzer := NewDependencyAnalyzer(nil)

	issues := []IssueInfo{
		{Number: 40, Title: "Auth", Body: "depends-on: #40"}, // 自引用
	}

	result, err := analyzer.Analyze(context.Background(), issues)
	require.NoError(t, err)
	assert.Empty(t, result.Dependencies[40])
}

func TestExtractJSON_PlainJSON(t *testing.T) {
	input := `{"dependencies": {}, "potential_conflicts": []}`
	assert.Equal(t, input, extractJSON(input))
}

func TestExtractJSON_MarkdownBlock(t *testing.T) {
	input := "```json\n{\"dependencies\": {}}\n```"
	assert.Equal(t, `{"dependencies": {}}`, extractJSON(input))
}

func TestParseAnalysisResponse(t *testing.T) {
	resp := `{"dependencies": {"42": [40, 41]}, "potential_conflicts": [[40, 41]]}`
	result, err := parseAnalysisResponse(resp)
	require.NoError(t, err)
	assert.Equal(t, []int{40, 41}, result.Dependencies[42])
	assert.Equal(t, [][]int{{40, 41}}, result.PotentialConflicts)
}
