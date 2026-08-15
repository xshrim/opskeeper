package migrations

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed sql/*.sql
var migrationFiles embed.FS

const (
	// Keep this value stable so migrators built from different releases contend on the same lock.
	migrationAdvisoryLockID int64 = 0x4f50534b4d494752
	migrationUnlockTimeout        = 5 * time.Second
)

type migration struct {
	version int64
	name    string
	sql     string
	downSQL string
}

func Apply(ctx context.Context, pool *pgxpool.Pool) error {
	return withMigrationLock(ctx, pool, apply)
}

func apply(ctx context.Context, conn *pgxpool.Conn) error {
	migrations, err := load()
	if err != nil {
		return err
	}

	if _, err := conn.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version bigint PRIMARY KEY,
			name text NOT NULL,
			applied_at timestamptz NOT NULL DEFAULT now()
		)`); err != nil {
		return fmt.Errorf("create schema migrations table: %w", err)
	}

	for _, item := range migrations {
		var applied bool
		if err := conn.QueryRow(ctx,
			"SELECT EXISTS (SELECT 1 FROM schema_migrations WHERE version = $1)",
			item.version,
		).Scan(&applied); err != nil {
			return fmt.Errorf("check migration %d: %w", item.version, err)
		}
		if applied {
			continue
		}

		tx, err := conn.Begin(ctx)
		if err != nil {
			return fmt.Errorf("begin migration %d: %w", item.version, err)
		}
		if _, err := tx.Exec(ctx, item.sql); err != nil {
			_ = tx.Rollback(ctx)
			return fmt.Errorf("apply migration %d (%s): %w", item.version, item.name, err)
		}
		if _, err := tx.Exec(ctx,
			"INSERT INTO schema_migrations (version, name) VALUES ($1, $2)",
			item.version,
			item.name,
		); err != nil {
			_ = tx.Rollback(ctx)
			return fmt.Errorf("record migration %d: %w", item.version, err)
		}
		if err := tx.Commit(ctx); err != nil {
			return fmt.Errorf("commit migration %d: %w", item.version, err)
		}
	}

	return nil
}

func RollbackLast(ctx context.Context, pool *pgxpool.Pool) error {
	return withMigrationLock(ctx, pool, rollbackLast)
}

func rollbackLast(ctx context.Context, conn *pgxpool.Conn) error {
	migrations, err := load()
	if err != nil {
		return err
	}
	if _, err := conn.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version bigint PRIMARY KEY,
			name text NOT NULL,
			applied_at timestamptz NOT NULL DEFAULT now()
		)`); err != nil {
		return fmt.Errorf("create schema migrations table: %w", err)
	}

	var version int64
	var name string
	if err := conn.QueryRow(ctx,
		"SELECT version, name FROM schema_migrations ORDER BY version DESC LIMIT 1",
	).Scan(&version, &name); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		return fmt.Errorf("find latest migration: %w", err)
	}

	var selected *migration
	for index := range migrations {
		if migrations[index].version == version {
			selected = &migrations[index]
			break
		}
	}
	if selected == nil || selected.name != name {
		return fmt.Errorf("migration %d (%s) is not embedded in this binary", version, name)
	}

	tx, err := conn.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin rollback %d: %w", version, err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, selected.downSQL); err != nil {
		return fmt.Errorf("rollback migration %d (%s): %w", version, name, err)
	}
	if _, err := tx.Exec(ctx, "DELETE FROM schema_migrations WHERE version = $1", version); err != nil {
		return fmt.Errorf("remove migration record %d: %w", version, err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit rollback %d: %w", version, err)
	}
	return nil
}

func withMigrationLock(
	ctx context.Context,
	pool *pgxpool.Pool,
	run func(context.Context, *pgxpool.Conn) error,
) (runErr error) {
	conn, err := pool.Acquire(ctx)
	if err != nil {
		return fmt.Errorf("acquire migration connection: %w", err)
	}
	if _, err := conn.Exec(ctx, "SELECT pg_advisory_lock($1)", migrationAdvisoryLockID); err != nil {
		closeErr := closeMigrationConnection(conn)
		return errors.Join(fmt.Errorf("acquire migration advisory lock: %w", err), closeErr)
	}

	defer func() {
		runErr = errors.Join(runErr, releaseMigrationLock(conn))
	}()
	return run(ctx, conn)
}

func releaseMigrationLock(conn *pgxpool.Conn) error {
	ctx, cancel := context.WithTimeout(context.Background(), migrationUnlockTimeout)
	defer cancel()

	var unlocked bool
	unlockErr := conn.QueryRow(ctx,
		"SELECT pg_advisory_unlock($1)",
		migrationAdvisoryLockID,
	).Scan(&unlocked)
	if unlockErr == nil && unlocked {
		conn.Release()
		return nil
	}

	closeErr := closeMigrationConnection(conn)
	if unlockErr != nil {
		return errors.Join(fmt.Errorf("release migration advisory lock: %w", unlockErr), closeErr)
	}
	return errors.Join(errors.New("release migration advisory lock: lock was not held"), closeErr)
}

func closeMigrationConnection(conn *pgxpool.Conn) error {
	rawConn := conn.Hijack()
	ctx, cancel := context.WithTimeout(context.Background(), migrationUnlockTimeout)
	defer cancel()
	if err := rawConn.Close(ctx); err != nil {
		return fmt.Errorf("close migration connection: %w", err)
	}
	return nil
}

func load() ([]migration, error) {
	entries, err := fs.ReadDir(migrationFiles, "sql")
	if err != nil {
		return nil, fmt.Errorf("read migrations: %w", err)
	}

	items := make([]migration, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".sql" || strings.HasSuffix(entry.Name(), ".down.sql") {
			continue
		}
		parts := strings.SplitN(strings.TrimSuffix(entry.Name(), ".sql"), "_", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid migration filename %q", entry.Name())
		}
		version, err := strconv.ParseInt(parts[0], 10, 64)
		if err != nil || version <= 0 {
			return nil, fmt.Errorf("invalid migration version in %q", entry.Name())
		}
		content, err := migrationFiles.ReadFile("sql/" + entry.Name())
		if err != nil {
			return nil, fmt.Errorf("read migration %q: %w", entry.Name(), err)
		}
		if len(strings.TrimSpace(string(content))) == 0 {
			return nil, errors.New("migration " + entry.Name() + " is empty")
		}
		downName := strings.TrimSuffix(entry.Name(), ".sql") + ".down.sql"
		downContent, err := migrationFiles.ReadFile("sql/" + downName)
		if err != nil {
			return nil, fmt.Errorf("read rollback migration %q: %w", downName, err)
		}
		if len(strings.TrimSpace(string(downContent))) == 0 {
			return nil, errors.New("rollback migration " + downName + " is empty")
		}
		items = append(items, migration{
			version: version,
			name:    parts[1],
			sql:     string(content),
			downSQL: string(downContent),
		})
	}

	sort.Slice(items, func(i, j int) bool { return items[i].version < items[j].version })
	for index := 1; index < len(items); index++ {
		if items[index-1].version == items[index].version {
			return nil, fmt.Errorf("duplicate migration version %d", items[index].version)
		}
	}
	return items, nil
}
