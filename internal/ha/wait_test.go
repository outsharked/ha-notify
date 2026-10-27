package ha

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
)

// fakeHA speaks enough of HA's websocket protocol: auth, one subscribe, then
// forwards anything sent on fire as a mobile_app_notification_action event.
func fakeHA(t *testing.T, token string, fire chan map[string]any) *httptest.Server {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		defer c.CloseNow()
		ctx := r.Context()
		wsjson.Write(ctx, c, map[string]any{"type": "auth_required"})
		var auth map[string]any
		wsjson.Read(ctx, c, &auth)
		if auth["access_token"] != token {
			wsjson.Write(ctx, c, map[string]any{"type": "auth_invalid", "message": "Invalid password"})
			return
		}
		wsjson.Write(ctx, c, map[string]any{"type": "auth_ok"})
		var sub map[string]any
		wsjson.Read(ctx, c, &sub)
		if sub["type"] != "subscribe_events" || sub["event_type"] != "mobile_app_notification_action" {
			return
		}
		wsjson.Write(ctx, c, map[string]any{"id": sub["id"], "type": "result", "success": true})
		for {
			select {
			case data := <-fire:
				wsjson.Write(ctx, c, map[string]any{"id": sub["id"], "type": "event", "event": map[string]any{"event_type": "mobile_app_notification_action", "data": data}})
			case <-ctx.Done():
				return
			}
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestWaitForActionReturnsMatchingEvent(t *testing.T) {
	fire := make(chan map[string]any, 4)
	srv := fakeHA(t, "tok", fire)
	ready := func() error {
		fire <- map[string]any{"action": "OTHER_ASK_YES"}                          // someone else's: ignored
		fire <- map[string]any{"action": "ID1_REPLY", "reply_text": "main please"} // ours
		return nil
	}
	ev, err := New(srv.URL, "tok").WaitForAction(context.Background(), []string{"ID1_YES", "ID1_NO", "ID1_REPLY"}, ready)
	if err != nil {
		t.Fatal(err)
	}
	if ev.Action != "ID1_REPLY" || ev.ReplyText != "main please" {
		t.Fatalf("ev = %#v", ev)
	}
}

func TestWaitForActionSubscribesBeforeReady(t *testing.T) {
	fire := make(chan map[string]any, 1)
	srv := fakeHA(t, "tok", fire)
	var subscribed bool
	ready := func() error {
		// the fake only forwards events once subscribed; an event fired here must still arrive
		fire <- map[string]any{"action": "A_YES"}
		subscribed = true
		return nil
	}
	ev, err := New(srv.URL, "tok").WaitForAction(context.Background(), []string{"A_YES"}, ready)
	if err != nil || !subscribed || ev.Action != "A_YES" {
		t.Fatalf("ev=%#v err=%v", ev, err)
	}
}

func TestWaitForActionTimesOut(t *testing.T) {
	srv := fakeHA(t, "tok", make(chan map[string]any))
	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()
	_, err := New(srv.URL, "tok").WaitForAction(ctx, []string{"A_YES"}, func() error { return nil })
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("err = %v", err)
	}
}

func TestWaitForActionBadTokenNotLeaked(t *testing.T) {
	srv := fakeHA(t, "right", make(chan map[string]any))
	_, err := New(srv.URL, "wrongtoken").WaitForAction(context.Background(), []string{"A"}, func() error { return nil })
	if err == nil || strings.Contains(err.Error(), "wrongtoken") {
		t.Fatalf("err = %v", err)
	}
}

func TestWaitForActionReadyErrorPropagates(t *testing.T) {
	srv := fakeHA(t, "tok", make(chan map[string]any))
	want := errors.New("send failed")
	_, err := New(srv.URL, "tok").WaitForAction(context.Background(), []string{"A"}, func() error { return want })
	if !errors.Is(err, want) {
		t.Fatalf("err = %v", err)
	}
}

func TestClearSendsClearNotificationWithTag(t *testing.T) {
	var body map[string]any
	var path string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path = r.URL.Path
		json.NewDecoder(r.Body).Decode(&body)
	}))
	defer srv.Close()
	if err := New(srv.URL, "tok").Clear(context.Background(), "notify.mobile_app_x", "ask-1"); err != nil {
		t.Fatal(err)
	}
	if path != "/api/services/notify/mobile_app_x" || body["message"] != "clear_notification" || body["data"].(map[string]any)["tag"] != "ask-1" {
		t.Fatalf("path=%s body=%#v", path, body)
	}
}
