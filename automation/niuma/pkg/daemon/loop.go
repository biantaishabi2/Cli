// pkg/daemon/loop.go
// daemon 轮询主循环：驱动 issue 状态流转
package daemon

import (
	"context"
	"errors"
	"fmt"
	"log"
	"sync"
	"time"
)

// Config daemon 运行配置
type Config struct {
	PollInterval  time.Duration     // 轮询间隔（默认 15s）
	MaxConcurrent int               // 最大并行任务数（默认 1）
	MaxRetries    int               // 单个 issue 最大重试次数（默认 3，0=无限）
	StatusMapping map[string]string // 状态名映射（内部名 → tracker 实际值）
}

// DefaultConfig 返回默认配置
func DefaultConfig() *Config {
	return &Config{
		PollInterval:  15 * time.Second,
		MaxConcurrent: 1,
		MaxRetries:    3,
		StatusMapping: map[string]string{
			"queued":  StatusQueued,
			"working": StatusWorking,
			"review":  StatusReview,
			"done":    StatusDone,
		},
	}
}

// mapStatus 将内部状态名映射到 tracker 实际值
func (c *Config) mapStatus(internal string) string {
	if mapped, ok := c.StatusMapping[internal]; ok {
		return mapped
	}
	// 回退：首字母大写
	if len(internal) > 0 {
		return string(internal[0]-32) + internal[1:]
	}
	return internal
}

// Daemon 轮询守护进程
type Daemon struct {
	tracker  Tracker
	executor Executor
	config   *Config

	mu      sync.Mutex
	running map[string]bool // 正在执行的 issue ID → 防重复派发
	retries map[string]int  // issue ID → 连续失败次数
	sem     chan struct{}    // 并发控制信号量
}

// New 创建 daemon 实例
func New(tracker Tracker, executor Executor, config *Config) *Daemon {
	if config == nil {
		config = DefaultConfig()
	}
	if config.MaxConcurrent <= 0 {
		config.MaxConcurrent = 1
	}
	return &Daemon{
		tracker:  tracker,
		executor: executor,
		config:   config,
		running:  make(map[string]bool),
		retries:  make(map[string]int),
		sem:      make(chan struct{}, config.MaxConcurrent),
	}
}

// Run 启动轮询主循环，直到 context 取消
func (d *Daemon) Run(ctx context.Context) error {
	log.Printf("[daemon] 启动，轮询间隔 %s，最大并行 %d", d.config.PollInterval, d.config.MaxConcurrent)

	// 启动时立即执行一轮
	d.tick(ctx)

	ticker := time.NewTicker(d.config.PollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Println("[daemon] 收到停止信号，等待进行中的任务完成...")
			d.waitRunning()
			log.Println("[daemon] 已停止")
			return nil
		case <-ticker.C:
			d.tick(ctx)
		}
	}
}

// tick 单轮轮询
func (d *Daemon) tick(ctx context.Context) {
	// 阶段 1: Queued → Working（依赖检查 + 派发）
	d.processQueued(ctx)

	// 阶段 2: Review → Done（PR 合并检测）
	d.processReview(ctx)
}

// processQueued 处理排队中的 issues
func (d *Daemon) processQueued(ctx context.Context) {
	statusQueued := d.config.mapStatus("queued")
	queued, err := d.tracker.FetchByStatus(ctx, statusQueued)
	if err != nil {
		log.Printf("[daemon] 拉取 %s issues 失败: %v", statusQueued, err)
		return
	}

	for _, issue := range queued {
		if ctx.Err() != nil {
			return
		}

		d.mu.Lock()
		alreadyRunning := d.running[issue.ID]
		d.mu.Unlock()
		if alreadyRunning {
			continue
		}

		// 检查重试次数
		if d.config.MaxRetries > 0 {
			d.mu.Lock()
			failures := d.retries[issue.ID]
			d.mu.Unlock()
			if failures >= d.config.MaxRetries {
				log.Printf("[daemon] #%d 已失败 %d 次（上限 %d），不再重试", issue.Number, failures, d.config.MaxRetries)
				continue
			}
		}

		// 检查依赖
		if !d.dependenciesMet(ctx, issue) {
			log.Printf("[daemon] #%d 依赖未满足，跳过", issue.Number)
			continue
		}

		// 尝试获取信号量（非阻塞）
		select {
		case d.sem <- struct{}{}:
			// 获取到并发槽位
		default:
			log.Printf("[daemon] 并发已满（%d），#%d 等待下轮", d.config.MaxConcurrent, issue.Number)
			return
		}

		// 设置状态为 Working
		statusWorking := d.config.mapStatus("working")
		if err := d.tracker.SetStatus(ctx, issue.ID, statusWorking); err != nil {
			log.Printf("[daemon] #%d 设置 %s 失败: %v", issue.Number, statusWorking, err)
			<-d.sem
			continue
		}

		d.mu.Lock()
		d.running[issue.ID] = true
		d.mu.Unlock()

		// 异步派发
		go d.dispatch(ctx, issue)
	}
}

// processReview 处理等待 review 的 issues（自动合并 + 状态推进）
func (d *Daemon) processReview(ctx context.Context) {
	statusReview := d.config.mapStatus("review")
	reviewing, err := d.tracker.FetchByStatus(ctx, statusReview)
	if err != nil {
		log.Printf("[daemon] 拉取 %s issues 失败: %v", statusReview, err)
		return
	}

	statusDone := d.config.mapStatus("done")
	for _, issue := range reviewing {
		if ctx.Err() != nil {
			return
		}

		// 已合并 → Done
		if issue.PRMerged {
			if err := d.tracker.SetStatus(ctx, issue.ID, statusDone); err != nil {
				log.Printf("[daemon] #%d 设置 %s 失败: %v", issue.Number, statusDone, err)
				continue
			}
			log.Printf("[daemon] #%d PR 已合并 → %s", issue.Number, statusDone)
			continue
		}

		// 未合并但可合并 → 自动合并
		if issue.PRNumber > 0 && issue.PRMergeable == "MERGEABLE" {
			log.Printf("[daemon] #%d PR #%d 可合并，尝试自动合并", issue.Number, issue.PRNumber)
			if err := d.tracker.MergePR(ctx, issue); err != nil {
				log.Printf("[daemon] #%d 自动合并失败: %v", issue.Number, err)
				continue
			}
			// 合并成功 → Done
			if err := d.tracker.SetStatus(ctx, issue.ID, statusDone); err != nil {
				log.Printf("[daemon] #%d 设置 %s 失败: %v", issue.Number, statusDone, err)
				continue
			}
			log.Printf("[daemon] #%d PR #%d 自动合并成功 → %s", issue.Number, issue.PRNumber, statusDone)
		}
	}
}

// dispatch 异步执行 niuma fix
func (d *Daemon) dispatch(ctx context.Context, issue Issue) {
	defer func() {
		<-d.sem
		d.mu.Lock()
		delete(d.running, issue.ID)
		d.mu.Unlock()
	}()

	log.Printf("[daemon] 开始处理 #%d: %s", issue.Number, issue.Title)

	err := d.executor.Fix(ctx, issue)
	if err != nil {
		d.failAndComment(ctx, issue, err)
		return
	}

	// 检查是否创建了 PR
	updated, _ := d.tracker.GetIssue(ctx, issue.ID)
	if updated == nil || updated.PRNumber == 0 {
		d.failAndComment(ctx, issue, fmt.Errorf("AI 执行完成但未创建 PR"))
		return
	}

	// 全部成功 → 清除重试计数
	d.mu.Lock()
	delete(d.retries, issue.ID)
	d.mu.Unlock()

	// 有 PR → Review（等 PR 合并后变 Done）
	statusReview := d.config.mapStatus("review")
	if err := d.tracker.SetStatus(ctx, issue.ID, statusReview); err != nil {
		log.Printf("[daemon] #%d 设置 %s 失败: %v", issue.Number, statusReview, err)
		return
	}
	log.Printf("[daemon] #%d 完成 → %s（PR #%d）", issue.Number, statusReview, updated.PRNumber)
}

// failAndComment 失败处理：累加重试、回退 Queued、在 issue 上留评论
func (d *Daemon) failAndComment(ctx context.Context, issue Issue, err error) {
	d.mu.Lock()
	d.retries[issue.ID]++
	failures := d.retries[issue.ID]
	d.mu.Unlock()

	log.Printf("[daemon] #%d 失败 (%d/%d): %v，回退到 Queued", issue.Number, failures, d.config.MaxRetries, err)

	statusQueued := d.config.mapStatus("queued")
	if setErr := d.tracker.SetStatus(ctx, issue.ID, statusQueued); setErr != nil {
		log.Printf("[daemon] #%d 回退状态失败: %v", issue.Number, setErr)
	}

	// 构造评论
	phase := "AI 执行"
	detail := err.Error()
	var gateErr *GateError
	if errors.As(err, &gateErr) {
		phase = "gate 验证"
		if gateErr.Output != "" {
			detail = gateErr.Output
		}
	}

	comment := fmt.Sprintf("🔴 自动处理失败（第 %d/%d 次）\n\n**阶段：** %s\n**错误：**\n```\n%s\n```\n\n下次重试将参考此反馈。",
		failures, d.config.MaxRetries, phase, detail)

	if cmtErr := d.tracker.AddComment(ctx, issue, comment); cmtErr != nil {
		log.Printf("[daemon] #%d 留言失败: %v", issue.Number, cmtErr)
	}
}

// dependenciesMet 检查 issue 的所有依赖是否已完成
func (d *Daemon) dependenciesMet(ctx context.Context, issue Issue) bool {
	deps := issue.DependsOn
	if len(deps) == 0 {
		// 没有 DependsOn 字段时尝试从 body 解析
		deps = ParseDependsOn(issue.Body)
	}
	if len(deps) == 0 {
		return true
	}

	statuses, err := d.tracker.CheckDependencies(ctx, deps)
	if err != nil {
		log.Printf("[daemon] #%d 检查依赖失败: %v", issue.Number, err)
		return false
	}

	statusDone := d.config.mapStatus("done")
	for _, dep := range deps {
		s, ok := statuses[dep]
		if !ok || s != statusDone {
			return false
		}
	}
	return true
}

// waitRunning 等待所有进行中的任务完成
func (d *Daemon) waitRunning() {
	for i := 0; i < cap(d.sem); i++ {
		d.sem <- struct{}{}
	}
	// 全部槽位回收，所有任务已完成
	for i := 0; i < cap(d.sem); i++ {
		<-d.sem
	}
}

// RunningCount 返回当前正在执行的任务数（用于测试）
func (d *Daemon) RunningCount() int {
	d.mu.Lock()
	defer d.mu.Unlock()
	return len(d.running)
}

// String 返回 daemon 状态摘要
func (d *Daemon) String() string {
	d.mu.Lock()
	n := len(d.running)
	d.mu.Unlock()
	return fmt.Sprintf("daemon(interval=%s, max=%d, running=%d)", d.config.PollInterval, d.config.MaxConcurrent, n)
}
