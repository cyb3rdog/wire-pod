// Package fileutil provides small file-persistence helpers shared across
// wire-pod's various JSON state files (jdocs, config, custom intents, bot
// pairing info).
package fileutil

import (
	"os"
	"path/filepath"
)

// WriteFileAtomic writes data to path by writing it to a temp file in the
// same directory and renaming it into place, so a crash or power loss
// mid-write can't leave path holding truncated or corrupt data -- a plain
// os.WriteFile to the final path directly can, and wire-pod's persisted
// state (jdocs, bot pairing info, server config) is rewritten on nearly
// every robot interaction.
func WriteFileAtomic(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	if dir == "" {
		dir = "."
	}
	tmp, err := os.CreateTemp(dir, ".tmp-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	// Best-effort cleanup; a no-op once the rename below has succeeded.
	defer os.Remove(tmpPath)

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmpPath, perm); err != nil {
		return err
	}
	return os.Rename(tmpPath, path)
}
