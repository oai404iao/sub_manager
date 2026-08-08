package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/oai404iao/sub_manager/internal/config"
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
