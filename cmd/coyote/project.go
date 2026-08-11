package main

import (
	"fmt"
	"os"
	"path/filepath"
)

func checkProject(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	hasGo, hasSettings := false, false
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".go" {
			continue
		}
		hasGo = true
		if entry.Name() == "settings.go" {
			hasSettings = true
		}
	}
	if !hasGo {
		return fmt.Errorf("no Go files in %s; run this from the directory holding your main package", dir)
	}
	if !hasSettings {
		fmt.Fprintf(os.Stderr, "coyote: warning: no settings.go in %s\n", dir)
	}
	return nil
}
