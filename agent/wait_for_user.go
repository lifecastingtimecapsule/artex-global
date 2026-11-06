package agent

// 2026-11-06: Give autonomous planner/workers a real pause path for missing human input.
import (
	"context"
	"encoding/json"
	"strings"

	"github.com/Autumn-27/artex/db"
	actool "github.com/Autumn-27/norma/tool"
)

const waitingForUserRules = `

【回答待ち・繰り返し時の停止規則：通常の新規意図生成より優先】
必要な対象・許可・資料が不足する場合、不足項目を一度だけまとめて wait_for_user(question) を呼ぶ。
「回答待ち」「再開条件の引継ぎ」「利用者回答の有無を確認」を目的に add_intent してはならない。別担当へ渡して同じ確認を繰り返さない。
新しい入力・根拠・方法がなく同じ失敗が2回続く場合も wait_for_user で停止理由と再開条件を報告する。
wait_for_user はタスク全体を一時停止する。人が回答を補足して明示的に再開するまで自動再試行しない。
時間・費用の強制上限を確認できない場合は未保証と一度伝え、上限資料の収集・引継ぎを無限に再計画しない。
`

// waitForUserTool emits a structured control request; only the engine can apply the pause barrier.
func waitForUserTool(role string, emit func(db.Activity)) actool.CoreTool {
	return writeTool("wait_for_user", "必要な利用者の回答・許可・資料を一度質問してタスク全体を一時停止する。待機の引継ぎ意図を作らない。", obj(map[string]any{"question": str("不足情報・停止理由・再開に必要な回答を日本語でまとめる")}), func(_ context.Context, raw json.RawMessage) (actool.Result, error) {
		var request struct {
			Question string `json:"question"`
		}
		if json.Unmarshal(raw, &request) != nil || strings.TrimSpace(request.Question) == "" {
			return actool.Errorf("question is required"), nil
		}
		if len([]rune(request.Question)) > 4000 {
			return actool.Errorf("question exceeds 4000 characters"), nil
		}
		if emit == nil {
			return actool.Errorf("Pause handler unavailable; ask the user to pause manually. Do not claim paused."), nil
		}
		emit(db.Activity{Worker: role, Kind: "text", Tool: "wait_for_user", Summary: "【回答待ち・一時停止要求】\n" + strings.TrimSpace(request.Question)})
		return actool.Text("Pause requested. Do not create further work. Await a human answer and explicit resume."), nil
	})
}
