package repository

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestRepositoryReadTools(t *testing.T) {
	d := t.TempDir()
	run := func(args ...string) {
		c := exec.Command("git", append([]string{"-C", d}, args...)...)
		c.Env = append(os.Environ(), "GIT_AUTHOR_NAME=test", "GIT_AUTHOR_EMAIL=test@example.com", "GIT_COMMITTER_NAME=test", "GIT_COMMITTER_EMAIL=test@example.com")
		if out, e := c.CombinedOutput(); e != nil {
			t.Fatalf("git %v: %v %s", args, e, out)
		}
	}
	run("init")
	if e := os.WriteFile(filepath.Join(d, "README.md"), []byte("hello repository\n"), 0600); e != nil {
		t.Fatal(e)
	}
	run("add", ".")
	run("commit", "-m", "initial")
	in := ConnectionInput{Path: d}
	ctx := context.Background()
	branches, e := Branches(ctx, in)
	if e != nil || len(branches) == 0 {
		t.Fatalf("branches=%v err=%v", branches, e)
	}
	content, e := File(ctx, in, "README.md", 1024)
	if e != nil || content != "hello repository\n" {
		t.Fatalf("file=%q err=%v", content, e)
	}
	matches, e := Search(ctx, in, "repository", 10)
	if e != nil || len(matches) != 1 {
		t.Fatalf("matches=%v err=%v", matches, e)
	}
	match := matches[0]
	if match["path"] != "README.md" || match["line"] != 1 || match["text"] != "hello repository" {
		t.Fatalf("unexpected search match: %#v", match)
	}

	firstRevision, e := exec.Command("git", "-C", d, "rev-parse", "HEAD").Output()
	if e != nil {
		t.Fatal(e)
	}
	if e := os.WriteFile(filepath.Join(d, "README.md"), []byte("updated content\n"), 0600); e != nil {
		t.Fatal(e)
	}
	run("add", ".")
	run("commit", "-m", "update")
	oldContent, e := FileAt(ctx, in, "README.md", 1024, string(bytes.TrimSpace(firstRevision)))
	if e != nil || oldContent != "hello repository\n" {
		t.Fatalf("revision file=%q err=%v", oldContent, e)
	}
}

func TestRepositoryRejectsHTTPSCredentialsInURL(t *testing.T) {
	_, cleanup, _, err := openRepository(context.Background(), ConnectionInput{URL: "https://user:secret@example.com/org/repo.git"}, "")
	if cleanup != nil {
		cleanup()
	}
	if err == nil || strings.Contains(err.Error(), "secret") {
		t.Fatalf("expected credential-safe URL validation error, got %v", err)
	}
}
