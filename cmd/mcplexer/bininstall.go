package main

import (
	"fmt"
	"os"
	"path/filepath"
)

// stableBinPath returns the stable binary path: ~/.mcplexer/bin/mcplexer
func stableBinPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("get home dir: %w", err)
	}
	return filepath.Join(home, ".mcplexer", "bin", "mcplexer"), nil
}

// installBinary copies srcPath to the stable binary location, creating
// directories as needed. It is idempotent — safe to call repeatedly.
func installBinary(srcPath string) error {
	dst, err := stableBinPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return fmt.Errorf("create bin dir: %w", err)
	}
	src, err := os.ReadFile(srcPath)
	if err != nil {
		return fmt.Errorf("read binary: %w", err)
	}
	if err := os.WriteFile(dst, src, 0755); err != nil {
		return fmt.Errorf("write binary: %w", err)
	}
	return nil
}
