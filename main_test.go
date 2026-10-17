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
