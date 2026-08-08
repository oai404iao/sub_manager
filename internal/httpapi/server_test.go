package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

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

	group, err := database.CreateGroup(context.Background(), "subscriptions", "")
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
