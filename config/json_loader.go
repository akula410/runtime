package config

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// JSONLoader loads a JSON config file from the given directory.
type JSONLoader struct {
	filename string
}

// NewJSONLoader creates a JSONLoader that reads <dir>/<filename>.
func NewJSONLoader(filename string) *JSONLoader {
	return &JSONLoader{filename: filename}
}

// Load reads <dir>/<filename> and JSON-decodes it into target.
// If the file does not exist, Load returns nil (config is optional).
func (l *JSONLoader) Load(_ context.Context, dir string, target any) error {
	path := filepath.Join(dir, l.filename)
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("config: open %s: %w", path, err)
	}
	defer f.Close()
	if err := json.NewDecoder(f).Decode(target); err != nil {
		return fmt.Errorf("config: decode %s: %w", path, err)
	}
	return nil
}
