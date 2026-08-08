package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"

	"github.com/oai404iao/sub_manager/internal/auth"
	"github.com/oai404iao/sub_manager/internal/model"
)

type Store struct {
	db *sql.DB
}

func Open(path, adminUser, adminPassword string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil && filepath.Dir(path) != "." {
		return nil, err
	}
	db, err := sql.Open("sqlite", path+"?_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)")
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	store := &Store{db: db}
	if err := store.migrate(); err != nil {
		_ = db.Close()
		return nil, err
	}
	if err := store.ensureAdmin(adminUser, adminPassword); err != nil {
		_ = db.Close()
		return nil, err
	}
	return store, nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) migrate() error {
	_, err := s.db.Exec(`
CREATE TABLE IF NOT EXISTS users (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	username TEXT NOT NULL UNIQUE,
	password_hash TEXT NOT NULL,
	created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE TABLE IF NOT EXISTS sessions (
	token_hash TEXT PRIMARY KEY,
	user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
	expires_at DATETIME NOT NULL,
	created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_sessions_expires ON sessions(expires_at);
CREATE TABLE IF NOT EXISTS groups (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	name TEXT NOT NULL UNIQUE,
	description TEXT NOT NULL DEFAULT '',
	created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE TABLE IF NOT EXISTS subscriptions (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	name TEXT NOT NULL,
	url TEXT NOT NULL,
	group_id INTEGER NOT NULL REFERENCES groups(id) ON DELETE RESTRICT,
	last_status TEXT NOT NULL DEFAULT 'pending',
	last_error TEXT NOT NULL DEFAULT '',
	last_synced_at DATETIME,
	created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE TABLE IF NOT EXISTS nodes (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	name TEXT NOT NULL,
	protocol TEXT NOT NULL,
	server TEXT NOT NULL,
	port INTEGER NOT NULL,
	config_json TEXT NOT NULL,
	subscription_id INTEGER REFERENCES subscriptions(id) ON DELETE SET NULL,
	created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE TABLE IF NOT EXISTS node_groups (
	node_id INTEGER NOT NULL REFERENCES nodes(id) ON DELETE CASCADE,
	group_id INTEGER NOT NULL REFERENCES groups(id) ON DELETE CASCADE,
	PRIMARY KEY(node_id, group_id)
);
`)
	return err
}

func (s *Store) ensureAdmin(username, password string) error {
	var count int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM users`).Scan(&count); err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	hash, err := auth.HashPassword(password)
	if err != nil {
		return err
	}
	_, err = s.db.Exec(`INSERT INTO users(username, password_hash) VALUES (?, ?)`, username, hash)
	return err
}

func (s *Store) Authenticate(username, password string) (model.User, bool, error) {
	var user model.User
	var hash string
	err := s.db.QueryRow(`SELECT id, username, password_hash FROM users WHERE username = ?`, username).
		Scan(&user.ID, &user.Username, &hash)
	if errors.Is(err, sql.ErrNoRows) {
		return model.User{}, false, nil
	}
	if err != nil {
		return model.User{}, false, err
	}
	return user, auth.CheckPassword(hash, password), nil
}

func (s *Store) CreateSession(userID int64, tokenHash string, expires time.Time) error {
	_, err := s.db.Exec(`INSERT INTO sessions(token_hash, user_id, expires_at) VALUES (?, ?, ?)`,
		tokenHash, userID, expires.UTC())
	return err
}

func (s *Store) SessionUser(tokenHash string) (model.User, bool, error) {
	var user model.User
	err := s.db.QueryRow(`
SELECT u.id, u.username
FROM sessions s JOIN users u ON u.id = s.user_id
WHERE s.token_hash = ? AND s.expires_at > ?`, tokenHash, time.Now().UTC()).
		Scan(&user.ID, &user.Username)
	if errors.Is(err, sql.ErrNoRows) {
		return model.User{}, false, nil
	}
	return user, err == nil, err
}

func (s *Store) DeleteSession(tokenHash string) error {
	_, err := s.db.Exec(`DELETE FROM sessions WHERE token_hash = ?`, tokenHash)
	return err
}

func (s *Store) State(ctx context.Context, user model.User) (model.State, error) {
	groups, err := s.Groups(ctx)
	if err != nil {
		return model.State{}, err
	}
	nodes, err := s.Nodes(ctx, 0)
	if err != nil {
		return model.State{}, err
	}
	subscriptions, err := s.Subscriptions(ctx)
	if err != nil {
		return model.State{}, err
	}
	return model.State{User: user, Groups: groups, Nodes: nodes, Subscriptions: subscriptions}, nil
}

func (s *Store) Groups(ctx context.Context) ([]model.Group, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT g.id, g.name, g.description, g.created_at, COUNT(ng.node_id)
FROM groups g LEFT JOIN node_groups ng ON ng.group_id = g.id
GROUP BY g.id ORDER BY g.name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []model.Group{}
	for rows.Next() {
		var group model.Group
		if err := rows.Scan(&group.ID, &group.Name, &group.Description, &group.CreatedAt, &group.NodeCount); err != nil {
			return nil, err
		}
		result = append(result, group)
	}
	return result, rows.Err()
}

func (s *Store) CreateGroup(ctx context.Context, name, description string) (model.Group, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return model.Group{}, errors.New("group name is required")
	}
	result, err := s.db.ExecContext(ctx, `INSERT INTO groups(name, description) VALUES (?, ?)`, name, description)
	if err != nil {
		return model.Group{}, err
	}
	id, _ := result.LastInsertId()
	return s.Group(ctx, id)
}

func (s *Store) Group(ctx context.Context, id int64) (model.Group, error) {
	var group model.Group
	err := s.db.QueryRowContext(ctx, `
SELECT g.id, g.name, g.description, g.created_at, COUNT(ng.node_id)
FROM groups g LEFT JOIN node_groups ng ON ng.group_id = g.id
WHERE g.id = ? GROUP BY g.id`, id).
		Scan(&group.ID, &group.Name, &group.Description, &group.CreatedAt, &group.NodeCount)
	return group, err
}

func (s *Store) DeleteGroup(ctx context.Context, id int64) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM groups WHERE id = ?`, id)
	return err
}

func (s *Store) Nodes(ctx context.Context, groupID int64) ([]model.Node, error) {
	query := `
SELECT DISTINCT n.id, n.name, n.protocol, n.server, n.port, n.config_json,
	n.subscription_id, n.created_at, n.updated_at
FROM nodes n`
	args := []any{}
	if groupID > 0 {
		query += ` JOIN node_groups filter_ng ON filter_ng.node_id = n.id WHERE filter_ng.group_id = ?`
		args = append(args, groupID)
	}
	query += ` ORDER BY n.updated_at DESC, n.id DESC`
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	result := []model.Node{}
	for rows.Next() {
		node, err := scanNode(rows)
		if err != nil {
			_ = rows.Close()
			return nil, err
		}
		result = append(result, node)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, err
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	for index := range result {
		result[index].GroupIDs, err = s.nodeGroups(ctx, result[index].ID)
		if err != nil {
			return nil, err
		}
	}
	return result, nil
}

func (s *Store) Node(ctx context.Context, id int64) (model.Node, error) {
	row := s.db.QueryRowContext(ctx, `
SELECT id, name, protocol, server, port, config_json, subscription_id, created_at, updated_at
FROM nodes WHERE id = ?`, id)
	node, err := scanNode(row)
	if err != nil {
		return model.Node{}, err
	}
	node.GroupIDs, err = s.nodeGroups(ctx, id)
	return node, err
}

func (s *Store) SaveNode(ctx context.Context, node model.Node) (model.Node, error) {
	data, err := json.Marshal(node)
	if err != nil {
		return model.Node{}, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return model.Node{}, err
	}
	defer tx.Rollback()
	if node.ID == 0 {
		result, err := tx.ExecContext(ctx, `
INSERT INTO nodes(name, protocol, server, port, config_json, subscription_id)
VALUES (?, ?, ?, ?, ?, ?)`, node.Name, node.Protocol, node.Server, node.Port, string(data), node.SubscriptionID)
		if err != nil {
			return model.Node{}, err
		}
		node.ID, _ = result.LastInsertId()
	} else {
		result, err := tx.ExecContext(ctx, `
UPDATE nodes SET name=?, protocol=?, server=?, port=?, config_json=?,
	subscription_id=?, updated_at=CURRENT_TIMESTAMP WHERE id=?`,
			node.Name, node.Protocol, node.Server, node.Port, string(data), node.SubscriptionID, node.ID)
		if err != nil {
			return model.Node{}, err
		}
		affected, _ := result.RowsAffected()
		if affected == 0 {
			return model.Node{}, sql.ErrNoRows
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM node_groups WHERE node_id = ?`, node.ID); err != nil {
			return model.Node{}, err
		}
	}
	for _, groupID := range node.GroupIDs {
		if _, err := tx.ExecContext(ctx, `INSERT OR IGNORE INTO node_groups(node_id, group_id) VALUES (?, ?)`,
			node.ID, groupID); err != nil {
			return model.Node{}, err
		}
	}
	if err := tx.Commit(); err != nil {
		return model.Node{}, err
	}
	return s.Node(ctx, node.ID)
}

func (s *Store) SaveNodes(ctx context.Context, nodes []model.Node) (int, error) {
	count := 0
	for _, node := range nodes {
		if _, err := s.SaveNode(ctx, node); err != nil {
			return count, err
		}
		count++
	}
	return count, nil
}

func (s *Store) DeleteNode(ctx context.Context, id int64) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM nodes WHERE id = ?`, id)
	return err
}

func (s *Store) nodeGroups(ctx context.Context, id int64) ([]int64, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT group_id FROM node_groups WHERE node_id = ? ORDER BY group_id`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []int64{}
	for rows.Next() {
		var groupID int64
		if err := rows.Scan(&groupID); err != nil {
			return nil, err
		}
		result = append(result, groupID)
	}
	return result, rows.Err()
}

func (s *Store) Subscriptions(ctx context.Context) ([]model.Subscription, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT id, name, url, group_id, last_status, last_error, last_synced_at, created_at
FROM subscriptions ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []model.Subscription{}
	for rows.Next() {
		var item model.Subscription
		var synced sql.NullTime
		if err := rows.Scan(&item.ID, &item.Name, &item.URL, &item.GroupID, &item.LastStatus,
			&item.LastError, &synced, &item.CreatedAt); err != nil {
			return nil, err
		}
		if synced.Valid {
			item.LastSyncedAt = &synced.Time
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

func (s *Store) CreateSubscription(ctx context.Context, name, rawURL string, groupID int64) (model.Subscription, error) {
	result, err := s.db.ExecContext(ctx, `
INSERT INTO subscriptions(name, url, group_id) VALUES (?, ?, ?)`, name, rawURL, groupID)
	if err != nil {
		return model.Subscription{}, err
	}
	id, _ := result.LastInsertId()
	return s.Subscription(ctx, id)
}

func (s *Store) UpdateSubscription(ctx context.Context, id int64, name, rawURL string, groupID int64) (model.Subscription, error) {
	result, err := s.db.ExecContext(ctx, `
UPDATE subscriptions
SET name = ?, url = ?, group_id = ?, last_status = 'pending', last_error = ''
WHERE id = ?`, name, rawURL, groupID, id)
	if err != nil {
		return model.Subscription{}, err
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		return model.Subscription{}, sql.ErrNoRows
	}
	return s.Subscription(ctx, id)
}

func (s *Store) Subscription(ctx context.Context, id int64) (model.Subscription, error) {
	var item model.Subscription
	var synced sql.NullTime
	err := s.db.QueryRowContext(ctx, `
SELECT id, name, url, group_id, last_status, last_error, last_synced_at, created_at
FROM subscriptions WHERE id = ?`, id).
		Scan(&item.ID, &item.Name, &item.URL, &item.GroupID, &item.LastStatus,
			&item.LastError, &synced, &item.CreatedAt)
	if synced.Valid {
		item.LastSyncedAt = &synced.Time
	}
	return item, err
}

func (s *Store) UpdateSubscriptionStatus(ctx context.Context, id int64, status, message string) error {
	_, err := s.db.ExecContext(ctx, `
UPDATE subscriptions SET last_status=?, last_error=?, last_synced_at=CURRENT_TIMESTAMP WHERE id=?`,
		status, message, id)
	return err
}

func (s *Store) ReplaceSubscriptionNodes(
	ctx context.Context,
	subscriptionID int64,
	groupID int64,
	nodes []model.Node,
) (int, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `DELETE FROM nodes WHERE subscription_id = ?`, subscriptionID); err != nil {
		return 0, err
	}
	for index := range nodes {
		node := nodes[index]
		node.ID = 0
		node.SubscriptionID = &subscriptionID
		node.GroupIDs = []int64{groupID}
		data, err := json.Marshal(node)
		if err != nil {
			return 0, err
		}
		result, err := tx.ExecContext(ctx, `
INSERT INTO nodes(name, protocol, server, port, config_json, subscription_id)
VALUES (?, ?, ?, ?, ?, ?)`,
			node.Name, node.Protocol, node.Server, node.Port, string(data), subscriptionID)
		if err != nil {
			return 0, err
		}
		nodeID, err := result.LastInsertId()
		if err != nil {
			return 0, err
		}
		if _, err := tx.ExecContext(ctx, `
INSERT INTO node_groups(node_id, group_id) VALUES (?, ?)`, nodeID, groupID); err != nil {
			return 0, err
		}
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return len(nodes), nil
}

func (s *Store) DeleteSubscription(ctx context.Context, id int64) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM subscriptions WHERE id = ?`, id)
	return err
}

type scanner interface {
	Scan(dest ...any) error
}

func scanNode(row scanner) (model.Node, error) {
	var node model.Node
	var data string
	var subscriptionID sql.NullInt64
	if err := row.Scan(&node.ID, &node.Name, &node.Protocol, &node.Server, &node.Port, &data,
		&subscriptionID, &node.CreatedAt, &node.UpdatedAt); err != nil {
		return model.Node{}, err
	}
	id, name, protocolName, server, port := node.ID, node.Name, node.Protocol, node.Server, node.Port
	createdAt, updatedAt := node.CreatedAt, node.UpdatedAt
	if err := json.Unmarshal([]byte(data), &node); err != nil {
		return model.Node{}, fmt.Errorf("decode node %d: %w", id, err)
	}
	node.ID, node.Name, node.Protocol, node.Server, node.Port = id, name, protocolName, server, port
	node.CreatedAt, node.UpdatedAt = createdAt, updatedAt
	if subscriptionID.Valid {
		node.SubscriptionID = &subscriptionID.Int64
	}
	return node, nil
}
