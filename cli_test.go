package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type stub struct {
	srv   *httptest.Server
	calls int
	body  map[string]any
	code  int
}

func newStub(t *testing.T) *stub {
	s := &stub{code: 200}
	s.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/services/") {
			s.calls++
			s.body = nil
			json.NewDecoder(r.Body).Decode(&s.body)
		}
		w.WriteHeader(s.code)
		w.Write([]byte("[]"))
	}))
	t.Cleanup(s.srv.Close)
	return s
}

const cfg = `{
  "url": "http://unused",
  "default": {"target": "notify.mobile_app_x", "data": {"channel": "Claude", "ttl": 0}},
  "templates": {
    "finished": {"title": "Claude is done", "message": "{task}", "data": {"notification_icon": "mdi:check-circle", "tag": "claude-{session}"}},
    "alpha": {"title": "A", "message": "a"}
  }
}`

func setup(t *testing.T, s *stub, env map[string]string) (cfgPath string, getenv func(string) string) {
	dir := t.TempDir()
	cfgPath = filepath.Join(dir, "c.json")
	if err := os.WriteFile(cfgPath, []byte(cfg), 0o600); err != nil {
		t.Fatal(err)
	}
	all := map[string]string{"HA_URL": s.srv.URL, "HA_TOKEN": "sekrit"}
	for k, v := range env {
		all[k] = v
	}
	return cfgPath, func(k string) string { return all[k] }
}

func exec(args []string, getenv func(string) string) (code int, out, errOut string) {
	var o, e bytes.Buffer
	code = run(args, getenv, &o, &e)
	return code, o.String(), e.String()
}

func TestSendFillsAndPosts(t *testing.T) {
	s := newStub(t)
	p, env := setup(t, s, nil)
	code, _, errOut := exec([]string{"send", "finished", "--config", p, "--set", "task=build ok"}, env)
	if code != 0 {
		t.Fatalf("code %d: %s", code, errOut)
	}
	if s.body["title"] != "Claude is done" || s.body["message"] != "build ok" {
		t.Fatalf("body = %#v", s.body)
	}
	data := s.body["data"].(map[string]any)
	if data["notification_icon"] != "mdi:check-circle" || data["channel"] != "Claude" || data["ttl"] != float64(0) {
		t.Fatalf("data = %#v", data)
	}
	wd, _ := os.Getwd()
	if !strings.Contains(data["tag"].(string), filepath.Base(wd)) {
		t.Fatalf("tag = %v, want cwd base name", data["tag"])
	}
}

func TestSendMissingPlaceholderExits1(t *testing.T) {
	s := newStub(t)
	p, env := setup(t, s, nil)
	code, _, errOut := exec([]string{"send", "finished", "--config", p}, env)
	if code != 1 || s.calls != 0 || !strings.Contains(errOut, "task") {
		t.Fatalf("code=%d calls=%d err=%q", code, s.calls, errOut)
	}
}

func TestSendMissingTokenNamesVariable(t *testing.T) {
	s := newStub(t)
	p, env := setup(t, s, map[string]string{"HA_TOKEN": ""})
	code, _, errOut := exec([]string{"send", "finished", "--config", p, "--set", "task=x"}, env)
	if code != 1 || !strings.Contains(errOut, "HA_TOKEN") || s.calls != 0 {
		t.Fatalf("code=%d err=%q", code, errOut)
	}
}

func TestSendUnknownTemplateListsNames(t *testing.T) {
	s := newStub(t)
	p, env := setup(t, s, nil)
	code, _, errOut := exec([]string{"send", "nope", "--config", p}, env)
	if code != 1 || !strings.Contains(errOut, "alpha, finished") {
		t.Fatalf("code=%d err=%q", code, errOut)
	}
}

func TestTitleMessageOverride(t *testing.T) {
	s := newStub(t)
	p, env := setup(t, s, nil)
	code, _, errOut := exec([]string{"send", "finished", "--config", p, "--title", "Custom title", "--message", "hello"}, env)
	if code != 0 {
		t.Fatalf("code %d: %s", code, errOut)
	}
	if s.body["title"] != "Custom title" || s.body["message"] != "hello" {
		t.Fatalf("body = %#v", s.body)
	}
}

func TestListPrintsSortedNames(t *testing.T) {
	s := newStub(t)
	p, env := setup(t, s, nil)
	code, out, _ := exec([]string{"list", "--config", p}, env)
	if code != 0 || out != "alpha\nfinished\n" {
		t.Fatalf("code=%d out=%q", code, out)
	}
}

func TestCheckDoesNotSend(t *testing.T) {
	s := newStub(t)
	p, env := setup(t, s, nil)
	code, out, errOut := exec([]string{"check", "--config", p}, env)
	if code != 0 || s.calls != 0 || !strings.Contains(out, "ok") {
		t.Fatalf("code=%d calls=%d out=%q err=%q", code, s.calls, out, errOut)
	}
}

func TestNoArgsUsageExit2(t *testing.T) {
	code, _, errOut := exec(nil, func(string) string { return "" })
	if code != 2 || !strings.Contains(errOut, "usage") {
		t.Fatalf("code=%d err=%q", code, errOut)
	}
}

func TestTokenNeverInOutput(t *testing.T) {
	s := newStub(t)
	s.code = 401
	p, env := setup(t, s, nil)
	code, out, errOut := exec([]string{"send", "alpha", "--config", p}, env)
	if code != 1 || strings.Contains(out+errOut, "sekrit") {
		t.Fatalf("code=%d out=%q err=%q", code, out, errOut)
	}
}

func TestOverrideTextIsSentVerbatim(t *testing.T) {
	s := newStub(t)
	p, env := setup(t, s, nil)
	code, _, errOut := exec([]string{"send", "finished", "--config", p, "--title", "fix {foo}", "--message", "use {{x}} and }"}, env)
	if code != 0 {
		t.Fatalf("code %d: %s", code, errOut)
	}
	if s.body["title"] != "fix {foo}" || s.body["message"] != "use {{x}} and }" {
		t.Fatalf("body = %#v", s.body)
	}
}

func TestVersionCommand(t *testing.T) {
	code, out, _ := exec([]string{"version"}, func(string) string { return "" })
	if code != 0 || out != "ha-notify "+version+"\n" {
		t.Fatalf("code=%d out=%q", code, out)
	}
}
