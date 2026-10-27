package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
)

func init() { clearDelay = 0 }

// askHA fakes HA: REST notify plus the event websocket. When a notification
// with actions arrives, it "taps" the button whose id ends with tap.
type askHA struct {
	srv  *httptest.Server
	mu   sync.Mutex
	sent []map[string]any
	fire chan map[string]any
	tap  string // YES, NO, REPLY, or "" for no tap
	text string
}

func newAskHA(t *testing.T) *askHA {
	a := &askHA{fire: make(chan map[string]any, 4)}
	mux := http.NewServeMux()
	mux.HandleFunc("/api/websocket", func(w http.ResponseWriter, r *http.Request) {
		c, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		defer c.CloseNow()
		ctx := r.Context()
		wsjson.Write(ctx, c, map[string]any{"type": "auth_required"})
		var m map[string]any
		wsjson.Read(ctx, c, &m)
		wsjson.Write(ctx, c, map[string]any{"type": "auth_ok"})
		wsjson.Read(ctx, c, &m)
		wsjson.Write(ctx, c, map[string]any{"id": m["id"], "type": "result", "success": true})
		for {
			select {
			case d := <-a.fire:
				wsjson.Write(ctx, c, map[string]any{"id": m["id"], "type": "event", "event": map[string]any{"data": d}})
			case <-ctx.Done():
				return
			}
		}
	})
	mux.HandleFunc("/api/services/notify/", func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		a.mu.Lock()
		a.sent = append(a.sent, body)
		a.mu.Unlock()
		data, _ := body["data"].(map[string]any)
		actions, _ := data["actions"].([]any)
		if a.tap != "" {
			for _, x := range actions {
				act := x.(map[string]any)["action"].(string)
				if strings.HasSuffix(act, "_"+a.tap) {
					a.fire <- map[string]any{"action": act, "reply_text": a.text}
				}
			}
		}
		w.Write([]byte("[]"))
	})
	a.srv = httptest.NewServer(mux)
	t.Cleanup(a.srv.Close)
	return a
}

func askSetup(t *testing.T, a *askHA) (string, func(string) string) {
	dir := t.TempDir()
	p := filepath.Join(dir, "c.json")
	os.WriteFile(p, []byte(cfg), 0o600)
	env := map[string]string{"HA_URL": a.srv.URL, "HA_TOKEN": "sekrit"}
	return p, func(k string) string { return env[k] }
}

func (a *askHA) snapshot() []map[string]any {
	a.mu.Lock()
	defer a.mu.Unlock()
	return append([]map[string]any(nil), a.sent...)
}

func TestAskAnswers(t *testing.T) {
	for _, c := range []struct{ tap, text, want string }{
		{"YES", "", "yes\n"},
		{"NO", "", "no\n"},
		{"REPLY", "main, please", "main, please\n"},
	} {
		t.Run(c.tap, func(t *testing.T) {
			a := newAskHA(t)
			a.tap, a.text = c.tap, c.text
			p, env := askSetup(t, a)
			code, out, errOut := exec([]string{"ask", "finished", "--config", p, "--set", "task=Merge?", "--timeout", "5s"}, env)
			if code != 0 || out != c.want {
				t.Fatalf("code=%d out=%q err=%q", code, out, errOut)
			}
			sent := a.snapshot()
			if len(sent) != 2 {
				t.Fatalf("want question + clear, got %d messages", len(sent))
			}
			data := sent[0]["data"].(map[string]any)
			tag := data["tag"].(string)
			actions := data["actions"].([]any)
			if !strings.HasPrefix(tag, "ask-") || len(actions) != 3 {
				t.Fatalf("tag=%q actions=%v", tag, actions)
			}
			reply := actions[2].(map[string]any)
			if reply["behavior"] != "textInput" || !strings.HasSuffix(reply["action"].(string), "_REPLY") {
				t.Fatalf("reply action = %v", reply)
			}
			if sent[1]["message"] != "clear_notification" || sent[1]["data"].(map[string]any)["tag"] != tag {
				t.Fatalf("clear = %v", sent[1])
			}
		})
	}
}

func TestAskTimeoutExits3AndClears(t *testing.T) {
	a := newAskHA(t)
	p, env := askSetup(t, a)
	code, out, errOut := exec([]string{"ask", "finished", "--config", p, "--set", "task=x", "--timeout", "300ms"}, env)
	if code != 3 || out != "" || !strings.Contains(errOut, "no answer") {
		t.Fatalf("code=%d out=%q err=%q", code, out, errOut)
	}
	sent := a.snapshot()
	if len(sent) != 2 || sent[1]["message"] != "clear_notification" {
		t.Fatalf("sent = %v", sent)
	}
}

func TestAskMissingPlaceholderSendsNothing(t *testing.T) {
	a := newAskHA(t)
	p, env := askSetup(t, a)
	code, _, errOut := exec([]string{"ask", "finished", "--config", p}, env)
	if code != 1 || len(a.snapshot()) != 0 || !strings.Contains(errOut, "task") {
		t.Fatalf("code=%d err=%q", code, errOut)
	}
}

func TestAskTwoAsksUseDifferentIDs(t *testing.T) {
	a, b := newAskHA(t), newAskHA(t)
	a.tap, b.tap = "YES", "YES"
	pa, ea := askSetup(t, a)
	pb, eb := askSetup(t, b)
	exec([]string{"ask", "alpha", "--config", pa}, ea)
	exec([]string{"ask", "alpha", "--config", pb}, eb)
	ida := a.snapshot()[0]["data"].(map[string]any)["tag"]
	idb := b.snapshot()[0]["data"].(map[string]any)["tag"]
	if ida == idb {
		t.Fatalf("both asks used tag %v", ida)
	}
}
