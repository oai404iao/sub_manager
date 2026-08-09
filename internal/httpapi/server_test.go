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
