package repository

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"io"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

type ConnectionInput struct {
	Path           string `json:"path,omitempty"`
	URL            string `json:"url,omitempty"`
	Branch         string `json:"branch,omitempty"`
	TimeoutSeconds int    `json:"timeout_seconds,omitempty"`
	StorageBackend string `json:"storage_backend,omitempty"`
	StorageKey     string `json:"storage_key,omitempty"`
	S3Endpoint     string `json:"s3_endpoint,omitempty"`
	S3Bucket       string `json:"s3_bucket,omitempty"`
	S3Prefix       string `json:"s3_prefix,omitempty"`
	S3AccessKey    string `json:"s3_access_key,omitempty"`
	S3SecretKey    string `json:"s3_secret_key,omitempty"`
	S3UseSSL       bool   `json:"s3_use_ssl,omitempty"`
}
type ToolInfo struct{ Name, Description string }

var tools = []ToolInfo{
	{"repository_branches", "List repository branches."},
	{"repository_checkout", "Switch the repository read branch."},
	{"repository_status", "Read repository status and HEAD."},
	{"repository_tree", "Read a bounded repository directory tree."},
	{"repository_file", "Read a bounded repository file."},
	{"repository_search", "Search text in bounded repository files."},
	{"repository_metadata", "Read repository remotes and recent commits."},
}

func ListTools() []ToolInfo { return append([]ToolInfo(nil), tools...) }
func InputSchema(extra map[string]any) map[string]any {
	p := map[string]any{"path": map[string]any{"type": "string"}, "url": map[string]any{"type": "string"}, "branch": map[string]any{"type": "string"}, "timeout_seconds": map[string]any{"type": "integer", "minimum": 1, "maximum": 300}, "storage_backend": map[string]any{"type": "string", "enum": []string{"local", "s3"}}, "storage_key": map[string]any{"type": "string"}, "s3_endpoint": map[string]any{"type": "string"}, "s3_bucket": map[string]any{"type": "string"}, "s3_prefix": map[string]any{"type": "string"}}
	for k, v := range extra {
		p[k] = v
	}
	return map[string]any{"type": "object", "properties": p, "additionalProperties": false}
}
func command(ctx context.Context, in ConnectionInput, args ...string) ([]byte, error) {
	if len(args) == 0 {
		return nil, errors.New("git command is required")
	}
	timeout := time.Duration(in.TimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	root := strings.TrimSpace(in.Path)
	cleanup := func() {}
	if root == "" && strings.EqualFold(in.StorageBackend, "s3") {
		var err error
		root, cleanup, err = materializeS3(ctx, in)
		if err != nil {
			return nil, err
		}
	}
	if root == "" && strings.TrimSpace(in.URL) != "" {
		d, err := os.MkdirTemp("", "opskeeper-repository-")
		if err != nil {
			return nil, err
		}
		cleanup = func() { _ = os.RemoveAll(d) }
		branch := strings.TrimSpace(in.Branch)
		clone := []string{"clone", "--no-tags", "--depth=1"}
		if branch != "" {
			clone = append(clone, "--branch", branch)
		}
		clone = append(clone, in.URL, d)
		if out, err := exec.CommandContext(ctx, "git", clone...).CombinedOutput(); err != nil {
			return nil, fmt.Errorf("git clone: %w: %s", err, strings.TrimSpace(string(out)))
		}
		root = d
	}
	if root == "" {
		return nil, errors.New("repository path or url is required")
	}
	clean, err := filepath.Abs(filepath.Clean(root))
	if err != nil {
		return nil, err
	}
	root = clean
	all := append([]string{"-C", root}, args...)
	out, err := exec.CommandContext(ctx, "git", all...).CombinedOutput()
	cleanup()
	if err != nil {
		return nil, fmt.Errorf("git %s: %w: %s", args[0], err, strings.TrimSpace(string(out)))
	}
	return out, nil
}
func materializeS3(ctx context.Context, in ConnectionInput) (string, func(), error) {
	if in.S3Endpoint == "" {
		in.S3Endpoint = os.Getenv("OPSK_REPOSITORY_S3_ENDPOINT")
	}
	if in.S3Bucket == "" {
		in.S3Bucket = os.Getenv("OPSK_REPOSITORY_S3_BUCKET")
	}
	if in.S3AccessKey == "" {
		in.S3AccessKey = os.Getenv("OPSK_REPOSITORY_S3_ACCESS_KEY")
	}
	if in.S3SecretKey == "" {
		in.S3SecretKey = os.Getenv("OPSK_REPOSITORY_S3_SECRET_KEY")
	}
	if in.S3Endpoint == "" || in.S3Bucket == "" || in.StorageKey == "" {
		return "", func() {}, errors.New("S3 repository endpoint, bucket and storage key are required")
	}
	endpoint := strings.TrimSpace(in.S3Endpoint)
	if parsed, e := url.Parse(endpoint); e == nil && parsed.Host != "" {
		endpoint = parsed.Host
		if parsed.Scheme == "https" {
			in.S3UseSSL = true
		}
	}
	client, err := minio.New(endpoint, &minio.Options{Creds: credentials.NewStaticV4(in.S3AccessKey, in.S3SecretKey, ""), Secure: in.S3UseSSL, BucketLookup: minio.BucketLookupPath})
	if err != nil {
		return "", func() {}, err
	}
	dir, err := os.MkdirTemp("", "opskeeper-repository-s3-")
	if err != nil {
		return "", func() {}, err
	}
	cleanup := func() { _ = os.RemoveAll(dir) }
	bundle := filepath.Join(dir, "repository.bundle")
	obj, err := client.GetObject(ctx, in.S3Bucket, in.StorageKey, minio.GetObjectOptions{})
	if err != nil {
		cleanup()
		return "", func() {}, err
	}
	f, err := os.Create(bundle)
	if err == nil {
		_, err = io.Copy(f, obj)
		_ = f.Close()
	}
	_ = obj.Close()
	if err != nil {
		cleanup()
		return "", func() {}, err
	}
	repo := filepath.Join(dir, "repo.git")
	if err = exec.CommandContext(ctx, "git", "init", "--bare", repo).Run(); err != nil {
		cleanup()
		return "", func() {}, err
	}
	if err = exec.CommandContext(ctx, "git", "--git-dir", repo, "fetch", bundle, "+refs/heads/*:refs/heads/*").Run(); err != nil {
		cleanup()
		return "", func() {}, err
	}
	return repo, cleanup, nil
}
func Branches(ctx context.Context, in ConnectionInput) ([]string, error) {
	out, e := command(ctx, in, "for-each-ref", "--format=%(refname:short)", "refs/heads", "refs/remotes")
	if e != nil {
		return nil, e
	}
	seen := map[string]bool{}
	var r []string
	for _, x := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		x = strings.TrimSpace(x)
		x = strings.TrimPrefix(x, "origin/")
		if x != "" && !seen[x] {
			seen[x] = true
			r = append(r, x)
		}
	}
	sort.Strings(r)
	return r, nil
}
func Checkout(ctx context.Context, in ConnectionInput, branch string) (map[string]any, error) {
	branch = strings.TrimSpace(branch)
	if branch == "" || strings.ContainsAny(branch, "\x00\n\r") || strings.Contains(branch, "..") {
		return nil, errors.New("invalid branch")
	}
	_, e := command(ctx, in, "symbolic-ref", "HEAD", "refs/heads/"+branch)
	if e != nil {
		_, e = command(ctx, in, "checkout", "--force", branch)
	}
	if e != nil {
		return nil, e
	}
	return map[string]any{"branch": branch}, nil
}
func Status(ctx context.Context, in ConnectionInput) (map[string]any, error) {
	head, _ := command(ctx, in, "rev-parse", "HEAD")
	br, _ := command(ctx, in, "symbolic-ref", "--short", "HEAD")
	st, e := command(ctx, in, "status", "--porcelain=v1")
	if e != nil {
		return nil, e
	}
	return map[string]any{"head": strings.TrimSpace(string(head)), "branch": strings.TrimSpace(string(br)), "clean": strings.TrimSpace(string(st)) == "", "changed_files": len(strings.Split(strings.TrimSpace(string(st)), "\n"))}, nil
}

type TreeEntry struct {
	Path string `json:"path"`
	Type string `json:"type"`
	Size int64  `json:"size,omitempty"`
}

func ref(in ConnectionInput) string {
	if strings.TrimSpace(in.Branch) != "" {
		return strings.TrimSpace(in.Branch)
	}
	return "HEAD"
}
func Tree(ctx context.Context, in ConnectionInput, path string, limit int) ([]TreeEntry, error) {
	if limit <= 0 || limit > 1000 {
		limit = 200
	}
	path, err := safePath(path)
	if err != nil {
		return nil, err
	}
	args := []string{"ls-tree", "-r", "-l", ref(in)}
	if path != "" {
		args = append(args, "--", path)
	}
	out, e := command(ctx, in, args...)
	if e != nil {
		return nil, e
	}
	var r []TreeEntry
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		f := strings.Fields(line)
		if len(f) < 4 {
			continue
		}
		n, _ := strconv.ParseInt(f[3], 10, 64)
		p := strings.Join(f[4:], " ")
		r = append(r, TreeEntry{Path: p, Type: "file", Size: n})
		if len(r) >= limit {
			break
		}
	}
	return r, nil
}
func File(ctx context.Context, in ConnectionInput, path string, max int64) (string, error) {
	path, e := safePath(path)
	if e != nil {
		return "", e
	}
	if max <= 0 || max > 4<<20 {
		max = 4 << 20
	}
	out, e := command(ctx, in, "show", ref(in)+":"+path)
	if e != nil {
		return "", e
	}
	if int64(len(out)) > max {
		return "", fmt.Errorf("file exceeds %d bytes", max)
	}
	if !utf8ish(out) {
		return "", errors.New("file is not UTF-8 text")
	}
	return string(out), nil
}
func Search(ctx context.Context, in ConnectionInput, q string, limit int) ([]map[string]any, error) {
	q = strings.TrimSpace(q)
	if q == "" || len(q) > 256 {
		return nil, errors.New("query is required and must be at most 256 characters")
	}
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	entries, err := Tree(ctx, in, "", 200)
	if err != nil {
		return nil, err
	}
	result := []map[string]any{}
	for _, entry := range entries {
		if len(result) >= limit {
			break
		}
		content, readErr := File(ctx, in, entry.Path, 1<<20)
		if readErr != nil {
			continue
		}
		for lineNo, line := range strings.Split(content, "\n") {
			if strings.Contains(line, q) {
				result = append(result, map[string]any{"path": entry.Path, "line": lineNo + 1, "text": line})
				if len(result) >= limit {
					break
				}
			}
		}
	}
	return result, nil
}
func Metadata(ctx context.Context, in ConnectionInput) (map[string]any, error) {
	rem, _ := command(ctx, in, "remote", "-v")
	log, e := command(ctx, in, "log", "-n", "20", "--date=iso-strict", "--pretty=format:%H%x09%ad%x09%an%x09%s")
	if e != nil {
		return nil, e
	}
	return map[string]any{"remotes": strings.TrimSpace(string(rem)), "commits": strings.Split(strings.TrimSpace(string(log)), "\n")}, nil
}
func safePath(p string) (string, error) {
	p = strings.TrimSpace(p)
	if p == "" {
		return "", nil
	}
	if filepath.IsAbs(p) || strings.ContainsRune(p, '\x00') {
		return "", errors.New("path must be relative")
	}
	c := filepath.ToSlash(filepath.Clean(p))
	if c == ".." || strings.HasPrefix(c, "../") {
		return "", errors.New("path escapes repository")
	}
	return c, nil
}
func utf8ish(b []byte) bool { return bytes.IndexByte(b, 0) < 0 }

var _ io.Reader
