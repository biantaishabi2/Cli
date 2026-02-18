package control

import "context"

func (c *Controller) runDagSyncEvent(ctx context.Context) error {
	if c == nil || c.cfg == nil {
		return nil
	}
	_, err := c.syncDagToGitHub(ctx, DagSyncModeEvent, false, false)
	return err
}

func (c *Controller) maybeRunDagReconcile(ctx context.Context) error {
	if c == nil || c.cfg == nil {
		return nil
	}

	store := c.dagSyncStore
	if store == nil {
		store = newDagSyncStateStore(c.cfg.DagSync.StateFile)
		c.dagSyncStore = store
	}
	state, err := store.Load()
	if err != nil {
		// 读取失败已在 store 内降级为内存态，这里继续执行一次 reconcile。
	}

	if !shouldRunDagReconcile(state, c.cfg.DagSync.PollInterval) {
		return nil
	}
	_, err = c.syncDagToGitHub(ctx, DagSyncModeReconcile, false, false)
	return err
}

// DagSync 手动触发 DAG -> GitHub 展示同步。
func (c *Controller) DagSync(ctx context.Context, dryRun bool) (DagSyncResult, error) {
	return c.syncDagToGitHub(ctx, DagSyncModeManual, false, dryRun)
}

// DagReconcile 手动触发 DAG/GitHub 展示对账纠偏。
func (c *Controller) DagReconcile(ctx context.Context, dryRun bool) (DagSyncResult, error) {
	return c.syncDagToGitHub(ctx, DagSyncModeReconcile, false, dryRun)
}
