// Package config provides config loading utilities for the runtime package.
package config

import "context"

// Loader loads configuration from a directory into target.
type Loader interface {
	Load(ctx context.Context, dir string, target any) error
}
