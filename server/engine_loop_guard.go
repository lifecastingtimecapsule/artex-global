package server

// 2026-11-06: Stop autonomous answer-wait loops in the engine, not through a periodic paid monitor.
import (
	"crypto/sha256"
	"fmt"
	"log"
	"strings"
	"sync"

	"github.com/Autumn-27/artex/agent"
	"github.com/Autumn-27/artex/db"
)

// taskLoopGuard counts planning opportunities without distinct successful external work.
type taskLoopGuard struct {
	mu         sync.Mutex
	progress   uint64
	observed   uint64
	idleRounds int
	seen       map[string]bool
}

// record ignores planning/graph bookkeeping and repeated identical tool results.
func (guard *taskLoopGuard) record(activity db.Activity) {
	if !strings.HasPrefix(activity.Worker, "work#") || activity.Kind != "tool_result" || activity.IsError || activity.Tool == "" {
		return
	}
	tool := strings.ToLower(activity.Tool)
	for _, prefix := range []string{"record_", "add_", "list_", "get_worker", "search_all_worker", "graph_", "node_", "prove_", "update_", "wait_for_user", "todo", "report_finding"} {
		if strings.HasPrefix(tool, prefix) {
			return
		}
	}
	fingerprint := fmt.Sprintf("%x", sha256.Sum256([]byte(tool+"\n"+activity.Summary+"\n"+activity.Detail)))
	guard.mu.Lock()
	defer guard.mu.Unlock()
	if guard.seen == nil {
		guard.seen = map[string]bool{}
	}
	if !guard.seen[fingerprint] && len(guard.seen) < 2048 {
		guard.progress++
		guard.seen[fingerprint] = true
	}
}

// shouldPause allows three no-progress rounds; active workers are allowed to finish, without raising the counter.
func (guard *taskLoopGuard) shouldPause(active bool) bool {
	guard.mu.Lock()
	defer guard.mu.Unlock()
	if guard.progress != guard.observed {
		guard.observed = guard.progress
		guard.idleRounds = 0
	}
	if active {
		return false
	}
	guard.idleRounds++
	return guard.idleRounds > 3
}

// loopGuard returns isolated per-task state; explicit Resume starts a fresh observation window.
func (e *Engine) loopGuard(taskID string) *taskLoopGuard {
	value, _ := e.loopGuards.LoadOrStore(taskID, &taskLoopGuard{})
	return value.(*taskLoopGuard)
}

// pauseForLoopGuard cancels all current autonomous calls and persists the pause before another wake can plan.
func (e *Engine) pauseForLoopGuard(t *Task, reason string) {
	if e.IsPaused(t.ID) || e.IsDeleting(t.ID) || isTerminalStatus(t.lifecycleSnapshot().Status) {
		return
	}
	e.Pause(t.ID, agent.Causef("paused_by_orchestrator", "回答待ち・進展停止", "%s", reason))
	if err := e.m.SetTaskPaused(t.ID, true); err != nil {
		log.Printf("[loop-guard] task %s persist pause failed: %v", t.ID, err)
	}
	e.emitActivity(t, db.Activity{Worker: "system", Kind: "text", Summary: "【自動一時停止】" + reason + "\n不足情報・新しい根拠・条件を補足してから、人が明示的に再開してください。"})
}
