package config

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func noEnv(string) string { return "" }

func envOf(m map[string]string) func(string) string {
	return func(k string) string { return m[k] }
}

func write(t *testing.T, dir, name, body string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

const sample = `{
  "url": "http://ha.local:8123",
  "default": {"target": "notify.mobile_app_x", "data": {"channel": "Claude", "ttl": 0}},
  "templates": {
    "finished": {"title": "Done", "message": "{task}", "data": {"notification_icon": "mdi:check", "channel": "Other"}},
    "alpha": {"title": "A"}
  }
}`

func TestResolveMergesData(t *testing.T) {
	p := write(t, t.TempDir(), "c.json", sample)
	c, _, err := Load(p, noEnv)
	if err != nil {
		t.Fatal(err)
	}
	tpl, err := c.Resolve("finished")
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]any{"channel": "Other", "ttl": float64(0), "notification_icon": "mdi:check"}
	if !reflect.DeepEqual(tpl.Data, want) {
		t.Fatalf("data = %#v", tpl.Data)
	}
	if tpl.Target != "notify.mobile_app_x" || tpl.Title != "Done" || tpl.Message != "{task}" {
		t.Fatalf("tpl = %#v", tpl)
	}
}

func TestResolveUnknownListsNames(t *testing.T) {
	p := write(t, t.TempDir(), "c.json", sample)
	c, _, _ := Load(p, noEnv)
	_, err := c.Resolve("nope")
	if err == nil || !strings.Contains(err.Error(), "alpha, finished") {
		t.Fatalf("err = %v", err)
	}
	if got := c.Names(); !reflect.DeepEqual(got, []string{"alpha", "finished"}) {
		t.Fatalf("names = %v", got)
	}
}

func TestEnvOverridesURL(t *testing.T) {
	p := write(t, t.TempDir(), "c.json", sample)
	c, _, _ := Load(p, envOf(map[string]string{"HA_URL": "http://other:1"}))
	if c.URL != "http://other:1" {
		t.Fatalf("url = %q", c.URL)
	}
}

func TestDotEnvLoadsToken(t *testing.T) {
	dir := t.TempDir()
	p := write(t, dir, "c.json", sample)
	write(t, dir, ".env", "# comment\nHA_TOKEN=\"abc123\"\n")
	c, _, _ := Load(p, noEnv)
	if c.Token != "abc123" {
		t.Fatalf("token = %q", c.Token)
	}
}

func TestDotEnvAcceptsAccessTokenAlias(t *testing.T) {
	dir := t.TempDir()
	p := write(t, dir, "c.json", sample)
	write(t, dir, ".env", "HA_ACCESS_TOKEN=zzz\n")
	c, _, _ := Load(p, noEnv)
	if c.Token != "zzz" {
		t.Fatalf("token = %q", c.Token)
	}
}

func TestRealEnvBeatsDotEnv(t *testing.T) {
	dir := t.TempDir()
	p := write(t, dir, "c.json", sample)
	write(t, dir, ".env", "HA_TOKEN=fromfile\n")
	c, _, _ := Load(p, envOf(map[string]string{"HA_TOKEN": "fromenv"}))
	if c.Token != "fromenv" {
		t.Fatalf("token = %q", c.Token)
	}
}

func TestLookupOrder(t *testing.T) {
	dir := t.TempDir()
	explicit := write(t, dir, "explicit.json", sample)
	viaEnv := write(t, dir, "viaenv.json", `{"url":"http://env"}`)
	_, used, err := Load(explicit, envOf(map[string]string{"HA_NOTIFY_CONFIG": viaEnv}))
	if err != nil || used != explicit {
		t.Fatalf("used = %q, err = %v", used, err)
	}
	_, used, err = Load("", envOf(map[string]string{"HA_NOTIFY_CONFIG": viaEnv}))
	if err != nil || used != viaEnv {
		t.Fatalf("used = %q, err = %v", used, err)
	}
}

func TestInvalidJSONNamesFile(t *testing.T) {
	p := write(t, t.TempDir(), "bad.json", "{nope")
	_, _, err := Load(p, noEnv)
	if err == nil || !strings.Contains(err.Error(), "bad.json") {
		t.Fatalf("err = %v", err)
	}
}

func TestMissingConfigNamesSearchedPaths(t *testing.T) {
	// isolate from the developer's real config dir and working directory
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("HOME", t.TempDir())
	t.Chdir(t.TempDir())
	_, _, err := Load("", envOf(map[string]string{"HA_NOTIFY_CONFIG": ""}))
	if err == nil || !strings.Contains(err.Error(), "ha-notify.json") {
		t.Fatalf("err = %v", err)
	}
}
