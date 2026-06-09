package config_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/akula410/runtime/config"
)

func TestJSONLoaderLoad(t *testing.T) {
	dir := t.TempDir()
	data := `{"host":"localhost","port":8080}`
	if err := os.WriteFile(filepath.Join(dir, "app.json"), []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}

	loader := config.NewJSONLoader("app.json")
	var cfg struct {
		Host string `json:"host"`
		Port int    `json:"port"`
	}
	if err := loader.Load(context.Background(), dir, &cfg); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Host != "localhost" {
		t.Errorf("host = %q, want %q", cfg.Host, "localhost")
	}
	if cfg.Port != 8080 {
		t.Errorf("port = %d, want %d", cfg.Port, 8080)
	}
}

func TestJSONLoaderMissingFile(t *testing.T) {
	loader := config.NewJSONLoader("app.json")
	var cfg struct{ Host string }
	if err := loader.Load(context.Background(), t.TempDir(), &cfg); err != nil {
		t.Fatalf("missing file should not error: %v", err)
	}
}

func TestJSONLoaderInvalidJSON(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "app.json"), []byte(`{bad`), 0o600); err != nil {
		t.Fatal(err)
	}
	loader := config.NewJSONLoader("app.json")
	var cfg struct{ Host string }
	if err := loader.Load(context.Background(), dir, &cfg); err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}
