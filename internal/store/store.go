package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	_ "modernc.org/sqlite"

	"github.com/oai404iao/sub_manager/internal/auth"
	"github.com/oai404iao/sub_manager/internal/model"
)

type Store struct {
	db *sql.DB
}

var (
	ErrGroupNameRequired    = errors.New("group name is required")
	ErrGroupChildNotFound   = errors.New("child group not found")
	ErrGroupCycle           = errors.New("group hierarchy cycle")
	ErrGroupNotFound        = errors.New("group not found")
	ErrNodeNotFound         = errors.New("node not found")
	ErrManagedNodeGroups    = errors.New("subscription-managed node groups cannot be changed")
	ErrInvalidNodeGroupMode = errors.New("invalid node group update mode")
)

const (
	NodeGroupModeAdd     = "add"
	NodeGroupModeRemove  = "remove"
	NodeGroupModeReplace = "replace"
)

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
CREATE TABLE IF NOT EXISTS group_children (
	parent_group_id INTEGER NOT NULL REFERENCES groups(id) ON DELETE CASCADE,
	child_group_id INTEGER NOT NULL REFERENCES groups(id) ON DELETE CASCADE,
	PRIMARY KEY(parent_group_id, child_group_id),
	CHECK(parent_group_id <> child_group_id)
);
CREATE INDEX IF NOT EXISTS idx_group_children_child ON group_children(child_group_id);
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
CREATE INDEX IF NOT EXISTS idx_node_groups_group ON node_groups(group_id, node_id);
CREATE TABLE IF NOT EXISTS shares (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	kind TEXT NOT NULL CHECK(kind IN ('node', 'group')),
	target_id INTEGER NOT NULL,
	target_name TEXT NOT NULL,
	url TEXT NOT NULL,
	expires_at DATETIME,
	created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_shares_created ON shares(created_at DESC, id DESC);
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
	shares, err := s.Shares(ctx)
	if err != nil {
		return model.State{}, err
	}
	return model.State{
		User:          user,
		Groups:        groups,
		Nodes:         nodes,
		Subscriptions: subscriptions,
		Shares:        shares,
	}, nil
}

func (s *Store) Groups(ctx context.Context) ([]model.Group, error) {
	rows, err := s.db.QueryContext(ctx, `
WITH RECURSIVE group_tree(root_id, group_id) AS (
	SELECT id, id FROM groups
	UNION
	SELECT tree.root_id, children.child_group_id
	FROM group_tree tree
	JOIN group_children children ON children.parent_group_id = tree.group_id
)
SELECT g.id, g.name, g.description, g.created_at, COUNT(DISTINCT ng.node_id)
FROM groups g
LEFT JOIN group_tree tree ON tree.root_id = g.id
LEFT JOIN node_groups ng ON ng.group_id = tree.group_id
GROUP BY g.id
ORDER BY g.name COLLATE NOCASE, g.id`)
	if err != nil {
		return nil, err
	}
	result := []model.Group{}
	for rows.Next() {
		var group model.Group
		if err := rows.Scan(
			&group.ID,
			&group.Name,
			&group.Description,
			&group.CreatedAt,
			&group.NodeCount,
		); err != nil {
			_ = rows.Close()
			return nil, err
		}
		result = append(result, group)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, err
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	children, err := s.groupChildrenByParent(ctx)
	if err != nil {
		return nil, err
	}
	for index := range result {
		result[index].ChildGroupIDs = append([]int64{}, children[result[index].ID]...)
	}
	return result, nil
}

func (s *Store) CreateGroup(
	ctx context.Context,
	name string,
	description string,
	childGroupIDs []int64,
) (model.Group, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return model.Group{}, ErrGroupNameRequired
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return model.Group{}, err
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(
		ctx,
		`INSERT INTO groups(name, description) VALUES (?, ?)`,
		name,
		description,
	)
	if err != nil {
		return model.Group{}, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return model.Group{}, err
	}
	if err := setGroupChildren(ctx, tx, id, childGroupIDs); err != nil {
		return model.Group{}, err
	}
	if err := tx.Commit(); err != nil {
		return model.Group{}, err
	}
	return s.Group(ctx, id)
}

func (s *Store) UpdateGroup(
	ctx context.Context,
	id int64,
	name string,
	description string,
	childGroupIDs []int64,
) (model.Group, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return model.Group{}, ErrGroupNameRequired
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return model.Group{}, err
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(
		ctx,
		`UPDATE groups SET name = ?, description = ? WHERE id = ?`,
		name,
		description,
		id,
	)
	if err != nil {
		return model.Group{}, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return model.Group{}, err
	}
	if affected == 0 {
		return model.Group{}, sql.ErrNoRows
	}
	if err := setGroupChildren(ctx, tx, id, childGroupIDs); err != nil {
		return model.Group{}, err
	}
	if err := tx.Commit(); err != nil {
		return model.Group{}, err
	}
	return s.Group(ctx, id)
}

func (s *Store) Group(ctx context.Context, id int64) (model.Group, error) {
	var group model.Group
	err := s.db.QueryRowContext(ctx, `
WITH RECURSIVE selected_groups(id) AS (
	SELECT ?
	UNION
	SELECT children.child_group_id
	FROM group_children children
	JOIN selected_groups selected ON selected.id = children.parent_group_id
)
SELECT g.id, g.name, g.description, g.created_at, (
	SELECT COUNT(DISTINCT ng.node_id)
	FROM node_groups ng
	JOIN selected_groups selected ON selected.id = ng.group_id
)
FROM groups g
WHERE g.id = ?`, id, id).
		Scan(&group.ID, &group.Name, &group.Description, &group.CreatedAt, &group.NodeCount)
	if err != nil {
		return model.Group{}, err
	}
	group.ChildGroupIDs, err = s.groupChildren(ctx, id)
	return group, err
}

func (s *Store) DeleteGroup(ctx context.Context, id int64) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM groups WHERE id = ?`, id)
	return err
}

func (s *Store) Nodes(ctx context.Context, groupID int64) ([]model.Node, error) {
	var query string
	args := []any{}
	if groupID > 0 {
		query = `
WITH RECURSIVE selected_groups(id) AS (
	SELECT ?
	UNION
	SELECT children.child_group_id
	FROM group_children children
	JOIN selected_groups selected ON selected.id = children.parent_group_id
)
SELECT DISTINCT n.id, n.name, n.protocol, n.server, n.port, n.config_json,
	n.subscription_id, n.created_at, n.updated_at
FROM nodes n
JOIN node_groups filter_ng ON filter_ng.node_id = n.id
JOIN selected_groups selected ON selected.id = filter_ng.group_id`
		args = append(args, groupID)
	} else {
		query = `
SELECT DISTINCT n.id, n.name, n.protocol, n.server, n.port, n.config_json,
	n.subscription_id, n.created_at, n.updated_at
FROM nodes n`
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

func (s *Store) NodesByIDs(ctx context.Context, ids []int64) ([]model.Node, error) {
	if len(ids) == 0 {
		return []model.Node{}, nil
	}
	rows, err := s.db.QueryContext(ctx, fmt.Sprintf(`
SELECT id, name, protocol, server, port, config_json, subscription_id, created_at, updated_at
FROM nodes
WHERE id IN (%s)`, placeholders(len(ids))), int64Args(ids)...)
	if err != nil {
		return nil, err
	}
	loaded := make([]model.Node, 0, len(ids))
	for rows.Next() {
		node, err := scanNode(rows)
		if err != nil {
			_ = rows.Close()
			return nil, err
		}
		loaded = append(loaded, node)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, err
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	nodesByID := make(map[int64]model.Node, len(loaded))
	for index := range loaded {
		loaded[index].GroupIDs, err = s.nodeGroups(ctx, loaded[index].ID)
		if err != nil {
			return nil, err
		}
		nodesByID[loaded[index].ID] = loaded[index]
	}
	result := make([]model.Node, 0, len(ids))
	for _, id := range ids {
		node, ok := nodesByID[id]
		if !ok {
			return nil, ErrNodeNotFound
		}
		result = append(result, node)
	}
	return result, nil
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

func (s *Store) DeleteNodes(ctx context.Context, ids []int64) (int, error) {
	if len(ids) == 0 {
		return 0, nil
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	query := fmt.Sprintf(`SELECT COUNT(*) FROM nodes WHERE id IN (%s)`, placeholders(len(ids)))
	var count int
	if err := tx.QueryRowContext(ctx, query, int64Args(ids)...).Scan(&count); err != nil {
		return 0, err
	}
	if count != len(ids) {
		return 0, ErrNodeNotFound
	}
	result, err := tx.ExecContext(
		ctx,
		fmt.Sprintf(`DELETE FROM nodes WHERE id IN (%s)`, placeholders(len(ids))),
		int64Args(ids)...,
	)
	if err != nil {
		return 0, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return 0, err
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return int(affected), nil
}

func (s *Store) UpdateNodeGroups(
	ctx context.Context,
	nodeIDs []int64,
	groupIDs []int64,
	mode string,
) (int, error) {
	switch mode {
	case NodeGroupModeAdd, NodeGroupModeRemove, NodeGroupModeReplace:
	default:
		return 0, ErrInvalidNodeGroupMode
	}
	if len(nodeIDs) == 0 {
		return 0, nil
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	var nodeCount, managedCount int
	if err := tx.QueryRowContext(
		ctx,
		fmt.Sprintf(`
SELECT COUNT(*),
	COALESCE(SUM(CASE WHEN subscription_id IS NOT NULL THEN 1 ELSE 0 END), 0)
FROM nodes
WHERE id IN (%s)`, placeholders(len(nodeIDs))),
		int64Args(nodeIDs)...,
	).Scan(&nodeCount, &managedCount); err != nil {
		return 0, err
	}
	if nodeCount != len(nodeIDs) {
		return 0, ErrNodeNotFound
	}
	if managedCount > 0 {
		return 0, ErrManagedNodeGroups
	}
	if len(groupIDs) > 0 {
		var groupCount int
		if err := tx.QueryRowContext(
			ctx,
			fmt.Sprintf(`SELECT COUNT(*) FROM groups WHERE id IN (%s)`, placeholders(len(groupIDs))),
			int64Args(groupIDs)...,
		).Scan(&groupCount); err != nil {
			return 0, err
		}
		if groupCount != len(groupIDs) {
			return 0, ErrGroupNotFound
		}
	}

	switch mode {
	case NodeGroupModeAdd:
		for _, nodeID := range nodeIDs {
			for _, groupID := range groupIDs {
				if _, err := tx.ExecContext(
					ctx,
					`INSERT OR IGNORE INTO node_groups(node_id, group_id) VALUES (?, ?)`,
					nodeID,
					groupID,
				); err != nil {
					return 0, err
				}
			}
		}
	case NodeGroupModeRemove:
		if len(groupIDs) > 0 {
			args := append(int64Args(nodeIDs), int64Args(groupIDs)...)
			if _, err := tx.ExecContext(
				ctx,
				fmt.Sprintf(`
DELETE FROM node_groups
WHERE node_id IN (%s) AND group_id IN (%s)`,
					placeholders(len(nodeIDs)),
					placeholders(len(groupIDs)),
				),
				args...,
			); err != nil {
				return 0, err
			}
		}
	case NodeGroupModeReplace:
		if _, err := tx.ExecContext(
			ctx,
			fmt.Sprintf(`DELETE FROM node_groups WHERE node_id IN (%s)`, placeholders(len(nodeIDs))),
			int64Args(nodeIDs)...,
		); err != nil {
			return 0, err
		}
		for _, nodeID := range nodeIDs {
			for _, groupID := range groupIDs {
				if _, err := tx.ExecContext(
					ctx,
					`INSERT INTO node_groups(node_id, group_id) VALUES (?, ?)`,
					nodeID,
					groupID,
				); err != nil {
					return 0, err
				}
			}
		}
	}
	if _, err := tx.ExecContext(
		ctx,
		fmt.Sprintf(`UPDATE nodes SET updated_at=CURRENT_TIMESTAMP WHERE id IN (%s)`, placeholders(len(nodeIDs))),
		int64Args(nodeIDs)...,
	); err != nil {
		return 0, err
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return nodeCount, nil
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

func (s *Store) groupChildren(ctx context.Context, id int64) ([]int64, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT child_group_id
FROM group_children
WHERE parent_group_id = ?
ORDER BY child_group_id`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []int64{}
	for rows.Next() {
		var childGroupID int64
		if err := rows.Scan(&childGroupID); err != nil {
			return nil, err
		}
		result = append(result, childGroupID)
	}
	return result, rows.Err()
}

func (s *Store) groupChildrenByParent(ctx context.Context) (map[int64][]int64, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT parent_group_id, child_group_id
FROM group_children
ORDER BY parent_group_id, child_group_id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := map[int64][]int64{}
	for rows.Next() {
		var parentGroupID, childGroupID int64
		if err := rows.Scan(&parentGroupID, &childGroupID); err != nil {
			return nil, err
		}
		result[parentGroupID] = append(result[parentGroupID], childGroupID)
	}
	return result, rows.Err()
}

func setGroupChildren(
	ctx context.Context,
	tx *sql.Tx,
	parentGroupID int64,
	childGroupIDs []int64,
) error {
	unique := make(map[int64]struct{}, len(childGroupIDs))
	normalized := make([]int64, 0, len(childGroupIDs))
	for _, childGroupID := range childGroupIDs {
		if _, ok := unique[childGroupID]; ok {
			continue
		}
		unique[childGroupID] = struct{}{}
		normalized = append(normalized, childGroupID)
	}
	sort.Slice(normalized, func(left, right int) bool {
		return normalized[left] < normalized[right]
	})

	for _, childGroupID := range normalized {
		if childGroupID < 1 {
			return ErrGroupChildNotFound
		}
		if childGroupID == parentGroupID {
			return ErrGroupCycle
		}
		var exists int
		err := tx.QueryRowContext(
			ctx,
			`SELECT 1 FROM groups WHERE id = ?`,
			childGroupID,
		).Scan(&exists)
		if errors.Is(err, sql.ErrNoRows) {
			return ErrGroupChildNotFound
		}
		if err != nil {
			return err
		}

		var createsCycle bool
		err = tx.QueryRowContext(ctx, `
WITH RECURSIVE descendants(id) AS (
	SELECT ?
	UNION
	SELECT children.child_group_id
	FROM group_children children
	JOIN descendants descendant ON descendant.id = children.parent_group_id
)
SELECT EXISTS(SELECT 1 FROM descendants WHERE id = ?)`,
			childGroupID,
			parentGroupID,
		).Scan(&createsCycle)
		if err != nil {
			return err
		}
		if createsCycle {
			return ErrGroupCycle
		}
	}

	if _, err := tx.ExecContext(
		ctx,
		`DELETE FROM group_children WHERE parent_group_id = ?`,
		parentGroupID,
	); err != nil {
		return err
	}
	for _, childGroupID := range normalized {
		if _, err := tx.ExecContext(ctx, `
INSERT INTO group_children(parent_group_id, child_group_id)
VALUES (?, ?)`, parentGroupID, childGroupID); err != nil {
			return err
		}
	}
	return nil
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
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `DELETE FROM nodes WHERE subscription_id = ?`, id); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM subscriptions WHERE id = ?`, id); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) CreateShare(ctx context.Context, item model.Share) (model.Share, error) {
	var expiresAt any
	if item.ExpiresAt != nil {
		expiresAt = item.ExpiresAt.UTC()
	}
	result, err := s.db.ExecContext(ctx, `
INSERT INTO shares(kind, target_id, target_name, url, expires_at)
VALUES (?, ?, ?, ?, ?)`,
		item.Kind, item.TargetID, item.TargetName, item.URL, expiresAt)
	if err != nil {
		return model.Share{}, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return model.Share{}, err
	}
	return s.Share(ctx, id)
}

func (s *Store) Shares(ctx context.Context) ([]model.Share, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT id, kind, target_id, target_name, url, expires_at, created_at
FROM shares ORDER BY created_at DESC, id DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := []model.Share{}
	for rows.Next() {
		item, err := scanShare(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

func (s *Store) Share(ctx context.Context, id int64) (model.Share, error) {
	return scanShare(s.db.QueryRowContext(ctx, `
SELECT id, kind, target_id, target_name, url, expires_at, created_at
FROM shares WHERE id = ?`, id))
}

type scanner interface {
	Scan(dest ...any) error
}

func scanShare(row scanner) (model.Share, error) {
	var item model.Share
	var expiresAt sql.NullTime
	if err := row.Scan(
		&item.ID,
		&item.Kind,
		&item.TargetID,
		&item.TargetName,
		&item.URL,
		&expiresAt,
		&item.CreatedAt,
	); err != nil {
		return model.Share{}, err
	}
	if expiresAt.Valid {
		item.ExpiresAt = &expiresAt.Time
	} else {
		item.Permanent = true
	}
	return item, nil
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

func placeholders(count int) string {
	return strings.TrimSuffix(strings.Repeat("?,", count), ",")
}

func int64Args(values []int64) []any {
	result := make([]any, len(values))
	for index, value := range values {
		result[index] = value
	}
	return result
}
