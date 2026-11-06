package agent

// 2026-11-06: Suppress exact planner redispatches within the same task and asset scope.
import (
	"encoding/json"
	"fmt"
	"github.com/Autumn-27/artex/db"
	"sort"
	"strings"
)

// intentDuplicateKey normalizes whitespace/case while preserving the explicitly targeted asset IDs.
func intentDuplicateKey(summary string, anchors []int64) string {
	normalized := strings.ToLower(strings.Join(strings.Fields(summary), " "))
	sorted := append([]int64(nil), anchors...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })
	return fmt.Sprintf("%s|%v", normalized, sorted)
}

// duplicatePlannerIntent reuses a recent exact match; paraphrases are handled by the engine's no-progress guard.
func (t *ToolSet) duplicatePlannerIntent(item intentItem) (int64, error) {
	if t.worker != "planner" {
		return 0, nil
	}
	nodes, err := t.ts.ListByKind(db.KindIntent, 200)
	if err != nil {
		return 0, err
	}
	key := intentDuplicateKey(item.Summary, pidList(item.AssetIDs))
	for _, node := range nodes {
		var prior struct {
			Summary  string            `json:"summary"`
			AssetIDs []json.RawMessage `json:"asset_ids"`
		}
		if json.Unmarshal(node.Payload, &prior) != nil {
			continue
		}
		if intentDuplicateKey(prior.Summary, pidList(prior.AssetIDs)) == key {
			return node.ID, nil
		}
	}
	return 0, nil
}
