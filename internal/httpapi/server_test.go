package httpapi

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/oai404iao/sub_manager/internal/config"
	"github.com/oai404iao/sub_manager/internal/model"
	"github.com/oai404iao/sub_manager/internal/store"
	"github.com/oai404iao/sub_manager/internal/version"
)

func TestPublicOperationalEndpoints(t *testing.T) {
	handler := New(config.Config{SigningKey: "test-signing-key"}, nil).Handler()

	t.Run("health", func(t *testing.T) {
		request := httptest.NewRequest(http.MethodGet, "/healthz", nil)
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)

		if response.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
		}
		if got := response.Header().Get("Cache-Control"); got != "no-store" {
			t.Fatalf("Cache-Control = %q", got)
		}
	})

	t.Run("version", func(t *testing.T) {
		request := httptest.NewRequest(http.MethodGet, "/api/version", nil)
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)

		if response.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
		}
		var info version.Info
		if err := json.NewDecoder(response.Body).Decode(&info); err != nil {
			t.Fatal(err)
		}
		if info.Version == "" || info.Commit == "" || info.BuildDate == "" {
			t.Fatalf("incomplete version response: %#v", info)
		}
	})
}

func TestGroupHierarchyCRUDAPI(t *testing.T) {
	database, err := store.Open(filepath.Join(t.TempDir(), "test.db"), "admin", "password")
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()

	handler := New(config.Config{SigningKey: "test-signing-key"}, database).Handler()
	login := performJSONRequest(t, handler, http.MethodPost, "/api/auth/login", map[string]any{
		"username": "admin",
		"password": "password",
	}, nil)
	if login.Code != http.StatusOK {
		t.Fatalf("login status = %d body=%s", login.Code, login.Body.String())
	}
	cookies := login.Result().Cookies()
	if len(cookies) == 0 {
		t.Fatal("login did not set a session cookie")
	}

	leafResponse := performJSONRequest(
		t,
		handler,
		http.MethodPost,
		"/api/groups",
		map[string]any{"name": "leaf", "description": ""},
		cookies[0],
	)
	if leafResponse.Code != http.StatusCreated {
		t.Fatalf("leaf status = %d body=%s", leafResponse.Code, leafResponse.Body.String())
	}
	var leaf model.Group
	if err := json.NewDecoder(leafResponse.Body).Decode(&leaf); err != nil {
		t.Fatal(err)
	}

	parentResponse := performJSONRequest(
		t,
		handler,
		http.MethodPost,
		"/api/groups",
		map[string]any{
			"name":            "parent",
			"description":     "nested",
			"child_group_ids": []int64{leaf.ID},
		},
		cookies[0],
	)
	if parentResponse.Code != http.StatusCreated {
		t.Fatalf("parent status = %d body=%s", parentResponse.Code, parentResponse.Body.String())
	}
	var parent model.Group
	if err := json.NewDecoder(parentResponse.Body).Decode(&parent); err != nil {
		t.Fatal(err)
	}
	if len(parent.ChildGroupIDs) != 1 || parent.ChildGroupIDs[0] != leaf.ID {
		t.Fatalf("unexpected parent: %#v", parent)
	}

	updatedResponse := performJSONRequest(
		t,
		handler,
		http.MethodPut,
		"/api/groups/"+strconv.FormatInt(parent.ID, 10),
		map[string]any{
			"name":            "updated parent",
			"description":     "updated",
			"child_group_ids": []int64{leaf.ID},
		},
		cookies[0],
	)
	if updatedResponse.Code != http.StatusOK {
		t.Fatalf("update status = %d body=%s", updatedResponse.Code, updatedResponse.Body.String())
	}
	if err := json.NewDecoder(updatedResponse.Body).Decode(&parent); err != nil {
		t.Fatal(err)
	}
	if parent.Name != "updated parent" || parent.Description != "updated" {
		t.Fatalf("group was not updated: %#v", parent)
	}

	cycleResponse := performJSONRequest(
		t,
		handler,
		http.MethodPut,
		"/api/groups/"+strconv.FormatInt(leaf.ID, 10),
		map[string]any{
			"name":            leaf.Name,
			"description":     leaf.Description,
			"child_group_ids": []int64{parent.ID},
		},
		cookies[0],
	)
	if cycleResponse.Code != http.StatusBadRequest ||
		!strings.Contains(cycleResponse.Body.String(), "形成循环") {
		t.Fatalf("cycle status = %d body=%s", cycleResponse.Code, cycleResponse.Body.String())
	}

	missingResponse := performJSONRequest(
		t,
		handler,
		http.MethodPut,
		"/api/groups/999999",
		map[string]any{
			"name":            "missing",
			"description":     "",
			"child_group_ids": []int64{},
		},
		cookies[0],
	)
	if missingResponse.Code != http.StatusNotFound {
		t.Fatalf("missing status = %d body=%s", missingResponse.Code, missingResponse.Body.String())
	}
}

func TestSubscriptionCRUDAPI(t *testing.T) {
	database, err := store.Open(filepath.Join(t.TempDir(), "test.db"), "admin", "password")
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()

	server := New(config.Config{SigningKey: "test-signing-key"}, database)
	server.client = &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		content := "vless://11111111-1111-1111-1111-111111111111@example.com:443?encryption=none&security=tls#api"
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(content)),
		}, nil
	})}
	handler := server.Handler()

	login := performJSONRequest(t, handler, http.MethodPost, "/api/auth/login", map[string]any{
		"username": "admin",
		"password": "password",
	}, nil)
	if login.Code != http.StatusOK {
		t.Fatalf("login status = %d body=%s", login.Code, login.Body.String())
	}
	cookies := login.Result().Cookies()
	if len(cookies) == 0 {
		t.Fatal("login did not set a session cookie")
	}

	group, err := database.CreateGroup(context.Background(), "subscriptions", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	created := performJSONRequest(t, handler, http.MethodPost, "/api/subscriptions", map[string]any{
		"name":     "original",
		"url":      "https://feed.example.com/one",
		"group_id": group.ID,
	}, cookies[0])
	if created.Code != http.StatusCreated {
		t.Fatalf("create status = %d body=%s", created.Code, created.Body.String())
	}
	var createResult struct {
		Subscription model.Subscription `json:"subscription"`
		Imported     int                `json:"imported"`
	}
	if err := json.NewDecoder(created.Body).Decode(&createResult); err != nil {
		t.Fatal(err)
	}
	if createResult.Subscription.ID == 0 || createResult.Imported != 1 {
		t.Fatalf("unexpected create result: %#v", createResult)
	}

	updated := performJSONRequest(
		t,
		handler,
		http.MethodPut,
		"/api/subscriptions/"+strconv.FormatInt(createResult.Subscription.ID, 10),
		map[string]any{
			"name":     "updated",
			"url":      "https://feed.example.com/two",
			"group_id": group.ID,
		},
		cookies[0],
	)
	if updated.Code != http.StatusOK {
		t.Fatalf("update status = %d body=%s", updated.Code, updated.Body.String())
	}
	subscription, err := database.Subscription(context.Background(), createResult.Subscription.ID)
	if err != nil {
		t.Fatal(err)
	}
	if subscription.Name != "updated" || subscription.URL != "https://feed.example.com/two" {
		t.Fatalf("subscription was not updated: %#v", subscription)
	}
	nodes, err := database.Nodes(context.Background(), 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(nodes) != 1 {
		t.Fatalf("refresh should replace subscription nodes, got %d", len(nodes))
	}

	deleted := performJSONRequest(
		t,
		handler,
		http.MethodDelete,
		"/api/subscriptions/"+strconv.FormatInt(createResult.Subscription.ID, 10),
		nil,
		cookies[0],
	)
	if deleted.Code != http.StatusNoContent {
		t.Fatalf("delete status = %d body=%s", deleted.Code, deleted.Body.String())
	}
	nodes, err = database.Nodes(context.Background(), 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(nodes) != 0 {
		t.Fatalf("delete should remove managed nodes, got %d", len(nodes))
	}
}

func TestBatchNodeOperationsAPI(t *testing.T) {
	database, err := store.Open(filepath.Join(t.TempDir(), "test.db"), "admin", "password")
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()

	handler := New(config.Config{SigningKey: "test-signing-key"}, database).Handler()
	login := performJSONRequest(t, handler, http.MethodPost, "/api/auth/login", map[string]any{
		"username": "admin",
		"password": "password",
	}, nil)
	if login.Code != http.StatusOK {
		t.Fatalf("login status = %d body=%s", login.Code, login.Body.String())
	}
	cookies := login.Result().Cookies()
	if len(cookies) == 0 {
		t.Fatal("login did not set a session cookie")
	}

	firstGroup, err := database.CreateGroup(context.Background(), "batch first", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	secondGroup, err := database.CreateGroup(context.Background(), "batch second", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	firstNode, err := database.SaveNode(context.Background(), model.Node{
		Name:     "batch first",
		Protocol: "socks5",
		Server:   "first.example.com",
		Port:     1080,
		Username: "first",
		Password: "password",
		GroupIDs: []int64{firstGroup.ID},
	})
	if err != nil {
		t.Fatal(err)
	}
	secondNode, err := database.SaveNode(context.Background(), model.Node{
		Name:     "batch second",
		Protocol: "socks5",
		Server:   "second.example.com",
		Port:     1080,
		Username: "second",
		Password: "password",
		GroupIDs: []int64{firstGroup.ID},
	})
	if err != nil {
		t.Fatal(err)
	}
	nodeIDs := []int64{secondNode.ID, firstNode.ID}

	uriResponse := performJSONRequest(
		t,
		handler,
		http.MethodPost,
		"/api/nodes/export",
		map[string]any{"ids": nodeIDs, "format": "uri"},
		cookies[0],
	)
	if uriResponse.Code != http.StatusOK {
		t.Fatalf("URI export status = %d body=%s", uriResponse.Code, uriResponse.Body.String())
	}
	var uriExport struct {
		Content string `json:"content"`
		Count   int    `json:"count"`
	}
	if err := json.NewDecoder(uriResponse.Body).Decode(&uriExport); err != nil {
		t.Fatal(err)
	}
	uriLines := strings.Split(uriExport.Content, "\n")
	if uriExport.Count != 2 || len(uriLines) != 2 {
		t.Fatalf("unexpected URI export: %#v", uriExport)
	}
	firstURI, err := url.Parse(uriLines[0])
	if err != nil {
		t.Fatal(err)
	}
	secondURI, err := url.Parse(uriLines[1])
	if err != nil {
		t.Fatal(err)
	}
	if firstURI.Fragment != secondNode.Name || secondURI.Fragment != firstNode.Name {
		t.Fatalf("export order = %q, %q", firstURI.Fragment, secondURI.Fragment)
	}

	base64Response := performJSONRequest(
		t,
		handler,
		http.MethodPost,
		"/api/nodes/export",
		map[string]any{"ids": nodeIDs, "format": "base64"},
		cookies[0],
	)
	if base64Response.Code != http.StatusOK {
		t.Fatalf("Base64 export status = %d body=%s", base64Response.Code, base64Response.Body.String())
	}
	var base64Export struct {
		Content string `json:"content"`
		Count   int    `json:"count"`
	}
	if err := json.NewDecoder(base64Response.Body).Decode(&base64Export); err != nil {
		t.Fatal(err)
	}
	decoded, err := base64.StdEncoding.DecodeString(base64Export.Content)
	if err != nil {
		t.Fatal(err)
	}
	if base64Export.Count != 2 || string(decoded) != uriExport.Content {
		t.Fatalf("unexpected Base64 export: %#v decoded=%q", base64Export, decoded)
	}

	groupResponse := performJSONRequest(
		t,
		handler,
		http.MethodPatch,
		"/api/nodes/groups",
		map[string]any{
			"ids":       nodeIDs,
			"group_ids": []int64{secondGroup.ID},
			"mode":      "replace",
		},
		cookies[0],
	)
	if groupResponse.Code != http.StatusOK {
		t.Fatalf("group update status = %d body=%s", groupResponse.Code, groupResponse.Body.String())
	}
	for _, nodeID := range nodeIDs {
		node, err := database.Node(context.Background(), nodeID)
		if err != nil {
			t.Fatal(err)
		}
		if len(node.GroupIDs) != 1 || node.GroupIDs[0] != secondGroup.ID {
			t.Fatalf("node groups were not replaced: %#v", node.GroupIDs)
		}
	}

	subscription, err := database.CreateSubscription(
		context.Background(),
		"batch managed",
		"https://example.com/subscription",
		firstGroup.ID,
	)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := database.ReplaceSubscriptionNodes(
		context.Background(),
		subscription.ID,
		firstGroup.ID,
		[]model.Node{
			{
				Name:     "managed one",
				Protocol: "socks5",
				Server:   "managed-one.example.com",
				Port:     1080,
			},
			{
				Name:     "managed two",
				Protocol: "socks5",
				Server:   "managed-two.example.com",
				Port:     1080,
			},
		},
	); err != nil {
		t.Fatal(err)
	}
	allNodes, err := database.Nodes(context.Background(), 0)
	if err != nil {
		t.Fatal(err)
	}
	managedNodes := []model.Node{}
	for _, node := range allNodes {
		if node.SubscriptionID != nil {
			managedNodes = append(managedNodes, node)
		}
	}
	if len(managedNodes) != 2 {
		t.Fatalf("managed nodes = %d, want 2", len(managedNodes))
	}
	managedResponse := performJSONRequest(
		t,
		handler,
		http.MethodPatch,
		"/api/nodes/groups",
		map[string]any{
			"ids":       []int64{managedNodes[0].ID},
			"group_ids": []int64{secondGroup.ID},
			"mode":      "add",
		},
		cookies[0],
	)
	if managedResponse.Code != http.StatusBadRequest ||
		!strings.Contains(managedResponse.Body.String(), "整条订阅迁移") {
		t.Fatalf(
			"managed group update status = %d body=%s",
			managedResponse.Code,
			managedResponse.Body.String(),
		)
	}
	managedResponse = performJSONRequest(
		t,
		handler,
		http.MethodPatch,
		"/api/nodes/groups",
		map[string]any{
			"ids":       []int64{managedNodes[0].ID},
			"group_ids": []int64{secondGroup.ID},
			"mode":      "replace",
		},
		cookies[0],
	)
	if managedResponse.Code != http.StatusOK {
		t.Fatalf(
			"managed replace status = %d body=%s",
			managedResponse.Code,
			managedResponse.Body.String(),
		)
	}
	var managedResult struct {
		Updated       int `json:"updated"`
		Subscriptions int `json:"subscriptions"`
	}
	if err := json.NewDecoder(managedResponse.Body).Decode(&managedResult); err != nil {
		t.Fatal(err)
	}
	if managedResult.Updated != 2 || managedResult.Subscriptions != 1 {
		t.Fatalf("unexpected managed replace result: %#v", managedResult)
	}
	subscription, err = database.Subscription(context.Background(), subscription.ID)
	if err != nil {
		t.Fatal(err)
	}
	if subscription.GroupID != secondGroup.ID {
		t.Fatalf("subscription group = %d, want %d", subscription.GroupID, secondGroup.ID)
	}
	for _, managedNode := range managedNodes {
		node, err := database.Node(context.Background(), managedNode.ID)
		if err != nil {
			t.Fatal(err)
		}
		if len(node.GroupIDs) != 1 || node.GroupIDs[0] != secondGroup.ID {
			t.Fatalf("managed node was not migrated: %#v", node.GroupIDs)
		}
	}
	managedNodes[0].GroupIDs = []int64{firstGroup.ID}
	saveManagedResponse := performJSONRequest(
		t,
		handler,
		http.MethodPut,
		"/api/nodes/"+strconv.FormatInt(managedNodes[0].ID, 10),
		managedNodes[0],
		cookies[0],
	)
	if saveManagedResponse.Code != http.StatusOK {
		t.Fatalf(
			"save managed node status = %d body=%s",
			saveManagedResponse.Code,
			saveManagedResponse.Body.String(),
		)
	}
	subscription, err = database.Subscription(context.Background(), subscription.ID)
	if err != nil {
		t.Fatal(err)
	}
	if subscription.GroupID != firstGroup.ID {
		t.Fatalf(
			"saved subscription group = %d, want %d",
			subscription.GroupID,
			firstGroup.ID,
		)
	}
	for _, managedNode := range managedNodes {
		node, err := database.Node(context.Background(), managedNode.ID)
		if err != nil {
			t.Fatal(err)
		}
		if len(node.GroupIDs) != 1 || node.GroupIDs[0] != firstGroup.ID {
			t.Fatalf("saved managed node was not migrated: %#v", node.GroupIDs)
		}
	}
	if err := database.DeleteSubscription(context.Background(), subscription.ID); err != nil {
		t.Fatal(err)
	}

	deleteResponse := performJSONRequest(
		t,
		handler,
		http.MethodDelete,
		"/api/nodes",
		map[string]any{"ids": nodeIDs},
		cookies[0],
	)
	if deleteResponse.Code != http.StatusOK {
		t.Fatalf("batch delete status = %d body=%s", deleteResponse.Code, deleteResponse.Body.String())
	}
	var deleteResult struct {
		Deleted int `json:"deleted"`
	}
	if err := json.NewDecoder(deleteResponse.Body).Decode(&deleteResult); err != nil {
		t.Fatal(err)
	}
	if deleteResult.Deleted != 2 {
		t.Fatalf("deleted = %d, want 2", deleteResult.Deleted)
	}
	nodes, err := database.Nodes(context.Background(), 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(nodes) != 0 {
		t.Fatalf("nodes still exist after batch delete: %#v", nodes)
	}
}

func TestShareHistoryPermanentAndQRCodeVariants(t *testing.T) {
	database, err := store.Open(filepath.Join(t.TempDir(), "test.db"), "admin", "password")
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()

	server := New(config.Config{
		BaseURL:    "https://share.example",
		SigningKey: "test-signing-key",
	}, database)
	handler := server.Handler()

	login := performJSONRequest(t, handler, http.MethodPost, "/api/auth/login", map[string]any{
		"username": "admin",
		"password": "password",
	}, nil)
	if login.Code != http.StatusOK {
		t.Fatalf("login status = %d body=%s", login.Code, login.Body.String())
	}
	cookies := login.Result().Cookies()
	if len(cookies) == 0 {
		t.Fatal("login did not set a session cookie")
	}

	leafGroup, err := database.CreateGroup(context.Background(), "shared leaf", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	childGroup, err := database.CreateGroup(
		context.Background(),
		"shared child",
		"",
		[]int64{leafGroup.ID},
	)
	if err != nil {
		t.Fatal(err)
	}
	group, err := database.CreateGroup(
		context.Background(),
		"shared group",
		"",
		[]int64{childGroup.ID},
	)
	if err != nil {
		t.Fatal(err)
	}
	node, err := database.SaveNode(context.Background(), model.Node{
		Name:     "shared node",
		Protocol: "socks5",
		Server:   "proxy.example.com",
		Port:     1080,
		Username: "user",
		Password: "password",
		GroupIDs: []int64{leafGroup.ID},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := database.SaveNode(context.Background(), model.Node{
		Name:     "parent node",
		Protocol: "socks5",
		Server:   "parent.example.com",
		Port:     1080,
		GroupIDs: []int64{group.ID},
	}); err != nil {
		t.Fatal(err)
	}

	nodeResponse := performJSONRequest(t, handler, http.MethodPost, "/api/shares", map[string]any{
		"kind":          "node",
		"id":            node.ID,
		"expires_hours": 24,
		"permanent":     false,
	}, cookies[0])
	if nodeResponse.Code != http.StatusCreated {
		t.Fatalf("node share status = %d body=%s", nodeResponse.Code, nodeResponse.Body.String())
	}
	var nodePayload map[string]json.RawMessage
	if err := json.NewDecoder(nodeResponse.Body).Decode(&nodePayload); err != nil {
		t.Fatal(err)
	}
	for _, legacyField := range []string{
		"subscription_url",
		"nodes_url",
		"qr_subscription_url",
		"qr_nodes_url",
	} {
		if _, ok := nodePayload[legacyField]; ok {
			t.Fatalf("legacy field %q is still present", legacyField)
		}
	}
	nodeData, err := json.Marshal(nodePayload)
	if err != nil {
		t.Fatal(err)
	}
	var nodeShare model.Share
	if err := json.Unmarshal(nodeData, &nodeShare); err != nil {
		t.Fatal(err)
	}
	if nodeShare.URL == "" || nodeShare.QRURL == "" || nodeShare.QRURIURL == "" {
		t.Fatalf("node share is missing URL or QR codes: %#v", nodeShare)
	}
	if nodeShare.Permanent || nodeShare.ExpiresAt == nil {
		t.Fatalf("node share should expire: %#v", nodeShare)
	}
	parsedNodeURL, err := url.Parse(nodeShare.URL)
	if err != nil {
		t.Fatal(err)
	}
	if parsedNodeURL.Query().Get("share") == "" {
		t.Fatalf("managed node share is missing revocation token: %q", nodeShare.URL)
	}
	nodePublicResponse := httptest.NewRecorder()
	handler.ServeHTTP(
		nodePublicResponse,
		httptest.NewRequest(http.MethodGet, parsedNodeURL.RequestURI(), nil),
	)
	if nodePublicResponse.Code != http.StatusOK {
		t.Fatalf(
			"node public share status=%d body=%q",
			nodePublicResponse.Code,
			nodePublicResponse.Body.String(),
		)
	}
	nodeQR, err := url.Parse(nodeShare.QRURIURL)
	if err != nil {
		t.Fatal(err)
	}
	if uri := nodeQR.Query().Get("data"); !strings.HasPrefix(uri, "socks5://") {
		t.Fatalf("node URI QR data = %q", uri)
	}

	groupResponse := performJSONRequest(t, handler, http.MethodPost, "/api/shares", map[string]any{
		"kind":      "group",
		"id":        group.ID,
		"permanent": true,
	}, cookies[0])
	if groupResponse.Code != http.StatusCreated {
		t.Fatalf("group share status = %d body=%s", groupResponse.Code, groupResponse.Body.String())
	}
	var groupShare model.Share
	if err := json.NewDecoder(groupResponse.Body).Decode(&groupShare); err != nil {
		t.Fatal(err)
	}
	if !groupShare.Permanent || groupShare.ExpiresAt != nil {
		t.Fatalf("group share should be permanent: %#v", groupShare)
	}
	if groupShare.URL == "" || groupShare.QRURL == "" || groupShare.QRURIURL != "" {
		t.Fatalf("group QR variants are incorrect: %#v", groupShare)
	}
	parsedGroupURL, err := url.Parse(groupShare.URL)
	if err != nil {
		t.Fatal(err)
	}
	if got := parsedGroupURL.Query().Get("exp"); got != "0" {
		t.Fatalf("permanent exp = %q, want 0", got)
	}
	if got := parsedGroupURL.Query().Get("content"); got != "subscription" {
		t.Fatalf("share content = %q, want subscription", got)
	}

	publicResponse := httptest.NewRecorder()
	handler.ServeHTTP(
		publicResponse,
		httptest.NewRequest(http.MethodGet, parsedGroupURL.RequestURI(), nil),
	)
	if publicResponse.Code != http.StatusOK {
		t.Fatalf("public share status = %d body=%s", publicResponse.Code, publicResponse.Body.String())
	}
	decoded, err := base64.StdEncoding.DecodeString(strings.TrimSpace(publicResponse.Body.String()))
	if err != nil {
		t.Fatalf("public share is not a Base64 subscription: %v", err)
	}
	if !strings.HasPrefix(string(decoded), "socks5://") {
		t.Fatalf("decoded subscription = %q", decoded)
	}
	if lines := strings.Split(strings.TrimSpace(string(decoded)), "\n"); len(lines) != 2 {
		t.Fatalf("nested group share contains %d nodes, want 2: %q", len(lines), decoded)
	}

	legacyNodesURL := server.signedURL(
		"node",
		node.ID,
		"nodes",
		time.Now().Add(time.Hour).Unix(),
	)
	parsedLegacyURL, err := url.Parse(legacyNodesURL)
	if err != nil {
		t.Fatal(err)
	}
	legacyResponse := httptest.NewRecorder()
	handler.ServeHTTP(
		legacyResponse,
		httptest.NewRequest(http.MethodGet, parsedLegacyURL.RequestURI(), nil),
	)
	if legacyResponse.Code != http.StatusOK ||
		!strings.HasPrefix(strings.TrimSpace(legacyResponse.Body.String()), "socks5://") {
		t.Fatalf(
			"legacy node share status=%d body=%q",
			legacyResponse.Code,
			legacyResponse.Body.String(),
		)
	}

	stateResponse := performJSONRequest(t, handler, http.MethodGet, "/api/state", nil, cookies[0])
	if stateResponse.Code != http.StatusOK {
		t.Fatalf("state status = %d body=%s", stateResponse.Code, stateResponse.Body.String())
	}
	var state model.State
	if err := json.NewDecoder(stateResponse.Body).Decode(&state); err != nil {
		t.Fatal(err)
	}
	if len(state.Shares) != 2 {
		t.Fatalf("share history count = %d, want 2", len(state.Shares))
	}
	if state.Shares[0].ID != groupShare.ID || state.Shares[0].QRURIURL != "" {
		t.Fatalf("unexpected group share history: %#v", state.Shares[0])
	}
	if state.Shares[1].ID != nodeShare.ID || state.Shares[1].QRURIURL == "" {
		t.Fatalf("unexpected node share history: %#v", state.Shares[1])
	}

	revokeResponse := performJSONRequest(
		t,
		handler,
		http.MethodPost,
		"/api/shares/"+strconv.FormatInt(nodeShare.ID, 10)+"/revoke",
		nil,
		cookies[0],
	)
	if revokeResponse.Code != http.StatusOK {
		t.Fatalf(
			"revoke share status=%d body=%s",
			revokeResponse.Code,
			revokeResponse.Body.String(),
		)
	}
	var revokedShare model.Share
	if err := json.NewDecoder(revokeResponse.Body).Decode(&revokedShare); err != nil {
		t.Fatal(err)
	}
	if !revokedShare.Revoked || revokedShare.RevokedAt == nil {
		t.Fatalf("share was not revoked: %#v", revokedShare)
	}
	revokedPublicResponse := httptest.NewRecorder()
	handler.ServeHTTP(
		revokedPublicResponse,
		httptest.NewRequest(http.MethodGet, parsedNodeURL.RequestURI(), nil),
	)
	if revokedPublicResponse.Code != http.StatusGone {
		t.Fatalf(
			"revoked public share status=%d body=%q",
			revokedPublicResponse.Code,
			revokedPublicResponse.Body.String(),
		)
	}
	deleteNodeShareResponse := performJSONRequest(
		t,
		handler,
		http.MethodDelete,
		"/api/shares/"+strconv.FormatInt(nodeShare.ID, 10),
		nil,
		cookies[0],
	)
	if deleteNodeShareResponse.Code != http.StatusNoContent {
		t.Fatalf(
			"delete node share status=%d body=%s",
			deleteNodeShareResponse.Code,
			deleteNodeShareResponse.Body.String(),
		)
	}
	deletedPublicResponse := httptest.NewRecorder()
	handler.ServeHTTP(
		deletedPublicResponse,
		httptest.NewRequest(http.MethodGet, parsedNodeURL.RequestURI(), nil),
	)
	if deletedPublicResponse.Code != http.StatusGone {
		t.Fatalf(
			"deleted public share status=%d body=%q",
			deletedPublicResponse.Code,
			deletedPublicResponse.Body.String(),
		)
	}

	deleteGroupShareResponse := performJSONRequest(
		t,
		handler,
		http.MethodDelete,
		"/api/shares/"+strconv.FormatInt(groupShare.ID, 10),
		nil,
		cookies[0],
	)
	if deleteGroupShareResponse.Code != http.StatusNoContent {
		t.Fatalf(
			"delete group share status=%d body=%s",
			deleteGroupShareResponse.Code,
			deleteGroupShareResponse.Body.String(),
		)
	}
	deletedGroupPublicResponse := httptest.NewRecorder()
	handler.ServeHTTP(
		deletedGroupPublicResponse,
		httptest.NewRequest(http.MethodGet, parsedGroupURL.RequestURI(), nil),
	)
	if deletedGroupPublicResponse.Code != http.StatusGone {
		t.Fatalf(
			"deleted group public share status=%d body=%q",
			deletedGroupPublicResponse.Code,
			deletedGroupPublicResponse.Body.String(),
		)
	}

	legacyShare, err := database.CreateShare(context.Background(), model.Share{
		Kind:       "node",
		TargetID:   node.ID,
		TargetName: node.Name,
		URL:        legacyNodesURL,
	})
	if err != nil {
		t.Fatal(err)
	}
	legacyRevokeResponse := performJSONRequest(
		t,
		handler,
		http.MethodPost,
		"/api/shares/"+strconv.FormatInt(legacyShare.ID, 10)+"/revoke",
		nil,
		cookies[0],
	)
	if legacyRevokeResponse.Code != http.StatusOK {
		t.Fatalf(
			"legacy revoke status=%d body=%s",
			legacyRevokeResponse.Code,
			legacyRevokeResponse.Body.String(),
		)
	}
	revokedLegacyResponse := httptest.NewRecorder()
	handler.ServeHTTP(
		revokedLegacyResponse,
		httptest.NewRequest(http.MethodGet, parsedLegacyURL.RequestURI(), nil),
	)
	if revokedLegacyResponse.Code != http.StatusGone {
		t.Fatalf(
			"revoked legacy share status=%d body=%q",
			revokedLegacyResponse.Code,
			revokedLegacyResponse.Body.String(),
		)
	}
	legacyDeleteResponse := performJSONRequest(
		t,
		handler,
		http.MethodDelete,
		"/api/shares/"+strconv.FormatInt(legacyShare.ID, 10),
		nil,
		cookies[0],
	)
	if legacyDeleteResponse.Code != http.StatusNoContent {
		t.Fatalf(
			"legacy delete status=%d body=%s",
			legacyDeleteResponse.Code,
			legacyDeleteResponse.Body.String(),
		)
	}
	deletedLegacyResponse := httptest.NewRecorder()
	handler.ServeHTTP(
		deletedLegacyResponse,
		httptest.NewRequest(http.MethodGet, parsedLegacyURL.RequestURI(), nil),
	)
	if deletedLegacyResponse.Code != http.StatusGone {
		t.Fatalf(
			"deleted legacy share status=%d body=%q",
			deletedLegacyResponse.Code,
			deletedLegacyResponse.Body.String(),
		)
	}

	stateResponse = performJSONRequest(t, handler, http.MethodGet, "/api/state", nil, cookies[0])
	if stateResponse.Code != http.StatusOK {
		t.Fatalf("state after share management status = %d", stateResponse.Code)
	}
	if err := json.NewDecoder(stateResponse.Body).Decode(&state); err != nil {
		t.Fatal(err)
	}
	if len(state.Shares) != 0 {
		t.Fatalf("deleted shares are still listed: %#v", state.Shares)
	}

	expiredURL := server.signedURL(
		"node",
		node.ID,
		"subscription",
		time.Now().Add(-time.Hour).Unix(),
	)
	parsedExpiredURL, err := url.Parse(expiredURL)
	if err != nil {
		t.Fatal(err)
	}
	expiredResponse := httptest.NewRecorder()
	handler.ServeHTTP(
		expiredResponse,
		httptest.NewRequest(http.MethodGet, parsedExpiredURL.RequestURI(), nil),
	)
	if expiredResponse.Code != http.StatusGone {
		t.Fatalf("expired share status = %d, want %d", expiredResponse.Code, http.StatusGone)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (function roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return function(request)
}

func performJSONRequest(
	t *testing.T,
	handler http.Handler,
	method string,
	path string,
	body any,
	cookie *http.Cookie,
) *httptest.ResponseRecorder {
	t.Helper()
	var requestBody io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
		requestBody = bytes.NewReader(data)
	}
	request := httptest.NewRequest(method, path, requestBody)
	request.Header.Set("Content-Type", "application/json")
	if cookie != nil {
		request.AddCookie(cookie)
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}
