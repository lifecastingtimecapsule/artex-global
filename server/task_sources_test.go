package server

// 2026-11-05: Exercise source import containment and URL validation without any external requests.
import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestSourcePathGuards prevents traversal and catches common exclusions, including nested keys.
func TestSourcePathGuards(t *testing.T) {
	for _, relative := range []string{"../secret", "/absolute", "a/../b", "a//b", "C:/secret", "a\\b", "a\nfile"} {
		if safeSourcePath(relative) {
			t.Fatalf("accepted %q", relative)
		}
	}
	for _, relative := range []string{"project/src/main.ts", "日本語/文書.txt"} {
		if !safeSourcePath(relative) {
			t.Fatalf("rejected %q", relative)
		}
	}
	for _, relative := range []string{"project/.env.production", "project/node_modules/x.js", "keys/private.pem", ".git/config"} {
		if !excludedSourcePath(relative) {
			t.Fatalf("not excluded %q", relative)
		}
	}
	if excludedSourcePath("src/environment.ts") {
		t.Fatal("ordinary source excluded")
	}
}

// TestSourceRepositoryURLGuards rejects alternative hosts, userinfo and extra URL components.
func TestSourceRepositoryURLGuards(t *testing.T) {
	for _, raw := range []string{"http://github.com/a/b", "https://github.com.evil/a/b", "https://user@github.com/a/b", "https://github.com/a/b/tree/main", "https://github.com/a/b?token=x", "https://github.com/../x", "https://127.0.0.1/a/b"} {
		if _, _, err := githubRepository(raw); err == nil {
			t.Fatalf("accepted %q", raw)
		}
	}
	owner, repo, err := githubRepository("https://github.com/owner/repo.git")
	if err != nil || owner != "owner" || repo != "repo" {
		t.Fatal(owner, repo, err)
	}
}

// TestSourceBatchConfinement preserves hierarchy, rejects collisions and blocks symlink writes.
func TestSourceBatchConfinement(t *testing.T) {
	batch, err := newSourceBatch(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer batch.finish(false)
	if err = batch.add("project/src/index.ts", strings.NewReader("hello")); err != nil {
		t.Fatal(err)
	}
	if len(batch.files) != 1 || batch.files[0].Name != "project/src/index.ts" || batch.total != 5 {
		t.Fatal(batch.files, batch.total)
	}
	if err = batch.add("project/src/index.ts", strings.NewReader("overwrite")); err == nil {
		t.Fatal("collision accepted")
	}
	if err = batch.add("../escape", strings.NewReader("escape")); err == nil {
		t.Fatal("traversal accepted")
	}
	if err = batch.add("project/.env", strings.NewReader("secret")); err != nil || batch.skipped != 1 {
		t.Fatal(err, batch.skipped)
	}
	outside := t.TempDir()
	if err = os.Symlink(outside, filepath.Join(batch.dir, "link")); err != nil {
		t.Fatal(err)
	}
	if err = batch.add("link/escape.txt", strings.NewReader("escape")); err == nil {
		t.Fatal("symlink accepted")
	}
	if _, err = os.Stat(filepath.Join(outside, "escape.txt")); !os.IsNotExist(err) {
		t.Fatal("outside modified")
	}
	batch.total = sourceMaxBytes
	if err = batch.add("too-big.txt", strings.NewReader("x")); err == nil {
		t.Fatal("byte limit accepted")
	}
}
