package agent

// 2026-11-06: Verify structured wait requests and scoped exact-intent matching without a provider.
import (
	"context"
	"encoding/json"
	"github.com/Autumn-27/artex/db"
	"testing"
)

// TestWaitForUserEmitsControl emits the question exactly once through the host control channel.
func TestWaitForUserEmitsControl(t *testing.T) {
	var events []db.Activity
	tool := waitForUserTool("work#1", func(event db.Activity) { events = append(events, event) })
	if _, err := tool.Call(context.Background(), json.RawMessage(`{"question":"Please supply authorization."}`), nil); err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 || events[0].Tool != "wait_for_user" || events[0].Worker != "work#1" {
		t.Fatal(events)
	}
	if _, err := tool.Call(context.Background(), json.RawMessage(`{"question":""}`), nil); err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 {
		t.Fatal("empty question emitted a stop")
	}
}

// TestIntentDuplicateKeysPreserveScope merges harmless formatting differences, not different asset sets.
func TestIntentDuplicateKeysPreserveScope(t *testing.T) {
	if intentDuplicateKey(" READ  source ", []int64{2, 1}) != intentDuplicateKey("read source", []int64{1, 2}) {
		t.Fatal("normalization mismatch")
	}
	if intentDuplicateKey("read source", []int64{1}) == intentDuplicateKey("read source", []int64{2}) {
		t.Fatal("different scope merged")
	}
}
