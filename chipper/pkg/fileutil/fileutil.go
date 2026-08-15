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
//
// path is resolved through any symlink first. rename(2) does not follow a
// symlink at its destination -- it unlinks the symlink itself and puts the
// new file directly in its place, silently detaching path from whatever it
// pointed at. That's exactly what docker/entrypoint.sh's persist_files sets
// up: apiConfig.json and friends are symlinked from the app tree into the
// bind-mounted data volume so the app can keep opening them by their usual
// relative path. Without resolving the symlink first, the very first
// atomic write here replaces that symlink with a real file living in the
// container's own ephemeral layer -- the bind-mounted copy is never
// touched again, and the next container start (whose entrypoint sees the
// bind-mounted file already "exists", even empty, and leaves it alone)
// re-links path back to that stale, empty file, discarding everything
// just written. Confirmed end-to-end with a real container run. Writing
// through the resolved target keeps the symlink intact and lands the
// write where it's actually meant to persist; when path isn't a symlink
// (the common case, e.g. in tests using a plain temp dir), EvalSymlinks
// is a no-op and behavior is unchanged.
func WriteFileAtomic(path string, data []byte, perm os.FileMode) error {
	target := path
	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		target = resolved
	}

	dir := filepath.Dir(target)
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
	return os.Rename(tmpPath, target)
}
