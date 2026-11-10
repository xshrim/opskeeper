package repository

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

type ConnectionInput struct {
	Path           string        `json:"path,omitempty"`
	URL            string        `json:"url,omitempty"`
	Branch         string        `json:"branch,omitempty"`
	Username       string        `json:"username,omitempty"`
	Password       string        `json:"password,omitempty"`
	TimeoutSeconds int           `json:"timeout_seconds,omitempty"`
	StorageBackend string        `json:"storage_backend,omitempty"`
	StorageKey     string        `json:"storage_key,omitempty"`
	S3Endpoint     string        `json:"s3_endpoint,omitempty"`
	S3Bucket       string        `json:"s3_bucket,omitempty"`
	S3Prefix       string        `json:"s3_prefix,omitempty"`
	S3AccessKey    string        `json:"s3_access_key,omitempty"`
	S3SecretKey    string        `json:"s3_secret_key,omitempty"`
	S3UseSSL       bool          `json:"s3_use_ssl,omitempty"`
	Postgres       *pgxpool.Pool `json:"-"`
	// Directory scopes application source queries to a relative subdirectory
	// of the repository. It is supplied by the server-side application binding.
	Directory string `json:"-"`
	// PinnedRevision is set by the execution-scoped repository snapshot. Model
	// supplied revisions cannot move a source-level diagnosis to another commit.
	PinnedRevision string `json:"-"`
}
type ToolInfo struct{ Name, Description string }

// Snapshot lazily materializes one repository for one Engine execution. The
// first repository tool pins the source; later tools reuse the same checkout.
type Snapshot struct {
	input    ConnectionInput
	once     sync.Once
	mu       sync.Mutex
	prepared ConnectionInput
	cleanup  func() error
	err      error
}

func NewSnapshot(input ConnectionInput) *Snapshot { return &Snapshot{input: input} }

func (s *Snapshot) resolve(ctx context.Context) (ConnectionInput, error) {
	s.once.Do(func() {
		prepared, cleanup, err := Prepare(ctx, s.input)
		s.mu.Lock()
		s.prepared, s.cleanup, s.err = prepared, cleanup, err
		s.mu.Unlock()
	})
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.prepared, s.err
}

func (s *Snapshot) Close() error {
	s.mu.Lock()
	cleanup := s.cleanup
	s.cleanup = nil
	s.mu.Unlock()
	if cleanup == nil {
		return nil
	}
	return cleanup()
}

func (s *Snapshot) Branches(ctx context.Context) ([]string, error) {
	in, err := s.resolve(ctx)
	if err != nil {
		return nil, err
	}
	return Branches(ctx, in)
}

func (s *Snapshot) Status(ctx context.Context) (map[string]any, error) {
	in, err := s.resolve(ctx)
	if err != nil {
		return nil, err
	}
	return Status(ctx, in)
}

func (s *Snapshot) Tree(ctx context.Context, path string, limit int, revision string) ([]TreeEntry, error) {
	in, err := s.resolve(ctx)
	if err != nil {
		return nil, err
	}
	return TreeAt(ctx, in, path, limit, revision)
}

func (s *Snapshot) TreePage(ctx context.Context, path string, offset, limit int, revision string) (TreePage, error) {
	in, err := s.resolve(ctx)
	if err != nil {
		return TreePage{}, err
	}
	return TreePageAt(ctx, in, path, offset, limit, revision)
}

func (s *Snapshot) File(ctx context.Context, path string, max int64, revision string) (string, error) {
	in, err := s.resolve(ctx)
	if err != nil {
		return "", err
	}
	return FileAt(ctx, in, path, max, revision)
}

func (s *Snapshot) FileRange(ctx context.Context, path string, offset, max int64, revision string) (FileRange, error) {
	in, err := s.resolve(ctx)
	if err != nil {
		return FileRange{}, err
	}
	return FileRangeAt(ctx, in, path, offset, max, revision)
}

func (s *Snapshot) Search(ctx context.Context, query string, limit int, revision string) ([]map[string]any, error) {
	in, err := s.resolve(ctx)
	if err != nil {
		return nil, err
	}
	return SearchAt(ctx, in, query, limit, revision)
}

func (s *Snapshot) SearchPage(ctx context.Context, query, path string, offset, limit int, revision string) (SearchPage, error) {
	in, err := s.resolve(ctx)
	if err != nil {
		return SearchPage{}, err
	}
	return SearchPageAt(ctx, in, query, path, offset, limit, revision)
}

func (s *Snapshot) Metadata(ctx context.Context) (map[string]any, error) {
	in, err := s.resolve(ctx)
	if err != nil {
		return nil, err
	}
	return Metadata(ctx, in)
}

var tools = []ToolInfo{
	{"repository_branches", "List repository branches."},
	{"repository_status", "Read the resolved repository revision and worktree status when applicable."},
	{"repository_tree", "Page through every file in a repository directory tree using offset and limit; continue until done is true."},
	{"repository_file", "Read a text file in byte ranges; continue from next_offset until done is true."},
	{"repository_search", "Page through literal text matches using offset and limit; continue until done is true and confirm complete is true. observed_matches is only a lower bound when complete is false."},
	{"repository_metadata", "Read repository remotes and recent commits at the resolved revision."},
}

func ListTools() []ToolInfo { return append([]ToolInfo(nil), tools...) }

// Prepare opens one immutable, execution-scoped repository snapshot. The
// returned input points at the materialized local repository, so every tool in
// the execution reads the same files even when the remote repository changes.
func Prepare(ctx context.Context, in ConnectionInput) (ConnectionInput, func() error, error) {
	sourceInput := configuredBundleStorage(in)
	root, cleanup, selectedRevision, err := openRepository(ctx, sourceInput, strings.TrimSpace(in.PinnedRevision))
	if err != nil {
		return ConnectionInput{}, func() error { return nil }, err
	}
	if strings.EqualFold(strings.TrimSpace(sourceInput.StorageBackend), "local") && strings.TrimSpace(sourceInput.StorageKey) != "" {
		dir, cloneErr := os.MkdirTemp("", "opskeeper-repository-snapshot-")
		if cloneErr != nil {
			cleanup()
			return ConnectionInput{}, func() error { return nil }, cloneErr
		}
		snapshotRoot := filepath.Join(dir, "repo.git")
		cmd := exec.CommandContext(ctx, "git", "clone", "--bare", root, snapshotRoot)
		cmd.Env = append(os.Environ(), gitCredentialEnvironment(sourceInput)...)
		if output, cloneErr := cmd.CombinedOutput(); cloneErr != nil {
			cleanup()
			_ = os.RemoveAll(dir)
			return ConnectionInput{}, func() error { return nil }, fmt.Errorf("git snapshot: %w: %s", cloneErr, strings.TrimSpace(string(output)))
		}
		previousCleanup := cleanup
		root = snapshotRoot
		cleanup = func() {
			previousCleanup()
			_ = os.RemoveAll(dir)
		}
	}
	prepared := in
	prepared.Path = root
	prepared.URL = ""
	prepared.StorageBackend = ""
	prepared.StorageKey = ""
	prepared.S3Endpoint = ""
	prepared.S3Bucket = ""
	prepared.S3Prefix = ""
	prepared.S3AccessKey = ""
	prepared.S3SecretKey = ""
	prepared.S3UseSSL = false
	if selectedRevision != "" {
		prepared.PinnedRevision = selectedRevision
	} else if revision := snapshotRevision(ctx, prepared, root); revision != "" {
		prepared.PinnedRevision = revision
	}
	return prepared, func() error {
		cleanup()
		return nil
	}, nil
}

func snapshotRevision(ctx context.Context, in ConnectionInput, root string) string {
	if head, err := runGit(ctx, in, root, "rev-parse", "--verify", "HEAD"); err == nil {
		return strings.TrimSpace(string(head))
	}
	refs, err := runGit(ctx, in, root, "for-each-ref", "--sort=-committerdate", "--format=%(refname)", "refs/heads")
	if err != nil {
		return ""
	}
	for _, ref := range strings.Fields(string(refs)) {
		if head, err := runGit(ctx, in, root, "rev-parse", "--verify", ref); err == nil {
			return strings.TrimSpace(string(head))
		}
	}
	return ""
}

func InputSchema(extra map[string]any) map[string]any {
	p := map[string]any{"path": map[string]any{"type": "string"}, "url": map[string]any{"type": "string"}, "branch": map[string]any{"type": "string"}, "username": map[string]any{"type": "string"}, "password": map[string]any{"type": "string"}, "timeout_seconds": map[string]any{"type": "integer", "minimum": 1, "maximum": 300}, "storage_backend": map[string]any{"type": "string", "enum": []string{"local", "postgres", "s3"}}, "storage_key": map[string]any{"type": "string"}, "s3_endpoint": map[string]any{"type": "string"}, "s3_bucket": map[string]any{"type": "string"}, "s3_prefix": map[string]any{"type": "string"}}
	for k, v := range extra {
		p[k] = v
	}
	return map[string]any{"type": "object", "properties": p, "additionalProperties": false}
}
func command(ctx context.Context, in ConnectionInput, args ...string) ([]byte, error) {
	ctx, cancel := repositoryContext(ctx, in)
	defer cancel()
	root, cleanup, _, err := openRepository(ctx, in, "")
	if err != nil {
		return nil, err
	}
	defer cleanup()
	return runGit(ctx, in, root, args...)
}

func openRepository(ctx context.Context, in ConnectionInput, revision string) (string, func(), string, error) {
	in = configuredBundleStorage(in)
	if len(revision) > 256 || strings.HasPrefix(revision, "-") || strings.ContainsAny(revision, "\x00\r\n:") || strings.Contains(revision, "..") || strings.Contains(revision, "@{") {
		return "", func() {}, "", errors.New("invalid repository revision")
	}
	root := strings.TrimSpace(in.Path)
	cleanup := func() {}
	backend := strings.ToLower(strings.TrimSpace(in.StorageBackend))
	if backend == "s3" {
		var err error
		root, cleanup, err = materializeS3(ctx, in)
		if err != nil {
			return "", func() {}, "", err
		}
	}
	if backend == "postgres" {
		var err error
		root, cleanup, err = materializePostgres(ctx, in)
		if err != nil {
			return "", func() {}, "", err
		}
	}
	if root == "" && strings.TrimSpace(in.URL) != "" {
		parsed, err := url.Parse(strings.TrimSpace(in.URL))
		if err != nil || ((parsed.Scheme == "http" || parsed.Scheme == "https") && parsed.User != nil) {
			return "", func() {}, "", errors.New("repository URL must not contain embedded credentials")
		}
		d, err := os.MkdirTemp("", "opskeeper-repository-")
		if err != nil {
			return "", func() {}, "", err
		}
		cleanup = func() { _ = os.RemoveAll(d) }
		branch := strings.TrimSpace(in.Branch)
		clone := []string{"clone", "--no-tags", "--depth=1"}
		if branch != "" {
			clone = append(clone, "--branch", branch)
		}
		clone = append(clone, in.URL, d)
		if _, err := runGit(ctx, in, d, clone...); err != nil {
			cleanup()
			return "", func() {}, "", fmt.Errorf("git clone: %w", err)
		}
		root = d
		if revision != "" && revision != branch {
			if _, err := runGit(ctx, in, root, "fetch", "--no-tags", "--depth=1", "origin", revision); err != nil {
				cleanup()
				return "", func() {}, "", fmt.Errorf("fetch repository revision: %w", err)
			}
			revision = "FETCH_HEAD"
		}
	}
	if root == "" {
		return "", func() {}, "", errors.New("repository path or url is required")
	}
	clean, err := filepath.Abs(filepath.Clean(root))
	if err != nil {
		cleanup()
		return "", func() {}, "", err
	}
	return clean, cleanup, revision, nil
}

// Bundle storage is selected when the process starts. Git resources have no
// storage key and continue to use their remote URL, so their connection input
// is left untouched.
func configuredBundleStorage(in ConnectionInput) ConnectionInput {
	if strings.TrimSpace(in.StorageKey) == "" {
		return in
	}
	backend := strings.TrimSpace(os.Getenv("OPSK_REPOSITORY_STORAGE_BACKEND"))
	if backend == "" {
		backend = "postgres"
	}
	in.StorageBackend = strings.ToLower(backend)
	if in.StorageBackend == "local" {
		if root := strings.TrimSpace(os.Getenv("OPSK_REPOSITORY_LOCAL_ROOT")); root != "" {
			in.Path = filepath.Join(root, in.StorageKey)
		}
	}
	if value := strings.TrimSpace(os.Getenv("OPSK_REPOSITORY_S3_ENDPOINT")); value != "" {
		in.S3Endpoint = value
	}
	if value := strings.TrimSpace(os.Getenv("OPSK_REPOSITORY_S3_BUCKET")); value != "" {
		in.S3Bucket = value
	}
	if value := strings.TrimSpace(os.Getenv("OPSK_REPOSITORY_S3_ACCESS_KEY")); value != "" {
		in.S3AccessKey = value
	}
	if value := strings.TrimSpace(os.Getenv("OPSK_REPOSITORY_S3_SECRET_KEY")); value != "" {
		in.S3SecretKey = value
	}
	if prefix := strings.TrimSpace(os.Getenv("OPSK_REPOSITORY_S3_PREFIX")); prefix != "" {
		in.S3Prefix = prefix
	}
	if value := strings.TrimSpace(os.Getenv("OPSK_REPOSITORY_S3_USE_SSL")); value != "" {
		in.S3UseSSL = strings.EqualFold(value, "true") || value == "1"
	}
	return in
}

func repositoryContext(ctx context.Context, in ConnectionInput) (context.Context, context.CancelFunc) {
	timeout := time.Duration(in.TimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	return context.WithTimeout(ctx, timeout)
}

func runGit(ctx context.Context, in ConnectionInput, root string, args ...string) ([]byte, error) {
	if len(args) == 0 {
		return nil, errors.New("git command is required")
	}
	commandArgs := append([]string{"-C", root}, args...)
	cmd := exec.CommandContext(ctx, "git", commandArgs...)
	cmd.Env = append(os.Environ(), gitCredentialEnvironment(in)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("git %s: %w: %s", args[0], err, strings.TrimSpace(string(out)))
	}
	return out, nil
}

func gitCredentialEnvironment(in ConnectionInput) []string {
	if in.Username == "" && in.Password == "" {
		return nil
	}
	token := base64.StdEncoding.EncodeToString([]byte(in.Username + ":" + in.Password))
	return []string{"GIT_CONFIG_COUNT=1", "GIT_CONFIG_KEY_0=http.extraHeader", "GIT_CONFIG_VALUE_0=Authorization: Basic " + token}
}

func materializePostgres(ctx context.Context, in ConnectionInput) (string, func(), error) {
	pool := in.Postgres
	closePool := func() {}
	if pool == nil {
		url := strings.TrimSpace(os.Getenv("OPSK_DATABASE_URL"))
		if url == "" || strings.TrimSpace(in.StorageKey) == "" {
			return "", func() {}, errors.New("PostgreSQL repository database URL and storage key are required")
		}
		var err error
		pool, err = pgxpool.New(ctx, url)
		if err != nil {
			return "", func() {}, err
		}
		closePool = pool.Close
	}
	if strings.TrimSpace(in.StorageKey) == "" {
		closePool()
		return "", func() {}, errors.New("PostgreSQL repository storage key is required")
	}
	var bundle []byte
	err := pool.QueryRow(ctx, `SELECT bundle FROM repository_bundles WHERE resource_id=$1::uuid`, in.StorageKey).Scan(&bundle)
	if err != nil {
		closePool()
		return "", func() {}, err
	}
	dir, err := os.MkdirTemp("", "opskeeper-repository-postgres-")
	if err != nil {
		closePool()
		return "", func() {}, err
	}
	cleanup := func() { closePool(); _ = os.RemoveAll(dir) }
	bundlePath := filepath.Join(dir, "repository.bundle")
	if err := os.WriteFile(bundlePath, bundle, 0600); err != nil {
		cleanup()
		return "", func() {}, err
	}
	repo := filepath.Join(dir, "repo.git")
	if err := exec.CommandContext(ctx, "git", "init", "--bare", repo).Run(); err != nil {
		cleanup()
		return "", func() {}, err
	}
	if err := exec.CommandContext(ctx, "git", "--git-dir", repo, "fetch", bundlePath, "+refs/heads/*:refs/heads/*").Run(); err != nil {
		cleanup()
		return "", func() {}, err
	}
	return repo, cleanup, nil
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
	in = configuredBundleStorage(in)
	ctx, cancel := repositoryContext(ctx, in)
	defer cancel()
	root, cleanup, selected, err := openRepository(ctx, in, "")
	if err != nil {
		return nil, err
	}
	defer cleanup()
	revision := snapshotRevision(ctx, in, root)
	if revision == "" {
		return nil, errors.New("repository has no readable commit")
	}
	if selected != "" {
		revision = selected
	}
	branch := ""
	if out, branchErr := runGit(ctx, in, root, "symbolic-ref", "--short", "HEAD"); branchErr == nil {
		if head, headErr := runGit(ctx, in, root, "rev-parse", "--verify", "HEAD"); headErr == nil && strings.TrimSpace(string(head)) == revision {
			branch = strings.TrimSpace(string(out))
		}
	}
	if branch == "" {
		if out, branchErr := runGit(ctx, in, root, "for-each-ref", "--points-at", revision, "--format=%(refname:short)", "refs/heads"); branchErr == nil {
			branch = strings.TrimSpace(strings.Split(string(out), "\n")[0])
		}
	}
	bare, err := runGit(ctx, in, root, "rev-parse", "--is-bare-repository")
	if err != nil {
		return nil, err
	}
	result := map[string]any{
		"head": revision, "branch": branch, "has_worktree": strings.TrimSpace(string(bare)) != "true",
		"resolved_revision": revision,
	}
	if strings.TrimSpace(string(bare)) == "true" {
		result["clean"] = nil
		result["changed_files"] = nil
		result["worktree_status"] = "not_applicable"
		return result, nil
	}
	st, e := runGit(ctx, in, root, "status", "--porcelain=v1")
	if e != nil {
		return nil, e
	}
	changed := 0
	if text := strings.TrimSpace(string(st)); text != "" {
		changed = len(strings.Split(text, "\n"))
	}
	result["clean"] = changed == 0
	result["changed_files"] = changed
	result["worktree_status"] = "available"
	return result, nil
}

type TreeEntry struct {
	Path string `json:"path"`
	Type string `json:"type"`
	Size int64  `json:"size,omitempty"`
}

type TreePage struct {
	Items      []TreeEntry `json:"items"`
	Offset     int         `json:"offset"`
	NextOffset int         `json:"next_offset"`
	Total      int         `json:"total"`
	Done       bool        `json:"done"`
}

type SearchPage struct {
	Items           []map[string]any `json:"items"`
	Offset          int              `json:"offset"`
	NextOffset      int              `json:"next_offset"`
	ObservedMatches int              `json:"observed_matches"`
	Done            bool             `json:"done"`
	Complete        bool             `json:"complete"`
	Truncated       bool             `json:"truncated"`
}

type FileRange struct {
	Path       string `json:"path"`
	Content    string `json:"content"`
	Offset     int64  `json:"offset"`
	NextOffset int64  `json:"next_offset"`
	TotalBytes int64  `json:"total_bytes"`
	Done       bool   `json:"done"`
}

func TreePageAt(ctx context.Context, in ConnectionInput, path string, offset, limit int, revision string) (TreePage, error) {
	if offset < 0 {
		offset = 0
	}
	if limit <= 0 || limit > 500 {
		limit = 200
	}
	path, err := scopedPath(in.Directory, path)
	if err != nil {
		return TreePage{}, err
	}
	ctx, cancel := repositoryContext(ctx, in)
	defer cancel()
	if in.PinnedRevision != "" {
		revision = ""
	}
	root, cleanup, selectedRevision, err := openRepository(ctx, in, revision)
	if err != nil {
		return TreePage{}, err
	}
	defer cleanup()
	selectedRevision = readRevision(ctx, in, root, selectedRevision)
	args := []string{"-C", root, "ls-tree", "-r", "-l", "-z", selectedRevision}
	if path != "" {
		args = append(args, "--", path)
	}
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Env = append(os.Environ(), gitCredentialEnvironment(in)...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return TreePage{}, err
	}
	if err := cmd.Start(); err != nil {
		return TreePage{}, err
	}
	page := TreePage{Offset: offset, Items: make([]TreeEntry, 0, limit)}
	reader := bufio.NewReader(stdout)
	for {
		record, readErr := reader.ReadString('\x00')
		if len(record) > 0 {
			header, filePath, found := strings.Cut(strings.TrimSuffix(record, "\x00"), "\t")
			fields := strings.Fields(header)
			if found && len(fields) >= 4 {
				size, _ := strconv.ParseInt(fields[3], 10, 64)
				if page.Total >= offset && len(page.Items) < limit {
					page.Items = append(page.Items, TreeEntry{Path: displayScopedPath(in.Directory, filePath), Type: "file", Size: size})
				}
				page.Total++
			}
		}
		if readErr != nil {
			if !errors.Is(readErr, io.EOF) {
				_ = cmd.Process.Kill()
				_ = cmd.Wait()
				return TreePage{}, readErr
			}
			break
		}
	}
	if err := cmd.Wait(); err != nil {
		return TreePage{}, fmt.Errorf("git ls-tree: %w", err)
	}
	if page.Offset > page.Total {
		page.Offset = page.Total
	}
	page.NextOffset = page.Offset + len(page.Items)
	page.Done = page.NextOffset >= page.Total
	return page, nil
}

func FileRangeAt(ctx context.Context, in ConnectionInput, path string, offset int64, max int64, revision string) (FileRange, error) {
	if offset < 0 {
		return FileRange{}, errors.New("file offset must not be negative")
	}
	if max <= 0 || max > 1<<20 {
		max = 1 << 20
	}
	cleanPath, err := scopedPath(in.Directory, path)
	if err != nil {
		return FileRange{}, err
	}
	ctx, cancel := repositoryContext(ctx, in)
	defer cancel()
	if in.PinnedRevision != "" {
		revision = ""
	}
	root, cleanup, selectedRevision, err := openRepository(ctx, in, revision)
	if err != nil {
		return FileRange{}, err
	}
	defer cleanup()
	selectedRevision = readRevision(ctx, in, root, selectedRevision)
	blob, err := runGit(ctx, in, root, "rev-parse", "--verify", selectedRevision+":"+cleanPath)
	if err != nil {
		return FileRange{}, err
	}
	blob = []byte(strings.TrimSpace(string(blob)))
	sizeOutput, err := runGit(ctx, in, root, "cat-file", "-s", string(blob))
	if err != nil {
		return FileRange{}, err
	}
	totalBytes, err := strconv.ParseInt(strings.TrimSpace(string(sizeOutput)), 10, 64)
	if err != nil {
		return FileRange{}, fmt.Errorf("read repository file size: %w", err)
	}
	if offset >= totalBytes {
		return FileRange{Path: displayScopedPath(in.Directory, cleanPath), Offset: totalBytes, NextOffset: totalBytes, TotalBytes: totalBytes, Done: true}, nil
	}
	cmd := exec.CommandContext(ctx, "git", "-C", root, "cat-file", "blob", string(blob))
	cmd.Env = append(os.Environ(), gitCredentialEnvironment(in)...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return FileRange{}, err
	}
	if err := cmd.Start(); err != nil {
		return FileRange{}, err
	}
	if _, err := io.CopyN(io.Discard, stdout, offset); err != nil {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		return FileRange{}, err
	}
	chunk := make([]byte, int(max)+4)
	n, _ := io.ReadFull(stdout, chunk)
	chunk = chunk[:n]
	start := 0
	for start < len(chunk) && chunk[start]&0xC0 == 0x80 {
		start++
	}
	end := start + int(max)
	if end > len(chunk) {
		end = len(chunk)
	}
	for end < len(chunk) && end > start && chunk[end]&0xC0 == 0x80 {
		end--
	}
	content := chunk[start:end]
	if !utf8ish(content) || !utf8.Valid(content) {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		return FileRange{}, errors.New("file is not UTF-8 text")
	}
	nextOffset := offset + int64(end)
	done := nextOffset >= totalBytes
	if !done {
		_ = cmd.Process.Kill()
	}
	waitErr := cmd.Wait()
	if done && waitErr != nil {
		return FileRange{}, fmt.Errorf("read repository file: %w", waitErr)
	}
	return FileRange{Path: displayScopedPath(in.Directory, cleanPath), Content: string(content), Offset: offset + int64(start), NextOffset: nextOffset, TotalBytes: totalBytes, Done: done}, nil
}

func SearchPageAt(ctx context.Context, in ConnectionInput, query, path string, offset, limit int, revision string) (SearchPage, error) {
	if offset < 0 {
		offset = 0
	}
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	query = strings.TrimSpace(query)
	if query == "" || len(query) > 256 {
		return SearchPage{}, errors.New("query is required and must be at most 256 characters")
	}
	path, err := scopedPath(in.Directory, path)
	if err != nil {
		return SearchPage{}, err
	}
	ctx, cancel := repositoryContext(ctx, in)
	defer cancel()
	if in.PinnedRevision != "" {
		revision = ""
	}
	root, cleanup, selectedRevision, err := openRepository(ctx, in, revision)
	if err != nil {
		return SearchPage{}, err
	}
	defer cleanup()
	selectedRevision = readRevision(ctx, in, root, selectedRevision)
	output := &limitedBuffer{max: 64 << 20}
	args := []string{"-C", root, "grep", "-z", "-n", "-F", "-I", "-e", query, selectedRevision, "--"}
	if path != "" {
		args = append(args, path)
	}
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Env = append(os.Environ(), gitCredentialEnvironment(in)...)
	cmd.Stdout = output
	cmd.Stderr = io.Discard
	err = cmd.Run()
	if err != nil {
		var exit *exec.ExitError
		if !errors.As(err, &exit) || exit.ExitCode() != 1 {
			return SearchPage{}, fmt.Errorf("git grep: %w", err)
		}
	}
	fields := bytes.Split(output.Bytes(), []byte{0})
	items := make([]map[string]any, 0, limit)
	total := 0
	matchPath := ""
	for i := 0; i+2 < len(fields); i += 2 {
		if matchPath == "" {
			matchPath = string(fields[i])
		}
		matchPath = strings.TrimPrefix(matchPath, selectedRevision+":")
		line, parseErr := strconv.Atoi(string(fields[i+1]))
		if matchPath == "" || parseErr != nil {
			continue
		}
		textAndNextPath := strings.SplitN(string(fields[i+2]), "\n", 2)
		text := strings.TrimSuffix(textAndNextPath[0], "\n")
		if total >= offset && len(items) < limit {
			match := map[string]any{"path": displayScopedPath(in.Directory, matchPath), "line": line, "text": text}
			if len(text) > 4096 {
				text = text[:4096]
				for !utf8.ValidString(text) {
					text = text[:len(text)-1]
				}
				match["text"] = text
				match["text_truncated"] = true
			}
			items = append(items, match)
		}
		total++
		matchPath = ""
		if len(textAndNextPath) > 1 {
			matchPath = textAndNextPath[1]
		}
	}
	next := offset + len(items)
	return SearchPage{Items: items, Offset: offset, NextOffset: next, ObservedMatches: total, Done: next >= total, Complete: !output.Truncated, Truncated: output.Truncated}, nil
}

func readRevision(ctx context.Context, in ConnectionInput, root, selected string) string {
	if in.PinnedRevision != "" {
		return in.PinnedRevision
	}
	if selected != "" {
		return selected
	}
	if branch := strings.TrimSpace(in.Branch); branch != "" {
		return branch
	}
	if revision := snapshotRevision(ctx, in, root); revision != "" {
		return revision
	}
	return "HEAD"
}

func Tree(ctx context.Context, in ConnectionInput, path string, limit int) ([]TreeEntry, error) {
	return TreeAt(ctx, in, path, limit, "")
}

func TreeAt(ctx context.Context, in ConnectionInput, path string, limit int, revision string) ([]TreeEntry, error) {
	if limit <= 0 || limit > 1000 {
		limit = 200
	}
	return treeAt(ctx, in, path, limit, revision)
}

// TreeAll returns every file in the selected repository path for UI previews.
func TreeAll(ctx context.Context, in ConnectionInput, path string) ([]TreeEntry, error) {
	return treeAt(ctx, in, path, 0, "")
}

func treeAt(ctx context.Context, in ConnectionInput, path string, limit int, revision string) ([]TreeEntry, error) {
	path, err := scopedPath(in.Directory, path)
	if err != nil {
		return nil, err
	}
	ctx, cancel := repositoryContext(ctx, in)
	defer cancel()
	if in.PinnedRevision != "" {
		revision = ""
	}
	root, cleanup, selectedRevision, err := openRepository(ctx, in, revision)
	if err != nil {
		return nil, err
	}
	defer cleanup()
	selectedRevision = readRevision(ctx, in, root, selectedRevision)
	args := []string{"ls-tree", "-r", "-l", selectedRevision}
	if path != "" {
		args = append(args, "--", path)
	}
	out, e := runGit(ctx, in, root, args...)
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
		p := displayScopedPath(in.Directory, strings.Join(f[4:], " "))
		r = append(r, TreeEntry{Path: p, Type: "file", Size: n})
		if limit > 0 && len(r) >= limit {
			break
		}
	}
	return r, nil
}
func File(ctx context.Context, in ConnectionInput, path string, max int64) (string, error) {
	return FileAt(ctx, in, path, max, "")
}

func FileAt(ctx context.Context, in ConnectionInput, path string, max int64, revision string) (string, error) {
	path, e := scopedPath(in.Directory, path)
	if e != nil {
		return "", e
	}
	if max <= 0 || max > 4<<20 {
		max = 4 << 20
	}
	ctx, cancel := repositoryContext(ctx, in)
	defer cancel()
	if in.PinnedRevision != "" {
		revision = ""
	}
	root, cleanup, selectedRevision, e := openRepository(ctx, in, revision)
	if e != nil {
		return "", e
	}
	defer cleanup()
	selectedRevision = readRevision(ctx, in, root, selectedRevision)
	out, e := runGit(ctx, in, root, "show", selectedRevision+":"+path)
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
	return SearchAt(ctx, in, q, limit, "")
}

func SearchAt(ctx context.Context, in ConnectionInput, q string, limit int, revision string) ([]map[string]any, error) {
	page, err := SearchPageAt(ctx, in, q, "", 0, limit, revision)
	if err != nil {
		return nil, err
	}
	return page.Items, nil
}

type limitedBuffer struct {
	bytes.Buffer
	max       int
	Truncated bool
}

func (b *limitedBuffer) Write(value []byte) (int, error) {
	remaining := b.max - b.Len()
	if remaining > 0 {
		if len(value) > remaining {
			_, _ = b.Buffer.Write(value[:remaining])
			b.Truncated = true
		} else {
			_, _ = b.Buffer.Write(value)
		}
	} else if len(value) > 0 {
		b.Truncated = true
	}
	return len(value), nil
}
func Metadata(ctx context.Context, in ConnectionInput) (map[string]any, error) {
	in = configuredBundleStorage(in)
	ctx, cancel := repositoryContext(ctx, in)
	defer cancel()
	root, cleanup, selected, err := openRepository(ctx, in, "")
	if err != nil {
		return nil, err
	}
	defer cleanup()
	revision := snapshotRevision(ctx, in, root)
	if revision == "" {
		return nil, errors.New("repository has no readable commit")
	}
	if selected != "" {
		revision = selected
	}
	rem, remoteErr := runGit(ctx, in, root, "remote", "-v")
	if remoteErr != nil {
		return nil, remoteErr
	}
	log, e := runGit(ctx, in, root, "log", "-n", "20", "--date=iso-strict", "--pretty=format:%H%x09%ad%x09%an%x09%s", revision)
	if e != nil {
		return nil, e
	}
	return map[string]any{"remotes": strings.TrimSpace(string(rem)), "has_remote": strings.TrimSpace(string(rem)) != "", "resolved_revision": revision, "commits": strings.Split(strings.TrimSpace(string(log)), "\n")}, nil
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

func scopedPath(directory, path string) (string, error) {
	directory, err := safePath(directory)
	if err != nil {
		return "", err
	}
	path, err = safePath(path)
	if err != nil {
		return "", err
	}
	if directory == "" {
		return path, nil
	}
	if path == "" {
		return directory, nil
	}
	return filepath.ToSlash(filepath.Join(directory, path)), nil
}

func displayScopedPath(directory, path string) string {
	directory = strings.Trim(strings.TrimSpace(filepath.ToSlash(directory)), "/")
	path = filepath.ToSlash(path)
	if directory == "" {
		return path
	}
	return strings.TrimPrefix(path, directory+"/")
}
func utf8ish(b []byte) bool { return bytes.IndexByte(b, 0) < 0 }
