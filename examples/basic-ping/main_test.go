package main

import (
	"github.com/callebjorkell/copper"
	"io"
	"net/http"
	"os"
	"testing"
)

func TestServer(t *testing.T) {
	go func() {
		StartServer()
	}()

	f, err := os.Open("spec.yaml")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	client, err := copper.WrapClient(http.DefaultClient, f)
	if err != nil {
		t.Fatal(err)
	}

	res, err := client.Get("http://localhost:8080/ping")
	if err != nil {
		t.Error(err)
	}

	body, err := io.ReadAll(res.Body)
	if err != nil {
		t.Error(err)
	}
	if string(body) != "pong" {
		t.Errorf("body wasn't pong")
	}

	// Verifying at the end checks that all paths, methods and responses are covered and that no extra paths have been hit.
	client.Verify(t)
}
