package repository

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
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
}
