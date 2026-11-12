package repository

import (
	"bytes"
	"context"
	"fmt"
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
	filePage, e := FileRangeAt(ctx, in, "README.md", 0, 5, "")
	if e != nil || filePage.Content != "hello" || filePage.Done || filePage.NextOffset <= filePage.Offset {
		t.Fatalf("first file range=%+v err=%v", filePage, e)
	}
	fileTail, e := FileRangeAt(ctx, in, "README.md", filePage.NextOffset, 1024, "")
	if e != nil || filePage.Content+fileTail.Content != "hello repository\n" || !fileTail.Done {
		t.Fatalf("file ranges=%+v %+v err=%v", filePage, fileTail, e)
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

func TestRepositorySearchSupportsExtendedRegularExpressionsAndIgnoreCase(t *testing.T) {
	d := t.TempDir()
	run := func(args ...string) {
		cmd := exec.Command("git", append([]string{"-C", d}, args...)...)
		cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=test", "GIT_AUTHOR_EMAIL=test@example.com", "GIT_COMMITTER_NAME=test", "GIT_COMMITTER_EMAIL=test@example.com")
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v %s", args, err, output)
		}
	}
	run("init")
	if err := os.WriteFile(filepath.Join(d, "main.go"), []byte("func StartServer() {}\nfunc stopServer() {}\nconst ignored = 1\n"), 0600); err != nil {
		t.Fatal(err)
	}
	run("add", ".")
	run("commit", "-m", "initial")

	page, err := SearchPageWithOptionsAt(context.Background(), ConnectionInput{Path: d}, SearchOptions{
		Pattern: `func (Start|stop)Server`, Limit: 10, RegularExpression: true, IgnoreCase: true,
	})
	if err != nil || len(page.Items) != 2 || !page.Done || !page.Complete {
		t.Fatalf("regex search page=%+v err=%v; want two complete matches", page, err)
	}
	if page.Items[0]["line"] != 1 || page.Items[1]["line"] != 2 {
		t.Fatalf("regex matches=%v; want lines 1 and 2", page.Items)
	}
	if _, err := SearchPageWithOptionsAt(context.Background(), ConnectionInput{Path: d}, SearchOptions{Pattern: "[", RegularExpression: true}); err == nil || !strings.Contains(strings.ToLower(err.Error()), "invalid regular expression") {
		t.Fatalf("invalid regex error=%v; want Git's actionable syntax error", err)
	}
}

func TestRepositoryTreeAllReturnsMoreThanPreviewToolLimit(t *testing.T) {
	d := t.TempDir()
	git := func(args ...string) {
		cmd := exec.Command("git", append([]string{"-C", d}, args...)...)
		cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=test", "GIT_AUTHOR_EMAIL=test@example.com", "GIT_COMMITTER_NAME=test", "GIT_COMMITTER_EMAIL=test@example.com")
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v %s", args, err, output)
		}
	}
	git("init")
	for i := 0; i < 205; i++ {
		name := filepath.Join(d, "src", fmt.Sprintf("file-%03d.txt", i))
		if err := os.MkdirAll(filepath.Dir(name), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(name, []byte("content"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	git("add", ".")
	git("commit", "-m", "initial")

	entries, err := TreeAll(context.Background(), ConnectionInput{Path: d}, "")
	if err != nil || len(entries) != 205 {
		t.Fatalf("TreeAll returned %d files, err=%v; want 205", len(entries), err)
	}
	defaultPage, err := TreePageAt(context.Background(), ConnectionInput{Path: d}, "", 0, 0, "")
	if err != nil || len(defaultPage.Items) != 205 || defaultPage.Total != 205 || !defaultPage.Done {
		t.Fatalf("default tree page=%+v err=%v; want all 205 entries in one page", defaultPage, err)
	}
	page, err := TreePageAt(context.Background(), ConnectionInput{Path: d}, "", 0, 100, "")
	if err != nil || len(page.Items) != 100 || page.Total != 205 || page.Done || page.NextOffset != 100 {
		t.Fatalf("first tree page=%+v err=%v", page, err)
	}
	last, err := TreePageAt(context.Background(), ConnectionInput{Path: d}, "", page.NextOffset, 200, "")
	if err != nil || len(last.Items) != 105 || !last.Done || last.Total != 205 {
		t.Fatalf("last tree page=%+v err=%v", last, err)
	}
	searchPage, err := SearchPageAt(context.Background(), ConnectionInput{Path: d}, "content", "", 0, 100, "")
	if err != nil || len(searchPage.Items) != 100 || searchPage.ObservedMatches != 205 || searchPage.Done || !searchPage.Complete {
		t.Fatalf("first search page=%+v err=%v", searchPage, err)
	}
	searchMiddle, err := SearchPageAt(context.Background(), ConnectionInput{Path: d}, "content", "", searchPage.NextOffset, 100, "")
	if err != nil || len(searchMiddle.Items) != 100 || searchMiddle.Done || !searchMiddle.Complete {
		t.Fatalf("middle search page=%+v err=%v", searchMiddle, err)
	}
	searchLast, err := SearchPageAt(context.Background(), ConnectionInput{Path: d}, "content", "", searchMiddle.NextOffset, 100, "")
	if err != nil || len(searchLast.Items) != 5 || !searchLast.Done || !searchLast.Complete {
		t.Fatalf("last search page=%+v err=%v", searchLast, err)
	}
}

func TestRepositoryFileRangeReadsFilesLargerThanLegacyLimit(t *testing.T) {
	d := t.TempDir()
	run := func(args ...string) {
		cmd := exec.Command("git", append([]string{"-C", d}, args...)...)
		cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=test", "GIT_AUTHOR_EMAIL=test@example.com", "GIT_COMMITTER_NAME=test", "GIT_COMMITTER_EMAIL=test@example.com")
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v %s", args, err, output)
		}
	}
	run("init")
	content := bytes.Repeat([]byte("a"), 5<<20)
	content[1<<20] = 0xE2
	content[(1<<20)+1] = 0x82
	content[(1<<20)+2] = 0xAC
	if err := os.WriteFile(filepath.Join(d, "large.txt"), content, 0600); err != nil {
		t.Fatal(err)
	}
	run("add", ".")
	run("commit", "-m", "large file")

	first, err := FileRangeAt(context.Background(), ConnectionInput{Path: d}, "large.txt", 0, 1<<20, "")
	if err != nil || first.Done || first.TotalBytes != int64(len(content)) || first.NextOffset <= 0 {
		t.Fatalf("first range=%+v err=%v", first, err)
	}
	second, err := FileRangeAt(context.Background(), ConnectionInput{Path: d}, "large.txt", first.NextOffset, 1<<20, "")
	if err != nil || !strings.Contains(second.Content, "€") {
		t.Fatalf("UTF-8 range=%+v err=%v", second, err)
	}
	last, err := FileRangeAt(context.Background(), ConnectionInput{Path: d}, "large.txt", int64(len(content)-(1<<20)), 1<<20, "")
	if err != nil || !last.Done || last.TotalBytes != int64(len(content)) {
		t.Fatalf("last range=%+v err=%v", last, err)
	}
}

func TestLimitedBufferMarksIncompleteRepositorySearchOutput(t *testing.T) {
	buffer := &limitedBuffer{max: 4}
	if n, err := buffer.Write([]byte("complete")); err != nil || n != len("complete") {
		t.Fatalf("write = %d, %v", n, err)
	}
	if string(buffer.Bytes()) != "comp" || !buffer.Truncated {
		t.Fatalf("buffer = %q, truncated=%v", buffer.Bytes(), buffer.Truncated)
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

func TestRepositoryReadToolsFallbackWhenBareHEADPointsToMissingBranch(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "source")
	bare := filepath.Join(root, "repo.git")
	run := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=test", "GIT_AUTHOR_EMAIL=test@example.com", "GIT_COMMITTER_NAME=test", "GIT_COMMITTER_EMAIL=test@example.com")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v %s", args, err, out)
		}
	}
	run("init", source)
	run("-C", source, "symbolic-ref", "HEAD", "refs/heads/main")
	if err := os.WriteFile(filepath.Join(source, "README.md"), []byte("bundle repository\n"), 0600); err != nil {
		t.Fatal(err)
	}
	run("-C", source, "add", ".")
	run("-C", source, "commit", "-m", "initial")
	run("init", "--bare", bare)
	run("--git-dir", bare, "fetch", source, "+refs/heads/*:refs/heads/*")
	run("--git-dir", bare, "symbolic-ref", "HEAD", "refs/heads/master")

	in := ConnectionInput{Path: bare}
	entries, err := Tree(context.Background(), in, "", 20)
	if err != nil || len(entries) != 1 || entries[0].Path != "README.md" {
		t.Fatalf("tree=%v err=%v", entries, err)
	}
	content, err := File(context.Background(), in, "README.md", 1024)
	if err != nil || content != "bundle repository\n" {
		t.Fatalf("file=%q err=%v", content, err)
	}
	matches, err := Search(context.Background(), in, "bundle repository", 10)
	if err != nil || len(matches) != 1 || matches[0]["path"] != "README.md" {
		t.Fatalf("search=%v err=%v", matches, err)
	}
	status, err := Status(context.Background(), in)
	if err != nil || status["head"] == "" || status["branch"] != "main" || status["has_worktree"] != false || status["worktree_status"] != "not_applicable" {
		t.Fatalf("bundle status=%v err=%v", status, err)
	}
	metadata, err := Metadata(context.Background(), in)
	if err != nil || metadata["resolved_revision"] == "" || metadata["has_remote"] != false {
		t.Fatalf("bundle metadata=%v err=%v", metadata, err)
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
	t.Setenv("OPSK_STORAGE_BACKEND", "local")
	t.Setenv("OPSK_REPOSITORY_LOCAL_ROOT", t.TempDir())
	t.Setenv("OPSK_STORAGE_S3_ENDPOINT", "https://storage.example.com")
	t.Setenv("OPSK_STORAGE_S3_BUCKET", "opskeeper")
	t.Setenv("OPSK_STORAGE_S3_PREFIX", "bundles")

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
