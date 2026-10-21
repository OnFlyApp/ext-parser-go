package main

import "testing"

func TestParseMethodReceiver(t *testing.T) {
	src := []byte("package demo\n\ntype Service struct{}\n\nfunc (s *Service) Execute() {}\n")
	chunks, err := parse("src/svc.go", src)
	if err != nil {
		t.Fatal(err)
	}
	var method Chunk
	found := false
	for _, chunk := range chunks {
		if chunk.Type == "method" && chunk.Metadata["name"] == "Execute" {
			method = chunk
			found = true
		}
	}
	if !found {
		t.Fatalf("%+v", chunks)
	}
	if method.Metadata["receiver"] != "Service" || method.FilePath != "src/svc.go" {
		t.Fatalf("%+v", method)
	}
}

func TestParamTypesAndHandlers(t *testing.T) {
	src := []byte(`package bus

import (
	"context"
	cmds "github.com/acme/app/commands/v2"
)

type Service struct{}

func (s *Service) Handle(ctx context.Context, cmd *cmds.ClawBack, retries int) error { return nil }
func (s *Service) OnRefunded(e RefundedEvent) {}
func (s *Service) Render(name string, v View, m map[string]Other) {}
func Execute() {}
`)
	chunks, err := parse("bus.go", src)
	if err != nil {
		t.Fatal(err)
	}
	by := map[string]map[string]string{}
	for _, c := range chunks {
		by[c.Metadata["name"]] = c.Metadata
	}
	const claw = "github.com/acme/app/commands/v2.ClawBack"
	for _, tc := range []struct{ name, types, handles string }{
		{"Handle", "context.Context," + claw, claw},
		{"OnRefunded", "RefundedEvent", "RefundedEvent"},
		{"Render", "View", ""}, // not a handler name; a map gives no type
		{"Execute", "", ""},
	} {
		m := by[tc.name]
		if m["param_types"] != tc.types || m["handles"] != tc.handles || m["package"] != "bus" {
			t.Errorf("%s: %v, want param_types=%q handles=%q", tc.name, m, tc.types, tc.handles)
		}
	}
}
