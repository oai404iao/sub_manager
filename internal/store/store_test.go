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
	group, err := database.CreateGroup(ctx, "office", "")
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

func TestSubscriptionCRUDAndNodeReplacement(t *testing.T) {
	database, err := Open(filepath.Join(t.TempDir(), "test.db"), "admin", "password")
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	firstGroup, err := database.CreateGroup(ctx, "first", "")
	if err != nil {
		t.Fatal(err)
	}
	secondGroup, err := database.CreateGroup(ctx, "second", "")
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
