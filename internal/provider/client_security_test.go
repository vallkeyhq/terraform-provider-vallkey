package provider

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAPIOrigins(t *testing.T) {
	for _, origin := range []string{"https://api.vallkey.com", "https://gateway.example.com/", "http://127.0.0.1:18471", "http://[::1]:18471"} {
		if err := validateAPIURL(origin); err != nil {
			t.Errorf("rejected %s: %v", origin, err)
		}
	}
	for _, origin := range []string{"http://example.com", "https://user:pass@example.com", "https://example.com/path", "https://example.com?x=1", "https://example.com#fragment", "file:///tmp/example", "invalid"} {
		if err := validateAPIURL(origin); err == nil {
			t.Errorf("accepted %s", origin)
		}
	}
}

func TestClientDoesNotFollowRedirects(t *testing.T) {
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { t.Error("redirect forwarded an authenticated request") }))
	defer target.Close()
	source := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Location", target.URL)
		w.WriteHeader(http.StatusTemporaryRedirect)
	}))
	defer source.Close()
	_, err := NewClient(source.URL, "synthetic-token", "test").CreateKey(context.Background(), CreateKeyInput{RealKey: "synthetic-key"})
	if err == nil {
		t.Fatal("redirect must fail")
	}
}

func TestClientDoesNotEchoUnstructuredErrorBodies(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(502)
		_, _ = w.Write([]byte("sensitive upstream response"))
	}))
	defer server.Close()
	err := NewClient(server.URL, "synthetic-token", "test").DeleteKey(context.Background(), "fixture")
	apiErr, ok := err.(*APIError)
	if !ok || apiErr.Message != "Bad Gateway" {
		t.Fatalf("unexpected error: %v", err)
	}
}
