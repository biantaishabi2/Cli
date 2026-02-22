package agent

import (
	"context"
	"fmt"

	"github.com/biantaishabi2/Cli/automation/niuma/pkg/marker"
)

func (o *Orchestrator) upsertProgressMarker(ctx context.Context, markerType marker.Type, phase, summary string) {
	rev := 1
	if existing, err := o.github.FindMarker(ctx, o.issueNumber, markerType); err == nil && existing != nil && existing.Marker != nil {
		rev = existing.Marker.Revision + 1
	}
	m := &marker.Marker{
		Type:     markerType,
		Issue:    o.issueNumber,
		Revision: rev,
		Mode:     phase,
	}
	body := fmt.Sprintf("%s\n\n## ⏱️ 进度更新\n\n- phase: `%s`\n- summary: %s", marker.Render(m), phase, summary)
	_ = o.github.CreateOrUpdateMarker(ctx, o.issueNumber, m, body)
}

