package mcp

import (
	"context"
	"encoding/json"

	"github.com/jackc/pgx/v5/pgxpool"
)

type SnapshotStore interface {
	Save(context.Context, Snapshot) (Snapshot, error)
	List(context.Context, string, int) ([]Snapshot, error)
}

type store struct{ pool *pgxpool.Pool }

func NewStore(pool *pgxpool.Pool) SnapshotStore { return &store{pool: pool} }

func (s *store) Save(ctx context.Context, item Snapshot) (Snapshot, error) {
	tools, err := json.Marshal(item.Tools)
	if err != nil {
		return Snapshot{}, err
	}
	err = s.pool.QueryRow(ctx, `INSERT INTO mcp_server_snapshots(server_resource_id,scope_id,protocol_version,server_name,server_version,tools,content_hash,status,error_message) VALUES($1::uuid,$2::uuid,$3,$4,$5,$6::jsonb,$7,$8,$9) RETURNING id::text,created_at`, item.ServerResourceID, item.ScopeID, item.ProtocolVersion, item.ServerName, item.ServerVersion, string(tools), item.Hash, item.Status, item.ErrorMessage).Scan(&item.ID, &item.CreatedAt)
	item.Untrusted = true
	return item, err
}

func (s *store) List(ctx context.Context, resourceID string, limit int) ([]Snapshot, error) {
	if limit < 1 || limit > 100 {
		limit = 20
	}
	rows, err := s.pool.Query(ctx, `SELECT id::text,server_resource_id::text,scope_id::text,protocol_version,server_name,server_version,tools,content_hash,status,error_message,created_at FROM mcp_server_snapshots WHERE server_resource_id=$1::uuid ORDER BY created_at DESC LIMIT $2`, resourceID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []Snapshot{}
	for rows.Next() {
		var item Snapshot
		var tools []byte
		if err := rows.Scan(&item.ID, &item.ServerResourceID, &item.ScopeID, &item.ProtocolVersion, &item.ServerName, &item.ServerVersion, &tools, &item.Hash, &item.Status, &item.ErrorMessage, &item.CreatedAt); err != nil {
			return nil, err
		}
		_ = json.Unmarshal(tools, &item.Tools)
		item.Untrusted = true
		items = append(items, item)
	}
	return items, rows.Err()
}
