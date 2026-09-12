package repository

import (
	"context"
	"errors"
	"fmt"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"io"
	"net/url"
	"opskeeper/backend/resource"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type Service struct {
	root      string
	maxBytes  int64
	resources interface {
		Get(context.Context, string) (resource.Resource, error)
	}
	mu      sync.Mutex
	storage StorageConfig
}
type StorageConfig struct {
	Backend, Root, Endpoint, Bucket, Prefix, AccessKey, SecretKey string
	UseSSL                                                        bool
}
type UploadResult struct {
	ResourceID   string   `json:"resource_id"`
	StorageKey   string   `json:"storage_key"`
	Branches     []string `json:"branches"`
	ArchivedRefs []string `json:"archived_refs,omitempty"`
}

func NewService(root string, maxBytes int64, resources interface {
	Get(context.Context, string) (resource.Resource, error)
}) *Service {
	if strings.TrimSpace(root) == "" {
		root = "/var/lib/opskeeper/repositories"
	}
	if maxBytes <= 0 {
		maxBytes = 512 << 20
	}
	return &Service{root: root, maxBytes: maxBytes, resources: resources, storage: StorageConfig{Backend: "local", Root: root}}
}
func NewServiceWithStorage(cfg StorageConfig, maxBytes int64, resources interface {
	Get(context.Context, string) (resource.Resource, error)
}) *Service {
	if cfg.Root == "" {
		cfg.Root = "/var/lib/opskeeper/repositories"
	}
	if cfg.Prefix == "" {
		cfg.Prefix = "repositories"
	}
	return &Service{root: cfg.Root, maxBytes: maxBytes, resources: resources, storage: cfg}
}
func (s *Service) UploadBundle(ctx context.Context, id string, r io.Reader, size int64) (UploadResult, error) {
	if s == nil || s.resources == nil {
		return UploadResult{}, errors.New("repository service unavailable")
	}
	if size <= 0 || size > s.maxBytes {
		return UploadResult{}, fmt.Errorf("bundle size must be between 1 and %d bytes", s.maxBytes)
	}
	item, e := s.resources.Get(ctx, id)
	if e != nil {
		return UploadResult{}, e
	}
	if item.Kind != "Repository" || !strings.EqualFold(item.Subtype, "Bundle") {
		return UploadResult{}, errors.New("resource must be a Bundle Repository")
	}
	if s.storage.Backend == "s3" {
		return s.uploadS3(ctx, id, r, size, item)
	}
	if err := os.MkdirAll(s.root, 0700); err != nil {
		return UploadResult{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	tmp, e := os.CreateTemp(s.root, "upload-*.bundle")
	if e != nil {
		return UploadResult{}, e
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if _, e = io.CopyN(tmp, r, size); e != nil {
		return UploadResult{}, e
	}
	if e = tmp.Close(); e != nil {
		return UploadResult{}, e
	}
	if e = run(ctx, "bundle", "verify", tmpName); e != nil {
		return UploadResult{}, e
	}
	repoDir := filepath.Join(s.root, id)
	if e = os.MkdirAll(repoDir, 0700); e != nil {
		return UploadResult{}, e
	}
	if _, e = os.Stat(filepath.Join(repoDir, "HEAD")); os.IsNotExist(e) {
		if e = run(ctx, "init", "--bare", repoDir); e != nil {
			return UploadResult{}, e
		}
	}
	archived, e := archiveHeads(ctx, repoDir)
	if e != nil {
		return UploadResult{}, e
	}
	if e = run(ctx, "--git-dir", repoDir, "fetch", tmpName, "+refs/heads/*:refs/heads/*"); e != nil {
		return UploadResult{}, e
	}
	branches, e := repoBranches(ctx, repoDir)
	if e != nil {
		return UploadResult{}, e
	}
	if updater, ok := s.resources.(interface {
		Update(context.Context, string, resource.UpdateInput) (resource.Resource, error)
	}); ok {
		cfg := item.Config
		if cfg == nil {
			cfg = map[string]any{}
		}
		cfg["path"] = repoDir
		cfg["storage_key"] = id
		_, _ = updater.Update(ctx, id, resource.UpdateInput{Config: &cfg})
	}
	return UploadResult{ResourceID: id, StorageKey: id, Branches: branches, ArchivedRefs: archived}, nil
}

func (s *Service) uploadS3(ctx context.Context, id string, r io.Reader, size int64, item resource.Resource) (UploadResult, error) {
	client, err := s.s3Client()
	if err != nil {
		return UploadResult{}, err
	}
	if err := client.MakeBucket(ctx, s.storage.Bucket, minio.MakeBucketOptions{}); err != nil {
		var exists minio.ErrorResponse
		if !errors.As(err, &exists) || exists.Code != "BucketAlreadyOwnedByYou" { /* bucket may be provisioned without permission */
		}
	}
	dir, err := os.MkdirTemp("", "opskeeper-s3-repo-")
	if err != nil {
		return UploadResult{}, err
	}
	defer os.RemoveAll(dir)
	repoDir := filepath.Join(dir, "repo.git")
	if err = os.MkdirAll(repoDir, 0700); err != nil {
		return UploadResult{}, err
	}
	key := s.objectKey(id)
	existing := filepath.Join(dir, "existing.bundle")
	if obj, e := client.GetObject(ctx, s.storage.Bucket, key, minio.GetObjectOptions{}); e == nil {
		f, ce := os.Create(existing)
		if ce == nil {
			_, ce = io.Copy(f, obj)
			_ = f.Close()
		}
		_ = obj.Close()
		if ce == nil {
			_ = run(ctx, "init", "--bare", repoDir)
			_ = run(ctx, "--git-dir", repoDir, "fetch", existing, "+refs/heads/*:refs/heads/*")
		}
	}
	upload := filepath.Join(dir, "upload.bundle")
	f, e := os.Create(upload)
	if e != nil {
		return UploadResult{}, e
	}
	if _, e = io.CopyN(f, r, size); e != nil {
		_ = f.Close()
		return UploadResult{}, e
	}
	_ = f.Close()
	if e = run(ctx, "bundle", "verify", upload); e != nil {
		return UploadResult{}, e
	}
	if _, e = os.Stat(filepath.Join(repoDir, "HEAD")); os.IsNotExist(e) {
		if e = run(ctx, "init", "--bare", repoDir); e != nil {
			return UploadResult{}, e
		}
	}
	archived, e := archiveHeads(ctx, repoDir)
	if e != nil {
		return UploadResult{}, e
	}
	if e = run(ctx, "--git-dir", repoDir, "fetch", upload, "+refs/heads/*:refs/heads/*"); e != nil {
		return UploadResult{}, e
	}
	full := filepath.Join(dir, "full.bundle")
	if e = run(ctx, "--git-dir", repoDir, "bundle", "create", full, "--all"); e != nil {
		return UploadResult{}, e
	}
	stat, e := os.Stat(full)
	if e != nil {
		return UploadResult{}, e
	}
	fh, e := os.Open(full)
	if e != nil {
		return UploadResult{}, e
	}
	_, e = client.PutObject(ctx, s.storage.Bucket, key, fh, stat.Size(), minio.PutObjectOptions{ContentType: "application/x-git-bundle"})
	_ = fh.Close()
	if e != nil {
		return UploadResult{}, e
	}
	branches, e := repoBranches(ctx, repoDir)
	if e != nil {
		return UploadResult{}, e
	}
	if updater, ok := s.resources.(interface {
		Update(context.Context, string, resource.UpdateInput) (resource.Resource, error)
	}); ok {
		cfg := item.Config
		if cfg == nil {
			cfg = map[string]any{}
		}
		cfg["storage_backend"] = "s3"
		cfg["storage_key"] = key
		cfg["s3_endpoint"] = s.storage.Endpoint
		cfg["s3_bucket"] = s.storage.Bucket
		cfg["s3_prefix"] = s.storage.Prefix
		_, _ = updater.Update(ctx, id, resource.UpdateInput{Config: &cfg})
	}
	return UploadResult{ResourceID: id, StorageKey: key, Branches: branches, ArchivedRefs: archived}, nil
}
func (s *Service) objectKey(id string) string {
	return strings.Trim(s.storage.Prefix, "/") + "/" + id + "/repository.bundle"
}
func (s *Service) s3Client() (*minio.Client, error) {
	if strings.TrimSpace(s.storage.Endpoint) == "" || strings.TrimSpace(s.storage.Bucket) == "" {
		return nil, errors.New("S3 endpoint and bucket are required")
	}
	endpoint := strings.TrimSpace(s.storage.Endpoint)
	if parsed, err := url.Parse(endpoint); err == nil && parsed.Host != "" {
		endpoint = parsed.Host
		if parsed.Scheme == "https" {
			s.storage.UseSSL = true
		}
	}
	return minio.New(endpoint, &minio.Options{Creds: credentials.NewStaticV4(s.storage.AccessKey, s.storage.SecretKey, ""), Secure: s.storage.UseSSL, BucketLookup: minio.BucketLookupPath})
}

func archiveHeads(ctx context.Context, dir string) ([]string, error) {
	out, err := exec.CommandContext(ctx, "git", "--git-dir", dir, "for-each-ref", "--format=%(refname:strip=2) %(objectname)", "refs/heads").Output()
	if err != nil {
		return nil, err
	}
	stamp := time.Now().UTC().Format("20060102T150405.000000000Z")
	archived := []string{}
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		fields := strings.Fields(line)
		if len(fields) != 2 {
			continue
		}
		ref := "refs/opskeeper/archive/" + stamp + "/" + fields[0]
		if err := run(ctx, "--git-dir", dir, "update-ref", ref, fields[1]); err != nil {
			return nil, err
		}
		archived = append(archived, ref)
	}
	return archived, nil
}
func run(ctx context.Context, args ...string) error {
	out, e := exec.CommandContext(ctx, "git", args...).CombinedOutput()
	if e != nil {
		return fmt.Errorf("git %v: %w: %s", args, e, strings.TrimSpace(string(out)))
	}
	return nil
}
func repoBranches(ctx context.Context, dir string) ([]string, error) {
	out, e := exec.CommandContext(ctx, "git", "--git-dir", dir, "for-each-ref", "--format=%(refname:short)", "refs/heads").Output()
	if e != nil {
		return nil, e
	}
	var b []string
	for _, x := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if strings.TrimSpace(x) != "" {
			b = append(b, strings.TrimSpace(x))
		}
	}
	return b, nil
}
