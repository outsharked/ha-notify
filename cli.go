package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/outsharked/ha-notify/internal/config"
	"github.com/outsharked/ha-notify/internal/ha"
	"github.com/outsharked/ha-notify/internal/render"
)

const usage = `usage:
  ha-notify send <template> [--set key=value]... [--title T] [--message M] [--config PATH]
  ha-notify ask <template> [--set key=value]... [--title T] [--message M] [--timeout 10m] [--config PATH]
  ha-notify list [--config PATH]
  ha-notify check [--config PATH]
  ha-notify version`

type setFlags map[string]string

func (s setFlags) String() string { return "" }
func (s setFlags) Set(v string) error {
	k, val, ok := strings.Cut(v, "=")
	if !ok || k == "" {
		return fmt.Errorf("--set wants key=value, got %q", v)
	}
	s[k] = val
	return nil
}

type usageError struct{ error }

// clearDelay lets the companion app finish showing a reply before the notification is cleared;
// cleared sooner, Android re-posts it with the reply bubble still showing.
var clearDelay = 2 * time.Second

// timeoutError means an ask got no answer in time (exit status 3).
type timeoutError struct{ error }

func run(args []string, getenv func(string) string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, usage)
		return 2
	}
	var err error
	switch args[0] {
	case "send":
		err = cmdSend(args[1:], getenv)
	case "version":
		fmt.Fprintln(stdout, "ha-notify", version)
	case "ask":
		err = cmdAsk(args[1:], getenv, stdout)
	case "list":
		err = cmdList(args[1:], getenv, stdout)
	case "check":
		err = cmdCheck(args[1:], getenv, stdout)
	default:
		err = usageError{fmt.Errorf("unknown command %q", args[0])}
	}
	var ue usageError
	var te timeoutError
	switch {
	case err == nil:
		return 0
	case errors.As(err, &te):
		fmt.Fprintf(stderr, "ha-notify: %v\n", te.error)
		return 3
	case errors.As(err, &ue):
		fmt.Fprintf(stderr, "%v\n%s\n", ue.error, usage)
		return 2
	default:
		fmt.Fprintf(stderr, "ha-notify: %v\n", err)
		return 1
	}
}

// parse accepts flags before or after positional arguments.
func parse(fs *flag.FlagSet, args []string) ([]string, error) {
	var pos []string
	for {
		if err := fs.Parse(args); err != nil {
			return nil, usageError{err}
		}
		if fs.NArg() == 0 {
			return pos, nil
		}
		pos = append(pos, fs.Arg(0))
		args = fs.Args()[1:]
	}
}

func newFlags(name string) (*flag.FlagSet, *string) {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	path := fs.String("config", "", "config file path")
	return fs, path
}

// outgoing is a fully resolved, placeholder-filled notification.
type outgoing struct {
	c                      *config.Config
	target, title, message string
	data                   map[string]any
}

// sendFlags are the flags shared by send and ask.
type sendFlags struct {
	fs                   *flag.FlagSet
	path, title, message *string
	sets                 setFlags
}

func newSendFlags(name string) *sendFlags {
	fs, path := newFlags(name)
	f := &sendFlags{fs: fs, path: path, sets: setFlags{}}
	fs.Var(f.sets, "set", "placeholder value, key=value (repeatable)")
	f.title = fs.String("title", "", "override the template title")
	f.message = fs.String("message", "", "override the template message")
	return f
}

// build loads the config and fills the named template.
func (f *sendFlags) build(args []string, getenv func(string) string) (*outgoing, error) {
	pos, err := parse(f.fs, args)
	if err != nil {
		return nil, err
	}
	if len(pos) != 1 {
		return nil, usageError{fmt.Errorf("%s wants exactly one template name", f.fs.Name())}
	}
	c, _, err := config.Load(*f.path, getenv)
	if err != nil {
		return nil, err
	}
	tpl, err := c.Resolve(pos[0])
	if err != nil {
		return nil, err
	}
	if *f.title != "" {
		tpl.Title = literal(*f.title)
	}
	if *f.message != "" {
		tpl.Message = literal(*f.message)
	}
	vars := map[string]string{}
	if wd, err := os.Getwd(); err == nil {
		vars["session"] = filepath.Base(wd)
	}
	for k, v := range f.sets {
		vars[k] = v
	}
	filled, err := render.Fill(map[string]any{"title": tpl.Title, "message": tpl.Message, "data": tpl.Data}, vars)
	if err != nil {
		return nil, err
	}
	m := filled.(map[string]any)
	data, _ := m["data"].(map[string]any)
	if data == nil {
		data = map[string]any{}
	}
	if err := requireCreds(c); err != nil {
		return nil, err
	}
	return &outgoing{c: c, target: tpl.Target, title: m["title"].(string), message: m["message"].(string), data: data}, nil
}

func cmdSend(args []string, getenv func(string) string) error {
	o, err := newSendFlags("send").build(args, getenv)
	if err != nil {
		return err
	}
	return ha.New(o.c.URL, o.c.Token).Notify(context.Background(), o.target, o.title, o.message, o.data)
}

// cmdAsk sends a Yes / No / Reply notification and prints the answer.
func cmdAsk(args []string, getenv func(string) string, stdout io.Writer) error {
	f := newSendFlags("ask")
	timeout := f.fs.Duration("timeout", 10*time.Minute, "how long to wait for an answer")
	o, err := f.build(args, getenv)
	if err != nil {
		return err
	}
	var b [4]byte
	if _, err := rand.Read(b[:]); err != nil {
		return err
	}
	id := hex.EncodeToString(b[:])
	prefix := "HA_NOTIFY_" + id
	yes, no, reply := prefix+"_YES", prefix+"_NO", prefix+"_REPLY"
	tag := "ask-" + id
	o.data["tag"] = tag
	o.data["actions"] = []any{
		map[string]any{"action": yes, "title": "Yes"},
		map[string]any{"action": no, "title": "No"},
		map[string]any{"action": reply, "title": "Reply", "behavior": "textInput"},
	}

	client := ha.New(o.c.URL, o.c.Token)
	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()
	ev, err := client.WaitForAction(ctx, []string{yes, no, reply}, func() error {
		return client.Notify(ctx, o.target, o.title, o.message, o.data)
	})
	clear := func() {
		time.Sleep(clearDelay)
		cctx, ccancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer ccancel()
		_ = client.Clear(cctx, o.target, tag) // best effort
	}
	if errors.Is(err, context.DeadlineExceeded) {
		clear()
		return timeoutError{fmt.Errorf("no answer within %s", *timeout)}
	}
	if err != nil {
		return err
	}
	clear()
	switch ev.Action {
	case yes:
		fmt.Fprintln(stdout, "yes")
	case no:
		fmt.Fprintln(stdout, "no")
	default:
		fmt.Fprintln(stdout, ev.ReplyText)
	}
	return nil
}

// literal escapes braces so override text is sent exactly as typed.
func literal(s string) string {
	return strings.NewReplacer("{", "{{", "}", "}}").Replace(s)
}

func requireCreds(c *config.Config) error {
	if c.URL == "" {
		return errors.New("no Home Assistant URL: set \"url\" in the config or HA_URL")
	}
	if c.Token == "" {
		return errors.New("no Home Assistant token: set HA_TOKEN in the environment or a .env file beside the config")
	}
	return nil
}

func cmdList(args []string, getenv func(string) string, stdout io.Writer) error {
	fs, path := newFlags("list")
	if _, err := parse(fs, args); err != nil {
		return err
	}
	c, _, err := config.Load(*path, getenv)
	if err != nil {
		return err
	}
	for _, n := range c.Names() {
		fmt.Fprintln(stdout, n)
	}
	return nil
}

func cmdCheck(args []string, getenv func(string) string, stdout io.Writer) error {
	fs, path := newFlags("check")
	if _, err := parse(fs, args); err != nil {
		return err
	}
	c, used, err := config.Load(*path, getenv)
	if err != nil {
		return err
	}
	for _, n := range c.Names() {
		tpl, _ := c.Resolve(n)
		if !strings.HasPrefix(tpl.Target, "notify.") {
			return fmt.Errorf("template %q: target %q must look like notify.<service>", n, tpl.Target)
		}
	}
	if err := requireCreds(c); err != nil {
		return err
	}
	if err := ha.New(c.URL, c.Token).Ping(context.Background()); err != nil {
		return err
	}
	fmt.Fprintf(stdout, "ok: %s, %d templates, %s reachable\n", used, len(c.Templates), c.URL)
	return nil
}
