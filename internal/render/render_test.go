package render

import (
	"reflect"
	"strings"
	"testing"
)

func TestFillString(t *testing.T) {
	got, err := Fill("Done: {task}", map[string]string{"task": "build"})
	if err != nil || got != "Done: build" {
		t.Fatalf("got %v, %v", got, err)
	}
}

func TestFillNested(t *testing.T) {
	in := map[string]any{
		"ttl":     float64(0),
		"actions": []any{map[string]any{"title": "Open {site}"}},
	}
	got, err := Fill(in, map[string]string{"site": "eBay"})
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]any{
		"ttl":     float64(0),
		"actions": []any{map[string]any{"title": "Open eBay"}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v", got)
	}
}

func TestFillEscapes(t *testing.T) {
	got, err := Fill("{{x}}", nil)
	if err != nil || got != "{x}" {
		t.Fatalf("got %v, %v", got, err)
	}
}

func TestFillNoRecursion(t *testing.T) {
	got, err := Fill("{task}", map[string]string{"task": "{session}", "session": "s"})
	if err != nil || got != "{session}" {
		t.Fatalf("got %v, %v", got, err)
	}
}

func TestFillUnknown(t *testing.T) {
	_, err := Fill("{missing}", nil)
	if err == nil || !strings.Contains(err.Error(), "missing") {
		t.Fatalf("err = %v", err)
	}
}

func TestFillUnicode(t *testing.T) {
	got, err := Fill("✅ {task}", map[string]string{"task": "café"})
	if err != nil || got != "✅ café" {
		t.Fatalf("got %v, %v", got, err)
	}
}
