package test

import (
	"net/http"
	"os"
	"testing"
)

var baseURL string

func TestMain(m *testing.M) {
	baseURL = "http://localhost:1323"
	code := m.Run()

	os.Exit(code)
}

func TestAPIHealth(t *testing.T) {
	resp, err := http.Get(baseURL + "/api/health")
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}
