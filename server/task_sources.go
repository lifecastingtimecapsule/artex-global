package server

// 2026-11-05: Add bounded folder/public-GitHub imports without running target code.
import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

const sourceMaxFiles = 2000
const sourceMaxBytes int64 = 100 << 20

// safeSourcePath rejects ambiguous or escaping paths before any filesystem operation.
func safeSourcePath(relative string) bool {
	if relative == "" || len(relative) > 2048 || strings.ContainsAny(relative, "\\\x00\r\n") || strings.HasPrefix(relative, "/") {
		return false
	}
	for _, part := range strings.Split(relative, "/") {
		if part == "" || part == "." || part == ".." || strings.Contains(part, ":") {
			return false
		}
	}
	return true
}

// excludedSourcePath omits common credentials, dependencies and build artifacts; not a secret detector.
func excludedSourcePath(relative string) bool {
	for _, part := range strings.Split(strings.ToLower(relative), "/") {
		switch part {
		case ".git", "node_modules", "vendor", ".next", "dist", "build", ".venv", "venv", "__pycache__", ".ds_store", "credentials", ".npmrc", ".pypirc", ".netrc", "id_rsa", "id_ed25519":
			return true
		}
		if part == ".env" || strings.HasPrefix(part, ".env.") || strings.HasSuffix(part, ".pem") || strings.HasSuffix(part, ".key") || strings.HasSuffix(part, ".p12") || strings.HasSuffix(part, ".pfx") {
			return true
		}
	}
	return false
}

// sourceBatch confines writes to a fresh upload directory using os.Root's traversal protection.
type sourceBatch struct {
	root    *os.Root
	dir     string
	files   []chatAttachment
	total   int64
	skipped int
}

func newSourceBatch(parent string) (*sourceBatch, error) {
	if err := os.MkdirAll(parent, 0700); err != nil {
		return nil, err
	}
	directory, err := os.MkdirTemp(parent, "source-")
	if err != nil {
		return nil, err
	}
	root, err := os.OpenRoot(directory)
	if err != nil {
		os.RemoveAll(directory)
		return nil, err
	}
	return &sourceBatch{root: root, dir: directory}, nil
}

// add validates each file, prevents overwrites/symlink traversal, and limits total expanded bytes.
func (batch *sourceBatch) add(relative string, content io.Reader) error {
	if !safeSourcePath(relative) {
		return fmt.Errorf("不正な相対パスです")
	}
	if excludedSourcePath(relative) {
		batch.skipped++
		return nil
	}
	if len(batch.files) >= sourceMaxFiles {
		return fmt.Errorf("2000ファイルを超えています")
	}
	if err := batch.root.MkdirAll(path.Dir(relative), 0700); err != nil {
		return err
	}
	destination, err := batch.root.OpenFile(relative, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return err
	}
	written, copyErr := io.Copy(destination, io.LimitReader(content, sourceMaxBytes-batch.total+1))
	closeErr := destination.Close()
	if copyErr != nil {
		return copyErr
	}
	if closeErr != nil {
		return closeErr
	}
	batch.total += written
	if batch.total > sourceMaxBytes {
		return fmt.Errorf("合計100MiBを超えています")
	}
	batch.files = append(batch.files, chatAttachment{Name: relative, Path: filepath.ToSlash(filepath.Join("uploads", filepath.Base(batch.dir), relative)), Abs: filepath.Join(batch.dir, filepath.FromSlash(relative)), Size: written})
	return nil
}

// finish releases the root and removes a failed/empty batch, leaving prior uploads intact.
func (batch *sourceBatch) finish(success bool) {
	batch.root.Close()
	if !success {
		os.RemoveAll(batch.dir)
	}
}

// uploadFolder handles explicit relative paths only for pre-task staging; legacy file upload remains unchanged.
func (s *Server) uploadFolder(w http.ResponseWriter, r *http.Request, parent string) {
	files := r.MultipartForm.File["file"]
	paths := r.MultipartForm.Value["relative_path"]
	if r.URL.Query().Get("scope") != "staging" || len(files) != len(paths) || len(files) > sourceMaxFiles {
		writeErr(w, 400, "フォルダーのファイル数・相対パスが不正です")
		return
	}
	batch, err := newSourceBatch(parent)
	if err != nil {
		writeErr(w, 500, "取り込み先を作成できません")
		return
	}
	success := false
	defer func() { batch.finish(success) }()
	for index, header := range files {
		source, openErr := header.Open()
		if openErr != nil {
			writeErr(w, 400, "ファイルを読み取れません")
			return
		}
		err = batch.add(paths[index], source)
		source.Close()
		if err != nil {
			writeErr(w, 400, err.Error())
			return
		}
	}
	if len(batch.files) == 0 {
		writeErr(w, 400, "除外後に取り込めるファイルがありません")
		return
	}
	success = true
	writeJSON(w, 200, map[string]any{"attachments": batch.files, "skipped": batch.skipped})
}

var githubSourceName = regexp.MustCompile(`^[A-Za-z0-9_.-]+$`)

// githubRepository accepts only credential-free public github.com repository URLs.
func githubRepository(raw string) (string, string, error) {
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Scheme != "https" || parsed.Host != "github.com" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return "", "", fmt.Errorf("https://github.com/所有者/リポジトリ の形式で入力してください")
	}
	parts := strings.Split(strings.Trim(parsed.Path, "/"), "/")
	if len(parts) != 2 {
		return "", "", fmt.Errorf("リポジトリURLとブランチ・コミットを分けて指定してください")
	}
	parts[1] = strings.TrimSuffix(parts[1], ".git")
	for _, part := range parts {
		if !githubSourceName.MatchString(part) || part == "." || part == ".." {
			return "", "", fmt.Errorf("不正なGitHubリポジトリ名です")
		}
	}
	return parts[0], parts[1], nil
}

// fetchPublicSource uses fixed HTTPS hosts, rejects redirects, and caps response sizes/time.
func fetchPublicSource(ctx context.Context, address string, limit int64) ([]byte, error) {
	client := &http.Client{Timeout: 45 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	request, err := http.NewRequestWithContext(ctx, "GET", address, nil)
	if err != nil {
		return nil, err
	}
	request.Header.Set("User-Agent", "ARTEX-Public-Source-Import")
	request.Header.Set("Accept", "application/vnd.github+json")
	response, err := client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("GitHubへの接続に失敗しました")
	}
	defer response.Body.Close()
	if response.StatusCode != 200 {
		return nil, fmt.Errorf("GitHubがHTTP %dを返しました（公開リポジトリ・参照・利用制限を確認）", response.StatusCode)
	}
	content, err := io.ReadAll(io.LimitReader(response.Body, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(content)) > limit {
		return nil, fmt.Errorf("取得容量の上限を超えています")
	}
	return content, nil
}

// importGithubSource resolves an immutable commit and safely extracts its public source; never invokes git/hooks.
func (s *Server) importGithubSource(w http.ResponseWriter, r *http.Request) {
	var input struct {
		URL     string `json:"url"`
		Ref     string `json:"ref"`
		Subpath string `json:"subpath"`
		DraftID string `json:"draft_id"`
	}
	r.Body = http.MaxBytesReader(w, r.Body, 8192)
	if json.NewDecoder(r.Body).Decode(&input) != nil || !safeChatID.MatchString(input.DraftID) {
		writeErr(w, 400, "入力が不正です")
		return
	}
	owner, repository, err := githubRepository(input.URL)
	if err != nil {
		writeErr(w, 400, err.Error())
		return
	}
	input.Ref = strings.TrimSpace(input.Ref)
	if input.Ref == "" {
		input.Ref = "HEAD"
	}
	if len(input.Ref) > 200 || strings.ContainsAny(input.Ref, "\r\n\x00") {
		writeErr(w, 400, "参照が不正です")
		return
	}
	input.Subpath = strings.Trim(input.Subpath, "/")
	if input.Subpath != "" && !safeSourcePath(input.Subpath) {
		writeErr(w, 400, "サブフォルダーが不正です")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 90*time.Second)
	defer cancel()
	metadata, err := fetchPublicSource(ctx, "https://api.github.com/repos/"+owner+"/"+repository+"/commits/"+url.PathEscape(input.Ref), 1<<20)
	if err != nil {
		writeErr(w, 400, err.Error())
		return
	}
	var commit struct {
		SHA string `json:"sha"`
	}
	if json.Unmarshal(metadata, &commit) != nil || !regexp.MustCompile(`^[0-9a-f]{40}$`).MatchString(commit.SHA) {
		writeErr(w, 400, "コミットを特定できません")
		return
	}
	archive, err := fetchPublicSource(ctx, "https://codeload.github.com/"+owner+"/"+repository+"/zip/"+commit.SHA, sourceMaxBytes)
	if err != nil {
		writeErr(w, 400, err.Error())
		return
	}
	reader, err := zip.NewReader(bytes.NewReader(archive), int64(len(archive)))
	if err != nil || len(reader.File) > 20000 {
		writeErr(w, 400, "ZIPが不正または項目数が多すぎます")
		return
	}
	batch, err := newSourceBatch(filepath.Join(s.m.dir, "drafts", input.DraftID, "uploads"))
	if err != nil {
		writeErr(w, 500, "取り込み先を作成できません")
		return
	}
	success := false
	defer func() { batch.finish(success) }()
	for _, entry := range reader.File {
		if !safeSourcePath(strings.TrimSuffix(entry.Name, "/")) {
			writeErr(w, 400, "ZIPに不正なパスがあります")
			return
		}
		_, relative, found := strings.Cut(entry.Name, "/")
		if !found || relative == "" || entry.FileInfo().IsDir() {
			continue
		}
		if input.Subpath != "" && !strings.HasPrefix(relative, input.Subpath+"/") {
			continue
		}
		if !entry.Mode().IsRegular() {
			batch.skipped++
			continue
		}
		if entry.UncompressedSize64 > uint64(sourceMaxBytes) {
			writeErr(w, 400, "ファイルの展開サイズが大きすぎます")
			return
		}
		source, openErr := entry.Open()
		if openErr != nil {
			writeErr(w, 400, "ZIPを読み取れません")
			return
		}
		err = batch.add(relative, source)
		source.Close()
		if err != nil {
			writeErr(w, 400, err.Error())
			return
		}
	}
	if len(batch.files) == 0 {
		writeErr(w, 400, "指定範囲に取り込めるファイルがありません")
		return
	}
	success = true
	writeJSON(w, 200, map[string]any{"attachments": batch.files, "commit": commit.SHA, "skipped": batch.skipped, "bytes": batch.total})
}
