package ha

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestNotifyRequest(t *testing.T) {
	var gotPath, gotAuth, gotMethod string
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath, gotAuth, gotMethod = r.URL.Path, r.Header.Get("Authorization"), r.Method
		json.NewDecoder(r.Body).Decode(&gotBody)
		w.Write([]byte("[]"))
	}))
	defer srv.Close()

	err := New(srv.URL, "tok").Notify(context.Background(), "notify.mobile_app_my_phone", "✅ Café", "msg", map[string]any{"ttl": 0, "tag": "t"})
	if err != nil {
		t.Fatal(err)
	}
	if gotMethod != "POST" || gotPath != "/api/services/notify/mobile_app_my_phone" || gotAuth != "Bearer tok" {
		t.Fatalf("method=%s path=%s auth=%s", gotMethod, gotPath, gotAuth)
	}
	if gotBody["title"] != "✅ Café" || gotBody["message"] != "msg" {
		t.Fatalf("body = %#v", gotBody)
	}
	data := gotBody["data"].(map[string]any)
	if data["ttl"] != float64(0) || data["tag"] != "t" {
		t.Fatalf("data = %#v", data)
	}
}

func TestNotifyTrailingSlashBase(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { gotPath = r.URL.Path }))
	defer srv.Close()
	if err := New(srv.URL+"/", "tok").Notify(context.Background(), "notify.x", "t", "m", nil); err != nil {
		t.Fatal(err)
	}
	if gotPath != "/api/services/notify/x" {
		t.Fatalf("path = %q", gotPath)
	}
}

func TestNotifyHTTPErrorShowsStatusNotToken(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "401: Unauthorized", http.StatusUnauthorized)
	}))
	defer srv.Close()
	err := New(srv.URL, "supersecret").Notify(context.Background(), "notify.x", "t", "m", nil)
	if err == nil || !strings.Contains(err.Error(), "401") || strings.Contains(err.Error(), "supersecret") {
		t.Fatalf("err = %v", err)
	}
}

func TestNotifyBadTarget(t *testing.T) {
	err := New("http://x", "tok").Notify(context.Background(), "light.kitchen", "t", "m", nil)
	if err == nil || !strings.Contains(err.Error(), "notify.") {
		t.Fatalf("err = %v", err)
	}
}

func TestPing(t *testing.T) {
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		if r.URL.Path != "/api/" {
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	if err := New(srv.URL, "tok").Ping(context.Background()); err != nil {
		t.Fatal(err)
	}
	if gotAuth != "Bearer tok" {
		t.Fatalf("auth = %q", gotAuth)
	}
}
