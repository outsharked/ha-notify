// Package config loads the ha-notify config file and the HA credentials.
package config

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type Template struct {
	Target  string         `json:"target"`
	Title   string         `json:"title"`
	Message string         `json:"message"`
	Data    map[string]any `json:"data"`
}

type Config struct {
	URL       string              `json:"url"`
	Default   Template            `json:"default"`
	Templates map[string]Template `json:"templates"`
	Token     string              `json:"-"`
}

// tokenVars are the accepted names for the HA token, in priority order.
var tokenVars = []string{"HA_TOKEN", "HA_ACCESS_TOKEN"}

// Load finds, reads and parses the config, then applies HA_URL and the token
// from the environment or a .env file beside the config. It returns the path used.
func Load(path string, getenv func(string) string) (*Config, string, error) {
	path, err := locate(path, getenv)
	if err != nil {
		return nil, "", err
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, "", err
	}
	var c Config
	if err := json.Unmarshal(raw, &c); err != nil {
		return nil, "", fmt.Errorf("%s: %w", path, err)
	}
	dotenv := readDotEnv(filepath.Join(filepath.Dir(path), ".env"))
	lookup := func(k string) string {
		if v := getenv(k); v != "" {
			return v
		}
		return dotenv[k]
	}
	if u := lookup("HA_URL"); u != "" {
		c.URL = u
	}
	for _, k := range tokenVars {
		if v := lookup(k); v != "" {
			c.Token = v
			break
		}
	}
	return &c, path, nil
}

func locate(explicit string, getenv func(string) string) (string, error) {
	if explicit != "" {
		return explicit, nil
	}
	if p := getenv("HA_NOTIFY_CONFIG"); p != "" {
		return p, nil
	}
	candidates := []string{"ha-notify.json"}
	if dir, err := os.UserConfigDir(); err == nil {
		candidates = append(candidates, filepath.Join(dir, "ha-notify", "config.json"))
	}
	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			return p, nil
		}
	}
	return "", fmt.Errorf("no config found (looked for %s); use --config or HA_NOTIFY_CONFIG", strings.Join(candidates, ", "))
}

func readDotEnv(path string) map[string]string {
	out := map[string]string{}
	f, err := os.Open(path)
	if err != nil {
		return out
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		v = strings.TrimSpace(v)
		if len(v) >= 2 && (v[0] == '"' || v[0] == '\'') && v[len(v)-1] == v[0] {
			v = v[1 : len(v)-1]
		}
		out[strings.TrimSpace(k)] = v
	}
	return out
}

// Names returns the template names, sorted.
func (c *Config) Names() []string {
	names := make([]string, 0, len(c.Templates))
	for n := range c.Templates {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

// Resolve returns the named template merged over the default.
func (c *Config) Resolve(name string) (Template, error) {
	t, ok := c.Templates[name]
	if !ok {
		return Template{}, fmt.Errorf("unknown template %q (available: %s)", name, strings.Join(c.Names(), ", "))
	}
	out := c.Default
	if t.Target != "" {
		out.Target = t.Target
	}
	if t.Title != "" {
		out.Title = t.Title
	}
	if t.Message != "" {
		out.Message = t.Message
	}
	out.Data = map[string]any{}
	for k, v := range c.Default.Data {
		out.Data[k] = v
	}
	for k, v := range t.Data {
		out.Data[k] = v
	}
	return out, nil
}
