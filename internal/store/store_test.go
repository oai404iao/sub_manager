package store

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"submanager/internal/model"
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
