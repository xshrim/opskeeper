package minio

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	client "github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

type ConnectionInput struct {
	Endpoint       string `json:"endpoint,omitempty"`
	AccessKey      string `json:"access_key,omitempty"`
	SecretKey      string `json:"secret_key,omitempty"`
	SessionToken   string `json:"session_token,omitempty"`
	Region         string `json:"region,omitempty"`
	Secure         bool   `json:"secure,omitempty"`
	TimeoutSeconds int    `json:"timeout_seconds,omitempty"`
}
type ToolInfo struct{ Name, Description string }
type BucketInfo struct {
	Name         string    `json:"name"`
	CreationDate time.Time `json:"creation_date"`
}
type ObjectInfo struct {
	Key          string    `json:"key"`
	Size         int64     `json:"size"`
	LastModified time.Time `json:"last_modified"`
	ETag         string    `json:"etag,omitempty"`
	ContentType  string    `json:"content_type,omitempty"`
	VersionID    string    `json:"version_id,omitempty"`
}

var tools = []ToolInfo{
	{"minio_health", "Check MinIO connectivity and return a bounded bucket summary."},
	{"minio_buckets", "List MinIO buckets visible to the configured credentials."},
	{"minio_bucket_objects", "List a bounded set of objects in a MinIO bucket."},
	{"minio_object_stat", "Read metadata for one MinIO object."},
	{"minio_bucket_versioning", "Read versioning configuration for one bucket."},
	{"minio_bucket_lifecycle", "Read lifecycle configuration for one bucket."},
}

func ListTools() []ToolInfo { return append([]ToolInfo(nil), tools...) }
func InputSchema(extra map[string]any) map[string]any {
	p := map[string]any{"endpoint": map[string]any{"type": "string", "minLength": 1}, "access_key": map[string]any{"type": "string"}, "secret_key": map[string]any{"type": "string"}, "session_token": map[string]any{"type": "string"}, "region": map[string]any{"type": "string"}, "secure": map[string]any{"type": "boolean"}, "timeout_seconds": map[string]any{"type": "integer", "minimum": 1, "maximum": 300}}
	var required []string
	for k, v := range extra {
		if k == "__required" {
			if values, ok := v.([]string); ok {
				required = append(required, values...)
			}
			continue
		}
		p[k] = v
	}
	schema := map[string]any{"type": "object", "properties": p, "additionalProperties": false}
	if len(required) > 0 {
		schema["required"] = required
	}
	return schema
}
func open(ctx context.Context, in ConnectionInput) (*client.Client, error) {
	endpoint := strings.TrimSpace(in.Endpoint)
	if endpoint == "" {
		return nil, errors.New("endpoint is required")
	}
	if strings.HasPrefix(strings.ToLower(endpoint), "https://") {
		in.Secure = true
		endpoint = endpoint[len("https://"):]
	} else if strings.HasPrefix(strings.ToLower(endpoint), "http://") {
		endpoint = endpoint[len("http://"):]
	}
	if strings.Contains(endpoint, "/") {
		return nil, errors.New("endpoint must not include a path")
	}
	if in.TimeoutSeconds <= 0 {
		in.TimeoutSeconds = 10
	}
	c, err := client.New(endpoint, &client.Options{Creds: credentials.NewStaticV4(in.AccessKey, in.SecretKey, in.SessionToken), Secure: in.Secure, Region: in.Region})
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(ctx, time.Duration(in.TimeoutSeconds)*time.Second)
	defer cancel()
	if _, err = c.ListBuckets(ctx); err != nil {
		return nil, err
	}
	return c, nil
}
func withClient[T any](ctx context.Context, in ConnectionInput, fn func(context.Context, *client.Client) (T, error)) (T, error) {
	var zero T
	c, err := open(ctx, in)
	if err != nil {
		return zero, err
	}
	timeout := in.TimeoutSeconds
	if timeout <= 0 {
		timeout = 10
	}
	operationCtx, cancel := context.WithTimeout(ctx, time.Duration(timeout)*time.Second)
	defer cancel()
	return fn(operationCtx, c)
}
func buckets(ctx context.Context, in ConnectionInput) ([]BucketInfo, error) {
	return withClient(ctx, in, func(ctx context.Context, c *client.Client) ([]BucketInfo, error) {
		items, err := c.ListBuckets(ctx)
		if err != nil {
			return nil, err
		}
		out := make([]BucketInfo, 0, len(items))
		for _, b := range items {
			out = append(out, BucketInfo{Name: b.Name, CreationDate: b.CreationDate})
		}
		return out, nil
	})
}
func Health(ctx context.Context, in ConnectionInput) (map[string]any, error) {
	started := time.Now()
	bs, err := buckets(ctx, in)
	if err != nil {
		return nil, err
	}
	return map[string]any{"status": "ok", "bucket_count": len(bs), "latency_ms": time.Since(started).Milliseconds()}, nil
}
func Buckets(ctx context.Context, in ConnectionInput) ([]BucketInfo, error) { return buckets(ctx, in) }
func BucketObjects(ctx context.Context, in ConnectionInput, bucket, prefix string, limit int) ([]ObjectInfo, error) {
	if strings.TrimSpace(bucket) == "" {
		return nil, errors.New("bucket is required")
	}
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	return withClient(ctx, in, func(ctx context.Context, c *client.Client) ([]ObjectInfo, error) {
		ch := c.ListObjects(ctx, bucket, client.ListObjectsOptions{Prefix: prefix, Recursive: true})
		out := make([]ObjectInfo, 0, limit)
		for item := range ch {
			if item.Err != nil {
				return nil, item.Err
			}
			out = append(out, ObjectInfo{Key: item.Key, Size: item.Size, LastModified: item.LastModified, ETag: item.ETag, ContentType: item.ContentType, VersionID: item.VersionID})
			if len(out) >= limit {
				break
			}
		}
		return out, nil
	})
}
func ObjectStat(ctx context.Context, in ConnectionInput, bucket, object string) (ObjectInfo, error) {
	if strings.TrimSpace(bucket) == "" || strings.TrimSpace(object) == "" {
		return ObjectInfo{}, errors.New("bucket and object are required")
	}
	return withClient(ctx, in, func(ctx context.Context, c *client.Client) (ObjectInfo, error) {
		v, e := c.StatObject(ctx, bucket, object, client.StatObjectOptions{})
		if e != nil {
			return ObjectInfo{}, e
		}
		return ObjectInfo{Key: object, Size: v.Size, LastModified: v.LastModified, ETag: v.ETag, ContentType: v.ContentType, VersionID: v.VersionID}, nil
	})
}
func BucketVersioning(ctx context.Context, in ConnectionInput, bucket string) (any, error) {
	if strings.TrimSpace(bucket) == "" {
		return nil, errors.New("bucket is required")
	}
	return withClient(ctx, in, func(ctx context.Context, c *client.Client) (any, error) { return c.GetBucketVersioning(ctx, bucket) })
}
func BucketLifecycle(ctx context.Context, in ConnectionInput, bucket string) (any, error) {
	if strings.TrimSpace(bucket) == "" {
		return nil, errors.New("bucket is required")
	}
	return withClient(ctx, in, func(ctx context.Context, c *client.Client) (any, error) { return c.GetBucketLifecycle(ctx, bucket) })
}
func Error(op string, err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%s: %w", op, err)
}
