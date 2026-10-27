package ha

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
)

// ActionEvent is a tap on a notification action button.
type ActionEvent struct {
	Action    string
	ReplyText string
}

// Clear removes a notification previously sent with the given tag.
func (c *Client) Clear(ctx context.Context, target, tag string) error {
	return c.Notify(ctx, target, "", "clear_notification", map[string]any{"tag": tag})
}

// WaitForAction subscribes to notification-action events, calls ready (which
// should send the notification, so no tap is missed), then blocks until an
// event whose action is one of actions arrives or ctx ends.
func (c *Client) WaitForAction(ctx context.Context, actions []string, ready func() error) (ActionEvent, error) {
	conn, _, err := websocket.Dial(ctx, c.wsURL(), nil)
	if err != nil {
		return ActionEvent{}, fmt.Errorf("cannot open Home Assistant websocket at %s: %s", c.BaseURL, redact(err.Error(), c.Token))
	}
	defer conn.CloseNow()

	var msg struct {
		Type    string `json:"type"`
		Message string `json:"message"`
		Success bool   `json:"success"`
		Event   struct {
			Data struct {
				Action    string `json:"action"`
				ReplyText string `json:"reply_text"`
			} `json:"data"`
		} `json:"event"`
	}
	read := func() error {
		msg.Type, msg.Message, msg.Success = "", "", false
		msg.Event.Data.Action, msg.Event.Data.ReplyText = "", ""
		return wsjson.Read(ctx, conn, &msg)
	}

	if err := read(); err != nil || msg.Type != "auth_required" {
		return ActionEvent{}, fmt.Errorf("unexpected websocket greeting from Home Assistant")
	}
	if err := wsjson.Write(ctx, conn, map[string]any{"type": "auth", "access_token": c.Token}); err != nil {
		return ActionEvent{}, err
	}
	if err := read(); err != nil {
		return ActionEvent{}, err
	}
	if msg.Type != "auth_ok" {
		return ActionEvent{}, fmt.Errorf("Home Assistant websocket rejected the token: %s", redact(msg.Message, c.Token))
	}
	if err := wsjson.Write(ctx, conn, map[string]any{"id": 1, "type": "subscribe_events", "event_type": "mobile_app_notification_action"}); err != nil {
		return ActionEvent{}, err
	}
	if err := read(); err != nil {
		return ActionEvent{}, err
	}
	if msg.Type != "result" || !msg.Success {
		return ActionEvent{}, errors.New("Home Assistant refused the event subscription")
	}

	if err := ready(); err != nil {
		return ActionEvent{}, err
	}
	for {
		if err := read(); err != nil {
			if ctx.Err() != nil {
				return ActionEvent{}, ctx.Err()
			}
			return ActionEvent{}, err
		}
		if msg.Type == "event" && slices.Contains(actions, msg.Event.Data.Action) {
			return ActionEvent{Action: msg.Event.Data.Action, ReplyText: msg.Event.Data.ReplyText}, nil
		}
	}
}

func (c *Client) wsURL() string {
	u := c.BaseURL
	switch {
	case strings.HasPrefix(u, "https://"):
		u = "wss://" + strings.TrimPrefix(u, "https://")
	case strings.HasPrefix(u, "http://"):
		u = "ws://" + strings.TrimPrefix(u, "http://")
	}
	return u + "/api/websocket"
}
