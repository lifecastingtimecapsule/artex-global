"use client";

import { useMemo } from "react";
import { useActiveLocale } from "@/i18n/locale-store";

// 2026-11-05: Human-readable summaries for shipped definitions; never used to assemble model tools or authorization rules.
const TOOL_SUMMARIES: Record<string, string> = {
  "add_company_scope": "企業の資産範囲にドメイン・IP・ネットワーク・企業キーワードを追加します。所属根拠が必要で、広すぎる範囲は拒否されます。",
  "add_hint": "人間や主担当からの方針を探索記録へ追加し、次の計画で参照できるようにします。",
  "add_intent": "新しい探索・検証の方向性を探索記録へ追加します。",
  "add_task_hint": "指定したタスクへ方針を追加し、次の計画で参照できるようにします。",
  "add_task_scope": "現在のタスクの検証対象範囲を追加します。範囲を広げる操作のため、根拠の記録が必要です。",
  "bind_finding_traffic": "登録済みの脆弱性に、確認した実際のHTTP通信を証拠として紐付けます。追加後は報告書が更新待ちになります。",
  "create_custom_tool": "カスタムツールの説明・実行仕様と担当を登録します。",
  "create_mcp": "外部MCPサーバーを登録します。利用には担当ごとの公開設定が必要です。",
  "create_skill": "仕様に沿ったSKILL.mdを作成し、新しいスキルを登録します。",
  "delete_assets_by_host": "指定ホストの資産と関連サービス・エンドポイントを完全削除します。共有資産に作用し、元に戻せません。",
  "expand_digest": "圧縮された探索記録の構成要素を一覧表示します。詳しい証拠は個別の記録から取得します。",
  "get_finding_retest_context": "現在の再検証セッションに関連する証拠、状態、補足説明、制約を読み取ります。",
  "get_finding_traffic": "脆弱性に紐付いた実際の通信証拠を、一覧または分割した本文で読み取ります。独立した脆弱性IDを使います。",
  "get_task_graph": "指定タスクの資産・探索状況・発見・カバレッジの概要を読み取ります。",
  "get_task_node_detail": "指定タスクの探索記録の詳細と証拠を読み取ります。対象は探索ノードIDです。",
  "get_task_worker_trace": "指定タスク内の実行手順を概要または選択した手順の詳細として読み取ります。",
  "get_worker_output": "対象の探索処理の最終結果、または中止までに得られた結果を読み取ります。",
  "get_worker_trace": "探索処理の手順を一覧・検索・選択した詳細として読み取ります。",
  "goal_met": "タスク全体の完了を確定します。単一の目標だけの達成や、計画の一区切りには使用しません。",
  "graph_overview": "資産、未探索の方向、発見、人間からの方針など、現在の探索状況を要約します。",
  "insert_assets": "発見した資産を種類ごとの必須項目に従って一括登録します。認証・技術・パラメーター情報は追加統合します。",
  "kill_work": "指定した実行中の探索処理を中止します。中止した処理は自動では再開されません。",
  "list_assets": "現在のタスクと直接関連するタスクの対象資産を、IDまたは条件式で検索します。",
  "list_companies": "登録企業、資産範囲、所属資産数を一覧表示します。",
  "list_facts": "現在のタスクと関連タスクの探索事実・結論を、検索やページ分割で読み取ります。",
  "list_findings": "現在のタスクと関連タスクで確認された脆弱性を一覧表示します。",
  "list_goals": "現在のタスクの目標と達成状態を一覧表示します。",
  "list_llm_profiles": "利用可能なモデル接続設定の識別子、形式、モデル、有効状態を一覧表示します。APIキーは返しません。",
  "list_task_findings": "指定タスクの確認された脆弱性と証拠の概要を読み取ります。",
  "list_task_worker_traces": "指定タスクの探索処理と手順数を一覧表示します。",
  "list_tasks": "すべてのタスクの概要、状態、実行時間、使用するモデル設定を一覧表示します。",
  "list_untested_assets": "現在のタスクと関連タスクの対象範囲で、探索記録がまだ紐付いていない資産を探します。",
  "node_detail": "現在のタスクまたは関連タスクの探索ノードの全文を読み取ります。資産の詳細取得とは別です。",
  "pause_task": "指定したタスクの計画・実行ループを一時停止します。",
  "prove_goal": "証拠となる記録を目標に紐付け、その目標を達成済みにします。",
  "record_fact": "実際に観察した探索事実や結論を、証拠とともに探索記録へ保存します。",
  "record_finding_retest_result": "現在の再検証の結論を保存します。修正確認が成功すると脆弱性の状態が修正済みになります。",
  "report_finding": "確認した脆弱性を、検証できる証拠とともに登録します。脆弱性IDと探索ノードIDは別です。",
  "search_all_worker_traces": "ほかの探索処理の手順をキーワードで検索し、概要と手順IDを取得します。",
  "search_task_worker_traces": "指定タスクの実行手順をキーワードで検索します。",
  "set_constraints": "現在のタスクで明示された許可・禁止事項を制約として登録します。",
  "set_goals": "現在のタスクに、達成を確認できる目標を追加します。追加だけでは達成扱いになりません。",
  "spawn_task": "新しい子タスクを作成し、探索エンジンを開始します。実際の処理が始まる操作です。",
  "steer_work": "実行中の探索処理に、次の動作から適用する修正方針を追加します。",
  "traffic_blob": "大きなHTTPリクエスト・レスポンスの保存済み本文を、位置を指定して分割取得します。",
  "traffic_get": "保存済みのHTTP通信をIDで読み取ります。大きい本文は分割取得します。",
  "traffic_search": "指定したホストの保存済み通信を検索します。脆弱性の証拠には、元のリクエスト・レスポンスの確認が必要です。",
  "update_custom_tool": "指定したカスタムツールの登録内容を変更します。",
  "update_finding_report": "登録済み脆弱性の詳細なMarkdown報告書を保存・更新します。探索ノードIDと実際に読んだ証拠の版を使います。",
  "update_mcp": "指定したMCPサーバーの登録内容を変更します。",
  "update_skill_file": "指定したスキル内のファイルを作成・更新します。"
};

const SKILL_SUMMARIES: Record<string, string> = {
  "api-recon": "WebサイトのAPIエンドポイントを調査し、一覧と根拠を整理するスキルです。",
  "playwright-cli": "ブラウザー操作、Web画面の確認、Playwrightによるテストを支援するスキルです。",
  "scopesentry": "ScopeSentryと連携して対象資産の情報を扱うスキルです。接続と公開設定が必要です。",
  "security-audit": "Cloudflareのセキュリティ監査手順です。ソース調査、候補の独立確認、根拠に基づく報告を支援します。完全な監査は明示的な依頼時に行い、対象コードの実行には別の隔離環境が必要です。"
};

const RULE_NAMES: Record<string, string> = {
  "[内置] 递归强制删除 rm -rf": "[組み込み] 再帰的な強制削除 rm -rf",
  "[内置] 删除系统关键目录": "[組み込み] システムの重要ディレクトリの削除",
  "[内置] 磁盘格式化 mkfs": "[組み込み] ディスクの初期化 mkfs",
  "[内置] 覆写磁盘设备 dd": "[組み込み] ディスクデバイスの上書き dd",
  "[内置] Fork 炸弹": "[組み込み] プロセスの大量生成（フォークボム）",
  "[内置] 关机 / 重启": "[組み込み] シャットダウン・再起動",
  "[内置] 杀死全部进程": "[組み込み] すべてのプロセスの強制終了",
  "[内置] 磁盘擦除 shred / wipe": "[組み込み] ディスク内容の消去 shred / wipe",
  "[内置] 清空防火墙规则": "[組み込み] ファイアウォールルールの全消去",
  "[内置] SQL DROP DATABASE / TABLE / SCHEMA": "[組み込み] SQLのデータベース・テーブル・スキーマ削除",
  "[内置] SQL TRUNCATE": "[組み込み] SQLのテーブル全件削除（TRUNCATE）",
  "[内置] MongoDB drop / dropDatabase": "[組み込み] MongoDBのコレクション・データベース削除",
  "[内置] Redis FLUSHALL / FLUSHDB": "[組み込み] Redisのデータ全消去",
  "[内置] curl / wget 发送 DELETE 请求": "[組み込み] curl・wgetによるDELETEリクエスト",
  "[内置] Python HTTP 客户端 DELETE（requests/httpx/aiohttp）": "[組み込み] Python HTTPクライアントのDELETE操作",
  "[内置] 脚本中声明 HTTP DELETE 方法（JS/通用）": "[組み込み] スクリプト内のHTTP DELETE指定",
  "[内置] 批量清空 / 清除接口路径": "[組み込み] 一括消去・初期化のAPIパス",
  "[内置] 破坏性系统命令": "[組み込み] 破壊的なシステムコマンド",
  "[内置] 数据外泄管道": "[組み込み] データを外部へ送るパイプ処理",
  "[内置] 删除类接口路径": "[組み込み] 削除操作のAPIパス",
  "[内置] 政府网站 (.gov)": "[組み込み] 政府機関のサイト (.gov)",
  "[内置] 政府网站 (.gov.cn)": "[組み込み] 政府機関のサイト (.gov.cn)",
  "[内置] 教育网站 (.edu)": "[組み込み] 教育機関のサイト (.edu)",
  "[内置] 教育网站 (.edu.cn)": "[組み込み] 教育機関のサイト (.edu.cn)"
};

/** Return locale-aware display text while retaining all stored AI definitions and user-created content. */
export function useReviewedDisplayLabels() {
  const locale = useActiveLocale();
  return useMemo(() => ({
    agentKey: (key: string) => locale === "ja" ? ({ goals: "目標分解", planner: "計画立案", mainagent: "メイン", worker: "実行", auto: "自動運用", pentest: "ペネトレーションテスト", reporter: "報告書作成", retester: "再検証" } as Record<string, string>)[key] ?? key : key,
    toolDescription: (key: string, original?: string) => locale === "ja" ? (TOOL_SUMMARIES[key] ?? original ?? "") : (original ?? ""),
    skillDescription: (key: string, original?: string) => locale === "ja" ? (SKILL_SUMMARIES[key] ?? original ?? "") : (original ?? ""),
    ruleName: (original: string) => locale === "ja" ? (RULE_NAMES[original] ?? original) : original,
  }), [locale]);
}
