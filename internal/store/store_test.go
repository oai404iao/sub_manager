package store

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/oai404iao/sub_manager/internal/model"
)

func TestStateLoadsNodeGroups(t *testing.T) {
	database, err := Open(filepath.Join(t.TempDir(), "test.db"), "admin", "password")
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	group, err := database.CreateGroup(ctx, "office", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := database.SaveNode(ctx, model.Node{
		Name:     "proxy",
		Protocol: "socks5",
		Server:   "example.com",
		Port:     1080,
		GroupIDs: []int64{group.ID},
	}); err != nil {
		t.Fatal(err)
	}
	state, err := database.State(ctx, model.User{ID: 1, Username: "admin"})
	if err != nil {
		t.Fatal(err)
	}
	if len(state.Nodes) != 1 || len(state.Nodes[0].GroupIDs) != 1 {
		t.Fatalf("unexpected state: %#v", state)
	}
}

func TestNestedGroupsAggregateNodesAndRejectCycles(t *testing.T) {
	database, err := Open(filepath.Join(t.TempDir(), "test.db"), "admin", "password")
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	leafOne, err := database.CreateGroup(ctx, "leaf one", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	leafTwo, err := database.CreateGroup(ctx, "leaf two", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	middle, err := database.CreateGroup(ctx, "middle", "", []int64{leafOne.ID})
	if err != nil {
		t.Fatal(err)
	}
	root, err := database.CreateGroup(
		ctx,
		"root",
		"",
		[]int64{middle.ID, leafTwo.ID},
	)
	if err != nil {
		t.Fatal(err)
	}

	testNodes := []model.Node{
		{Name: "leaf one", Protocol: "socks5", Server: "one.example.com", Port: 1080, GroupIDs: []int64{leafOne.ID}},
		{Name: "middle", Protocol: "socks5", Server: "middle.example.com", Port: 1080, GroupIDs: []int64{middle.ID}},
		{Name: "leaf two", Protocol: "socks5", Server: "two.example.com", Port: 1080, GroupIDs: []int64{leafTwo.ID}},
		{Name: "root", Protocol: "socks5", Server: "root.example.com", Port: 1080, GroupIDs: []int64{root.ID}},
		{
			Name:     "shared",
			Protocol: "socks5",
			Server:   "shared.example.com",
			Port:     1080,
			GroupIDs: []int64{leafOne.ID, leafTwo.ID},
		},
	}
	for _, node := range testNodes {
		if _, err := database.SaveNode(ctx, node); err != nil {
			t.Fatal(err)
		}
	}

	root, err = database.Group(ctx, root.ID)
	if err != nil {
		t.Fatal(err)
	}
	if root.NodeCount != 5 {
		t.Fatalf("root node count = %d, want 5", root.NodeCount)
	}
	if len(root.ChildGroupIDs) != 2 ||
		!containsID(root.ChildGroupIDs, middle.ID) ||
		!containsID(root.ChildGroupIDs, leafTwo.ID) {
		t.Fatalf("unexpected root children: %#v", root.ChildGroupIDs)
	}
	nodes, err := database.Nodes(ctx, root.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(nodes) != 5 {
		t.Fatalf("root nodes = %d, want 5: %#v", len(nodes), nodes)
	}

	groups, err := database.Groups(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var listedRoot model.Group
	for _, group := range groups {
		if group.ID == root.ID {
			listedRoot = group
			break
		}
	}
	if listedRoot.ID == 0 || listedRoot.NodeCount != 5 ||
		len(listedRoot.ChildGroupIDs) != 2 {
		t.Fatalf("unexpected listed root: %#v", listedRoot)
	}

	if _, err := database.UpdateGroup(
		ctx,
		leafOne.ID,
		leafOne.Name,
		leafOne.Description,
		[]int64{root.ID},
	); !errors.Is(err, ErrGroupCycle) {
		t.Fatalf("cycle error = %v, want %v", err, ErrGroupCycle)
	}
	if _, err := database.UpdateGroup(
		ctx,
		root.ID,
		root.Name,
		root.Description,
		[]int64{root.ID},
	); !errors.Is(err, ErrGroupCycle) {
		t.Fatalf("self cycle error = %v, want %v", err, ErrGroupCycle)
	}
	if _, err := database.UpdateGroup(
		ctx,
		root.ID,
		root.Name,
		root.Description,
		[]int64{999999},
	); !errors.Is(err, ErrGroupChildNotFound) {
		t.Fatalf("missing child error = %v, want %v", err, ErrGroupChildNotFound)
	}

	root, err = database.UpdateGroup(
		ctx,
		root.ID,
		"updated root",
		"updated",
		[]int64{leafTwo.ID},
	)
	if err != nil {
		t.Fatal(err)
	}
	if root.Name != "updated root" || root.NodeCount != 3 ||
		len(root.ChildGroupIDs) != 1 || root.ChildGroupIDs[0] != leafTwo.ID {
		t.Fatalf("unexpected updated root: %#v", root)
	}
	nodes, err = database.Nodes(ctx, root.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(nodes) != 3 {
		t.Fatalf("updated root nodes = %d, want 3: %#v", len(nodes), nodes)
	}
}

func TestOpenMigratesExistingDatabaseForGroupHierarchy(t *testing.T) {
	path := filepath.Join(t.TempDir(), "legacy.db")
	legacy, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := legacy.Exec(`
CREATE TABLE groups (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	name TEXT NOT NULL UNIQUE,
	description TEXT NOT NULL DEFAULT '',
	created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
INSERT INTO groups(name, description) VALUES ('legacy', 'existing database');
`); err != nil {
		_ = legacy.Close()
		t.Fatal(err)
	}
	if err := legacy.Close(); err != nil {
		t.Fatal(err)
	}

	database, err := Open(path, "admin", "password")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	child, err := database.CreateGroup(ctx, "child", "", nil)
	if err != nil {
		_ = database.Close()
		t.Fatal(err)
	}
	parent, err := database.UpdateGroup(
		ctx,
		1,
		"legacy",
		"existing database",
		[]int64{child.ID},
	)
	if err != nil {
		_ = database.Close()
		t.Fatal(err)
	}
	if len(parent.ChildGroupIDs) != 1 || parent.ChildGroupIDs[0] != child.ID {
		_ = database.Close()
		t.Fatalf("unexpected migrated hierarchy: %#v", parent)
	}
	if err := database.Close(); err != nil {
		t.Fatal(err)
	}

	reopened, err := Open(path, "admin", "password")
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	parent, err = reopened.Group(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(parent.ChildGroupIDs) != 1 || parent.ChildGroupIDs[0] != child.ID {
		t.Fatalf("hierarchy did not survive reopen: %#v", parent)
	}
}

func TestSubscriptionCRUDAndNodeReplacement(t *testing.T) {
	database, err := Open(filepath.Join(t.TempDir(), "test.db"), "admin", "password")
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	firstGroup, err := database.CreateGroup(ctx, "first", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	secondGroup, err := database.CreateGroup(ctx, "second", "", nil)
	if err != nil {
		t.Fatal(err)
	}

	subscription, err := database.CreateSubscription(ctx, "original", "https://example.com/one", firstGroup.ID)
	if err != nil {
		t.Fatal(err)
	}
	subscription, err = database.UpdateSubscription(
		ctx,
		subscription.ID,
		"updated",
		"https://example.com/two",
		secondGroup.ID,
	)
	if err != nil {
		t.Fatal(err)
	}
	if subscription.Name != "updated" || subscription.URL != "https://example.com/two" ||
		subscription.GroupID != secondGroup.ID {
		t.Fatalf("unexpected subscription: %#v", subscription)
	}

	firstNodes := []model.Node{
		{Name: "one", Protocol: "socks5", Server: "one.example.com", Port: 1080},
		{Name: "two", Protocol: "socks5", Server: "two.example.com", Port: 1080},
	}
	if count, err := database.ReplaceSubscriptionNodes(ctx, subscription.ID, secondGroup.ID, firstNodes); err != nil || count != 2 {
		t.Fatalf("first replacement count=%d err=%v", count, err)
	}
	if count, err := database.ReplaceSubscriptionNodes(ctx, subscription.ID, secondGroup.ID, firstNodes[:1]); err != nil || count != 1 {
		t.Fatalf("second replacement count=%d err=%v", count, err)
	}
	nodes, err := database.Nodes(ctx, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(nodes) != 1 || nodes[0].SubscriptionID == nil || *nodes[0].SubscriptionID != subscription.ID ||
		len(nodes[0].GroupIDs) != 1 || nodes[0].GroupIDs[0] != secondGroup.ID {
		t.Fatalf("subscription nodes were not replaced: %#v", nodes)
	}

	if err := database.DeleteSubscription(ctx, subscription.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Subscription(ctx, subscription.ID); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("expected deleted subscription, got %v", err)
	}
	nodes, err = database.Nodes(ctx, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(nodes) != 0 {
		t.Fatalf("deleting a subscription should remove managed nodes: %#v", nodes)
	}
}

func TestBatchNodeOperations(t *testing.T) {
	database, err := Open(filepath.Join(t.TempDir(), "test.db"), "admin", "password")
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	firstGroup, err := database.CreateGroup(ctx, "batch first", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	secondGroup, err := database.CreateGroup(ctx, "batch second", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	firstNode, err := database.SaveNode(ctx, model.Node{
		Name:     "batch first",
		Protocol: "socks5",
		Server:   "first.example.com",
		Port:     1080,
		GroupIDs: []int64{firstGroup.ID},
	})
	if err != nil {
		t.Fatal(err)
	}
	secondNode, err := database.SaveNode(ctx, model.Node{
		Name:     "batch second",
		Protocol: "socks5",
		Server:   "second.example.com",
		Port:     1080,
		GroupIDs: []int64{firstGroup.ID},
	})
	if err != nil {
		t.Fatal(err)
	}

	ordered, err := database.NodesByIDs(ctx, []int64{secondNode.ID, firstNode.ID})
	if err != nil {
		t.Fatal(err)
	}
	if len(ordered) != 2 || ordered[0].ID != secondNode.ID || ordered[1].ID != firstNode.ID {
		t.Fatalf("nodes were not returned in request order: %#v", ordered)
	}

	nodeIDs := []int64{firstNode.ID, secondNode.ID}
	if updated, err := database.UpdateNodeGroups(
		ctx,
		nodeIDs,
		[]int64{secondGroup.ID},
		NodeGroupModeAdd,
	); err != nil || updated != 2 {
		t.Fatalf("add groups updated=%d err=%v", updated, err)
	}
	for _, nodeID := range nodeIDs {
		node, err := database.Node(ctx, nodeID)
		if err != nil {
			t.Fatal(err)
		}
		if len(node.GroupIDs) != 2 ||
			!containsID(node.GroupIDs, firstGroup.ID) ||
			!containsID(node.GroupIDs, secondGroup.ID) {
			t.Fatalf("groups were not added: %#v", node.GroupIDs)
		}
	}

	if updated, err := database.UpdateNodeGroups(
		ctx,
		nodeIDs,
		[]int64{firstGroup.ID},
		NodeGroupModeRemove,
	); err != nil || updated != 2 {
		t.Fatalf("remove groups updated=%d err=%v", updated, err)
	}
	for _, nodeID := range nodeIDs {
		node, err := database.Node(ctx, nodeID)
		if err != nil {
			t.Fatal(err)
		}
		if len(node.GroupIDs) != 1 || node.GroupIDs[0] != secondGroup.ID {
			t.Fatalf("groups were not removed: %#v", node.GroupIDs)
		}
	}

	if updated, err := database.UpdateNodeGroups(
		ctx,
		nodeIDs,
		nil,
		NodeGroupModeReplace,
	); err != nil || updated != 2 {
		t.Fatalf("replace groups updated=%d err=%v", updated, err)
	}
	for _, nodeID := range nodeIDs {
		node, err := database.Node(ctx, nodeID)
		if err != nil {
			t.Fatal(err)
		}
		if len(node.GroupIDs) != 0 {
			t.Fatalf("groups were not cleared: %#v", node.GroupIDs)
		}
	}

	subscription, err := database.CreateSubscription(
		ctx,
		"batch managed",
		"https://example.com/subscription",
		firstGroup.ID,
	)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := database.ReplaceSubscriptionNodes(
		ctx,
		subscription.ID,
		firstGroup.ID,
		[]model.Node{{
			Name:     "managed",
			Protocol: "socks5",
			Server:   "managed.example.com",
			Port:     1080,
		}},
	); err != nil {
		t.Fatal(err)
	}
	nodes, err := database.Nodes(ctx, 0)
	if err != nil {
		t.Fatal(err)
	}
	var managedNode model.Node
	for _, node := range nodes {
		if node.SubscriptionID != nil {
			managedNode = node
			break
		}
	}
	if managedNode.ID == 0 {
		t.Fatal("managed node was not created")
	}
	if _, err := database.UpdateNodeGroups(
		ctx,
		[]int64{managedNode.ID},
		[]int64{secondGroup.ID},
		NodeGroupModeAdd,
	); !errors.Is(err, ErrManagedNodeGroups) {
		t.Fatalf("managed node group error = %v, want %v", err, ErrManagedNodeGroups)
	}

	if _, err := database.DeleteNodes(
		ctx,
		[]int64{firstNode.ID, 999999},
	); !errors.Is(err, ErrNodeNotFound) {
		t.Fatalf("missing node delete error = %v, want %v", err, ErrNodeNotFound)
	}
	if _, err := database.Node(ctx, firstNode.ID); err != nil {
		t.Fatalf("batch delete was not atomic: %v", err)
	}
	if deleted, err := database.DeleteNodes(ctx, nodeIDs); err != nil || deleted != 2 {
		t.Fatalf("delete nodes deleted=%d err=%v", deleted, err)
	}
	if _, err := database.Node(ctx, firstNode.ID); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("first node still exists: %v", err)
	}
	if _, err := database.Node(ctx, secondNode.ID); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("second node still exists: %v", err)
	}
}

func TestShareHistory(t *testing.T) {
	database, err := Open(filepath.Join(t.TempDir(), "test.db"), "admin", "password")
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	expiresAt := time.Now().UTC().Add(24 * time.Hour).Truncate(time.Second)

	temporary, err := database.CreateShare(ctx, model.Share{
		Kind:       "node",
		TargetID:   1,
		TargetName: "temporary",
		URL:        "https://share.example/s?temporary",
		ExpiresAt:  &expiresAt,
	})
	if err != nil {
		t.Fatal(err)
	}
	permanent, err := database.CreateShare(ctx, model.Share{
		Kind:       "group",
		TargetID:   2,
		TargetName: "permanent",
		URL:        "https://share.example/s?permanent",
	})
	if err != nil {
		t.Fatal(err)
	}

	if temporary.Permanent || temporary.ExpiresAt == nil ||
		temporary.ExpiresAt.Unix() != expiresAt.Unix() {
		t.Fatalf("unexpected temporary share: %#v", temporary)
	}
	if !permanent.Permanent || permanent.ExpiresAt != nil {
		t.Fatalf("unexpected permanent share: %#v", permanent)
	}

	state, err := database.State(ctx, model.User{ID: 1, Username: "admin"})
	if err != nil {
		t.Fatal(err)
	}
	if len(state.Shares) != 2 {
		t.Fatalf("share count = %d, want 2", len(state.Shares))
	}
	if state.Shares[0].ID != permanent.ID || state.Shares[1].ID != temporary.ID {
		t.Fatalf("shares are not newest first: %#v", state.Shares)
	}
}

func containsID(ids []int64, target int64) bool {
	for _, id := range ids {
		if id == target {
			return true
		}
	}
	return false
}
