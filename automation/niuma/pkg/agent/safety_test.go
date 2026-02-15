package agent

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValidateChanges_AllAllowed(t *testing.T) {
	changes := []FileChange{
		{Path: "src/auth.go", Action: "modify"},
		{Path: "lib/utils.go", Action: "modify"},
		{Path: "tests/auth_test.go", Action: "create"},
		{Path: "pkg/core/core.go", Action: "modify"},
	}

	result := ValidateChanges(changes)
	assert.True(t, result.IsClean())
	assert.Len(t, result.Allowed, 4)
	assert.Empty(t, result.Rejected)
	assert.Empty(t, result.HighRisk)
}

func TestValidateChanges_Rejected(t *testing.T) {
	changes := []FileChange{
		{Path: "src/auth.go", Action: "modify"},
		{Path: "config/database.yml", Action: "modify"}, // 不在白名单
		{Path: "scripts/deploy.sh", Action: "create"},    // 不在白名单
	}

	result := ValidateChanges(changes)
	assert.False(t, result.IsClean())
	assert.Len(t, result.Allowed, 1)
	assert.Len(t, result.Rejected, 2)
	assert.Contains(t, result.Rejected, "config/database.yml")
	assert.Contains(t, result.Rejected, "scripts/deploy.sh")
}

func TestValidateChanges_HighRisk(t *testing.T) {
	changes := []FileChange{
		{Path: "src/main.go", Action: "modify"},
		{Path: ".github/workflows/ci.yml", Action: "modify"},
		{Path: "Dockerfile", Action: "modify"},
		{Path: "auth/login.go", Action: "modify"},
	}

	result := ValidateChanges(changes)
	assert.True(t, result.IsClean()) // 高风险不会被拒绝
	assert.Len(t, result.HighRisk, 3)
	assert.Contains(t, result.HighRisk, ".github/workflows/ci.yml")
	assert.Contains(t, result.HighRisk, "Dockerfile")
	assert.Contains(t, result.HighRisk, "auth/login.go")
}

func TestIsHighRiskChange(t *testing.T) {
	tests := []struct {
		path     string
		expected bool
	}{
		{"auth/login.go", true},
		{".github/workflows/ci.yml", true},
		{"Dockerfile", true},
		{"deploy/scripts/run.sh", true},
		{"infra/terraform/main.tf", true},
		{"docker-compose.yml", true},
		{"src/auth.go", false},
		{"lib/utils.go", false},
		{"tests/test.go", false},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			assert.Equal(t, tt.expected, IsHighRiskChange(tt.path))
		})
	}
}

func TestFormatValidationError(t *testing.T) {
	result := &ValidationResult{
		Rejected: []string{"config/db.yml", "scripts/hack.sh"},
		HighRisk: []string{".github/workflows/ci.yml"},
	}

	formatted := FormatValidationError(result)
	assert.Contains(t, formatted, "路径校验失败")
	assert.Contains(t, formatted, "config/db.yml")
	assert.Contains(t, formatted, "高风险")
}

func TestFormatValidationError_Clean(t *testing.T) {
	result := &ValidationResult{
		Allowed: []string{"src/main.go"},
	}
	assert.Empty(t, FormatValidationError(result))
}
