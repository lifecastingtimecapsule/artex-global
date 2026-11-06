package server

// 2026-11-06: Verify bookkeeping-only loops are bounded without making model calls.
import (
	"github.com/Autumn-27/artex/db"
	"testing"
)

// TestLoopGuardBoundsPlanning stops before a fourth idle planner invocation.
func TestLoopGuardBoundsPlanning(t *testing.T) {
	guard := &taskLoopGuard{}
	for i := 0; i < 3; i++ {
		if guard.shouldPause(false) {
			t.Fatal("paused before three opportunities")
		}
	}
	if !guard.shouldPause(false) {
		t.Fatal("did not stop fourth call")
	}
}

// TestLoopGuardProgress distinguishes real worker results from repeated book-keeping and errors.
func TestLoopGuardProgress(t *testing.T) {
	guard := &taskLoopGuard{}
	for i := 0; i < 3; i++ {
		guard.shouldPause(false)
	}
	guard.record(db.Activity{Worker: "work#1", Kind: "tool_result", Tool: "record_fact", Summary: "waiting"})
	if !guard.shouldPause(false) {
		t.Fatal("bookkeeping reset guard")
	}
	guard.record(db.Activity{Worker: "work#1", Kind: "tool_result", Tool: "Read", Summary: "source", Detail: "new source"})
	if guard.shouldPause(false) {
		t.Fatal("real work did not reset")
	}
	guard.shouldPause(false)
	guard.shouldPause(false)
	guard.record(db.Activity{Worker: "work#1", Kind: "tool_result", Tool: "Read", Summary: "source", Detail: "new source"})
	if !guard.shouldPause(false) {
		t.Fatal("identical repeated result reset guard")
	}
	guard.record(db.Activity{Worker: "work#2", Kind: "tool_result", Tool: "Bash", Summary: "failed", IsError: true})
	if !guard.shouldPause(false) {
		t.Fatal("error reset guard")
	}
}

// TestLoopGuardIsolationAndActiveWork preserves active work and isolates unrelated tasks.
func TestLoopGuardIsolationAndActiveWork(t *testing.T) {
	engine := NewEngine(nil)
	for i := 0; i < 10; i++ {
		if engine.loopGuard("1").shouldPause(true) {
			t.Fatal("active work paused")
		}
	}
	for i := 0; i < 3; i++ {
		engine.loopGuard("1").shouldPause(false)
	}
	if !engine.loopGuard("1").shouldPause(false) {
		t.Fatal("first task not bounded")
	}
	if engine.loopGuard("2").shouldPause(false) {
		t.Fatal("second task affected")
	}
}
