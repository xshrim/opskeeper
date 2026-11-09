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

func TestRepositoryDirectoryScopesSourceQueries(t *testing.T) {
	d := t.TempDir()
	run := func(args ...string) {
		c := exec.Command("git", append([]string{"-C", d}, args...)...)
		c.Env = append(os.Environ(), "GIT_AUTHOR_NAME=test", "GIT_AUTHOR_EMAIL=test@example.com", "GIT_COMMITTER_NAME=test", "GIT_COMMITTER_EMAIL=test@example.com")
		if out, err := c.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v %s", args, err, out)
		}
	}
	run("init")
	if err := os.MkdirAll(filepath.Join(d, "services", "orders"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(d, "services", "orders", "main.go"), []byte("orders source\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(d, "README.md"), []byte("root source\n"), 0600); err != nil {
		t.Fatal(err)
	}
	run("add", ".")
	run("commit", "-m", "initial")
	in := ConnectionInput{Path: d, Directory: "services/orders"}
	entries, err := Tree(context.Background(), in, "", 20)
	if err != nil || len(entries) != 1 || entries[0].Path != "main.go" {
		t.Fatalf("scoped tree=%v err=%v", entries, err)
	}
	content, err := File(context.Background(), in, "main.go", 1024)
	if err != nil || content != "orders source\n" {
		t.Fatalf("scoped file=%q err=%v", content, err)
	}
	matches, err := Search(context.Background(), in, "orders source", 10)
	if err != nil || len(matches) != 1 || matches[0]["path"] != "main.go" {
		t.Fatalf("scoped search=%v err=%v", matches, err)
	}
	if _, err := File(context.Background(), in, "../README.md", 1024); err == nil {
		t.Fatal("expected scoped path traversal to be rejected")
	}
}

func TestPreparePinsOneRepositorySnapshot(t *testing.T) {
	d := t.TempDir()
	run := func(args ...string) {
		cmd := exec.Command("git", append([]string{"-C", d}, args...)...)
		cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=test", "GIT_AUTHOR_EMAIL=test@example.com", "GIT_COMMITTER_NAME=test", "GIT_COMMITTER_EMAIL=test@example.com")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v %s", args, err, out)
		}
	}
	run("init")
	if err := os.WriteFile(filepath.Join(d, "main.go"), []byte("version one\n"), 0600); err != nil {
		t.Fatal(err)
	}
	run("add", ".")
	run("commit", "-m", "one")
	prepared, cleanup, err := Prepare(context.Background(), ConnectionInput{URL: "file://" + d})
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	defer cleanup()
	if prepared.PinnedRevision == "" {
		t.Fatal("Prepare did not pin the repository HEAD")
	}
	if err := os.WriteFile(filepath.Join(d, "main.go"), []byte("version two\n"), 0600); err != nil {
		t.Fatal(err)
	}
	run("add", ".")
	run("commit", "-m", "two")
	content, err := File(context.Background(), prepared, "main.go", 1024)
	if err != nil || content != "version one\n" {
		t.Fatalf("snapshot content=%q err=%v", content, err)
	}
	content, err = FileAt(context.Background(), prepared, "main.go", 1024, "invalid:revision")
	if err != nil || content != "version one\n" {
		t.Fatalf("pinned snapshot content=%q err=%v", content, err)
	}
}

func TestConfiguredBundleStorageUsesProcessEnvironment(t *testing.T) {
	t.Setenv("OPSK_REPOSITORY_STORAGE_BACKEND", "local")
	t.Setenv("OPSK_REPOSITORY_LOCAL_ROOT", t.TempDir())
	t.Setenv("OPSK_REPOSITORY_S3_ENDPOINT", "https://storage.example.com")
	t.Setenv("OPSK_REPOSITORY_S3_BUCKET", "opskeeper")
	t.Setenv("OPSK_REPOSITORY_S3_PREFIX", "bundles")

	bundle := configuredBundleStorage(ConnectionInput{
		Path:           "/resource-selected-path",
		StorageBackend: "local",
		StorageKey:     "resource-id",
		S3Endpoint:     "https://resource-selected.example.com",
		S3Bucket:       "resource-selected",
		S3Prefix:       "resource-selected",
	})
	if bundle.StorageBackend != "local" {
		t.Fatalf("storage backend = %q, want local", bundle.StorageBackend)
	}
	if bundle.Path == "/resource-selected-path" {
		t.Fatal("bundle path still came from resource configuration")
	}
	if bundle.S3Endpoint != "https://storage.example.com" || bundle.S3Bucket != "opskeeper" || bundle.S3Prefix != "bundles" {
		t.Fatalf("bundle S3 configuration = %#v, want process configuration", bundle)
	}

	git := configuredBundleStorage(ConnectionInput{URL: "https://git.example.com/repo.git", StorageBackend: "local"})
	if git.StorageBackend != "local" {
		t.Fatalf("Git storage backend changed without a storage key: %q", git.StorageBackend)
	}
}
