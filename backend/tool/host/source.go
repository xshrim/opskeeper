package host

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

type source interface {
	ReadFile(context.Context, string) ([]byte, error)
	ReadDir(context.Context, string) ([]string, error)
	ReadLink(context.Context, string) (string, error)
	StatFS(context.Context, string) (fsStat, error)
	Close() error
}

type fsStat struct{ Total, Free, Available uint64 }

type localSource struct{}

func (localSource) ReadFile(_ context.Context, path string) ([]byte, error) { return os.ReadFile(path) }
func (localSource) ReadDir(_ context.Context, path string) ([]string, error) {
	entries, err := os.ReadDir(path)
	if err != nil {
		return nil, err
	}
	paths := make([]string, 0, len(entries))
	for _, entry := range entries {
		paths = append(paths, filepath.Join(path, entry.Name()))
	}
	return paths, nil
}
func (localSource) ReadLink(_ context.Context, path string) (string, error) { return os.Readlink(path) }
func (localSource) StatFS(_ context.Context, path string) (fsStat, error) {
	var stat syscallStatfs
	if err := statfs(path, &stat); err != nil {
		return fsStat{}, err
	}
	return fsStat{Total: stat.blocks * uint64(stat.blockSize), Free: stat.bfree * uint64(stat.blockSize), Available: stat.bavail * uint64(stat.blockSize)}, nil
}
func (localSource) Close() error { return nil }

// The tiny wrapper keeps source.go independent of platform-specific syscall
// fields while retaining Linux statfs semantics in statfs_linux.go.
type syscallStatfs struct{ blocks, bfree, bavail, blockSize uint64 }

type sshSource struct{ client *ssh.Client }

func (s *sshSource) ReadFile(ctx context.Context, path string) ([]byte, error) {
	return s.run(ctx, "cat -- "+shellQuote(path), MaxReadBytes)
}
func (s *sshSource) ReadDir(ctx context.Context, path string) ([]string, error) {
	output, err := s.run(ctx, "find -L -- "+shellQuote(path)+" -mindepth 1 -maxdepth 1 -type d -printf '%f\\n'", 1<<20)
	if err != nil {
		return nil, err
	}
	items := make([]string, 0)
	for _, line := range strings.Split(string(output), "\n") {
		if line = strings.TrimSpace(line); line != "" {
			items = append(items, filepath.Join(path, line))
		}
	}
	return items, nil
}
func (s *sshSource) ReadLink(ctx context.Context, path string) (string, error) {
	output, err := s.run(ctx, "readlink -- "+shellQuote(path), 16<<10)
	return strings.TrimSpace(string(output)), err
}
func (s *sshSource) StatFS(ctx context.Context, path string) (fsStat, error) {
	output, err := s.run(ctx, "stat -f -c '%S %b %f %a' -- "+shellQuote(path), 1024)
	if err != nil {
		return fsStat{}, err
	}
	fields := strings.Fields(string(output))
	if len(fields) != 4 {
		return fsStat{}, errors.New("invalid remote filesystem statistics")
	}
	values := make([]uint64, 4)
	for i, field := range fields {
		values[i], err = strconv.ParseUint(field, 10, 64)
		if err != nil {
			return fsStat{}, err
		}
	}
	return fsStat{Total: values[0] * values[1], Free: values[0] * values[2], Available: values[0] * values[3]}, nil
}
func (s *sshSource) Close() error {
	if s == nil || s.client == nil {
		return nil
	}
	return s.client.Close()
}
func (s *sshSource) run(ctx context.Context, command string, limit int64) ([]byte, error) {
	if s == nil || s.client == nil {
		return nil, ErrSSHUnavailable
	}
	session, err := s.client.NewSession()
	if err != nil {
		return nil, err
	}
	defer session.Close()
	var output bytes.Buffer
	session.Stdout = &output
	if err := session.Start(command); err != nil {
		return nil, err
	}
	done := make(chan error, 1)
	go func() { done <- session.Wait() }()
	select {
	case <-ctx.Done():
		_ = session.Signal(ssh.SIGKILL)
		return nil, ctx.Err()
	case err := <-done:
		if err != nil {
			return nil, err
		}
	}
	if int64(output.Len()) > limit {
		return nil, fmt.Errorf("remote response exceeds %d bytes", limit)
	}
	return output.Bytes(), nil
}

func openSource(ctx context.Context, input ConnectionInput) (source, Target, error) {
	input, target, err := resolveInput(input)
	if err != nil {
		return nil, Target{}, err
	}
	if target.Mode == "local" {
		return localSource{}, target, nil
	}
	connectCtx, cancel := timeoutContext(ctx, input)
	defer cancel()
	clientConfig, cleanup, err := sshConfig(input)
	if err != nil {
		return nil, Target{}, err
	}
	address := net.JoinHostPort(input.Host, strconv.Itoa(input.Port))
	client, err := (&net.Dialer{}).DialContext(connectCtx, "tcp", address)
	if err != nil {
		cleanup()
		return nil, Target{}, fmt.Errorf("dial SSH host: %w", err)
	}
	conn, chans, requests, err := ssh.NewClientConn(client, address, clientConfig)
	if err != nil {
		_ = client.Close()
		cleanup()
		return nil, Target{}, fmt.Errorf("SSH handshake: %w", err)
	}
	sshClient := ssh.NewClient(conn, chans, requests)
	return &sshSource{client: sshClient}, target, nil
}

func sshConfig(input ConnectionInput) (*ssh.ClientConfig, func(), error) {
	var auth ssh.AuthMethod
	switch input.AuthMethod {
	case "password":
		auth = ssh.Password(input.Password)
	case "key":
		key, err := privateKeyBytes(input.PrivateKey)
		if err != nil {
			return nil, func() {}, err
		}
		var signer ssh.Signer
		if input.Passphrase != "" {
			signer, err = ssh.ParsePrivateKeyWithPassphrase(key, []byte(input.Passphrase))
		} else {
			signer, err = ssh.ParsePrivateKey(key)
		}
		if err != nil {
			return nil, func() {}, fmt.Errorf("parse SSH private key: %w", err)
		}
		auth = ssh.PublicKeys(signer)
	default:
		return nil, func() {}, fmt.Errorf("%w: unsupported auth method", ErrInvalidArgument)
	}
	hostKeyCallback, cleanup, err := knownHostsCallback(input.KnownHosts)
	if err != nil {
		return nil, func() {}, err
	}
	return &ssh.ClientConfig{User: input.Username, Auth: []ssh.AuthMethod{auth}, HostKeyCallback: hostKeyCallback, Timeout: time.Duration(input.TimeoutSeconds) * time.Second}, cleanup, nil
}

func privateKeyBytes(raw string) ([]byte, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, fmt.Errorf("%w: private_key is empty", ErrInvalidArgument)
	}
	if decoded, err := base64.StdEncoding.DecodeString(strings.Join(strings.Fields(raw), "")); err == nil && bytes.Contains(decoded, []byte("PRIVATE KEY")) {
		return decoded, nil
	}
	return []byte(raw), nil
}

func knownHostsCallback(raw string) (ssh.HostKeyCallback, func(), error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		if home, err := os.UserHomeDir(); err == nil {
			raw = filepath.Join(home, ".ssh", "known_hosts")
		}
	}
	if raw == "" {
		return nil, func() {}, fmt.Errorf("%w: known_hosts is required for SSH", ErrInvalidArgument)
	}
	path := raw
	cleanup := func() {}
	if strings.Contains(raw, "\n") || strings.HasPrefix(raw, "ssh-") || strings.HasPrefix(raw, "@cert-authority") {
		file, err := os.CreateTemp("", "opskeeper-host-known-hosts-")
		if err != nil {
			return nil, func() {}, err
		}
		path = file.Name()
		cleanup = func() { _ = os.Remove(path) }
		if err := file.Chmod(0o600); err != nil {
			file.Close()
			cleanup()
			return nil, func() {}, err
		}
		if _, err := file.WriteString(raw + "\n"); err != nil {
			file.Close()
			cleanup()
			return nil, func() {}, err
		}
		if err := file.Close(); err != nil {
			cleanup()
			return nil, func() {}, err
		}
	}
	callback, err := knownhosts.New(path)
	if err != nil {
		cleanup()
		return nil, func() {}, fmt.Errorf("load known_hosts: %w", err)
	}
	return callback, cleanup, nil
}

func lookupUser(uid uint64, passwd []byte) string {
	for _, line := range strings.Split(string(passwd), "\n") {
		fields := strings.SplitN(line, ":", 4)
		if len(fields) >= 3 && fields[2] == strconv.FormatUint(uid, 10) {
			return fields[0]
		}
	}
	if current, err := user.LookupId(strconv.FormatUint(uid, 10)); err == nil {
		return current.Username
	}
	return ""
}

func readLimited(ctx context.Context, src source, path string) ([]byte, error) {
	data, err := src.ReadFile(ctx, path)
	if err != nil {
		return nil, err
	}
	if len(data) > MaxReadBytes {
		return data[:MaxReadBytes], fmt.Errorf("response exceeds %d bytes", MaxReadBytes)
	}
	return data, nil
}

func parseUintField(value string) uint64 {
	parsed, _ := strconv.ParseUint(strings.TrimSpace(value), 10, 64)
	return parsed
}
func parseIntField(value string) int {
	parsed, _ := strconv.Atoi(strings.TrimSpace(value))
	return parsed
}
func firstLine(data []byte) string {
	return strings.TrimSpace(strings.SplitN(string(data), "\n", 2)[0])
}
func contextOrBackground(ctx context.Context) context.Context {
	if ctx == nil {
		return context.Background()
	}
	return ctx
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'"
}

// Keep io imported for old Go toolchains that do not optimize LimitReader
// away in builds where source implementations are replaced by tests.
var _ = io.EOF
