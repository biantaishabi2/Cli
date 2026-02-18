package control

// 编译期守卫：确保关键测试 mock 与 GitHubOps 接口保持一致。
var _ GitHubOps = (*mockGitHubOps)(nil)
