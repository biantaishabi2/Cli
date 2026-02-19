package integration

import "github.com/biantaishabi2/Cli/automation/niuma/pkg/control"

// 编译期守卫：确保关键集成 mock 与 control.GitHubOps 接口保持一致。
var _ control.GitHubOps = (*controlFlowGitHubMock)(nil)
