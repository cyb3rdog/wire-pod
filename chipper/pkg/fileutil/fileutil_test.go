package fileutil

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWriteFileAtomicWritesContentAndPerm(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")

	if err := WriteFileAtomic(path, []byte(`{"a":1}`), 0644); err != nil {
		t.Fatalf("WriteFileAtomic: %v", err)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(got) != `{"a":1}` {
		t.Fatalf("got %q, want %q", got, `{"a":1}`)
	}

	// No leftover temp files.
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected exactly 1 file in dir, got %d", len(entries))
	}
}

func TestWriteFileAtomicOverwritesExisting(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")

	if err := os.WriteFile(path, []byte("old"), 0644); err != nil {
		t.Fatalf("seed WriteFile: %v", err)
	}
	if err := WriteFileAtomic(path, []byte("new"), 0644); err != nil {
		t.Fatalf("WriteFileAtomic: %v", err)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(got) != "new" {
		t.Fatalf("got %q, want %q", got, "new")
	}
}

// TestWriteFileAtomicPreservesSymlink reproduces docker/entrypoint.sh's
// persist_files layout: an "app tree" path that is a symlink into a
// separate "data volume" directory, mirroring apiConfig.json being
// symlinked from /opt/wire-pod/chipper into the bind-mounted /data. A
// naive os.Rename(tmp, path) replaces the symlink itself with a plain
// file on the first write -- this asserts that doesn't happen: the
// symlink must still exist and still point at the data-volume file
// afterward, and that file (not some detached copy) must hold the new
// content, exactly as required for the write to actually survive a
// container restart (which re-derives the app tree from the image and
// only keeps the data volume).
func TestWriteFileAtomicPreservesSymlink(t *testing.T) {
	appTree := t.TempDir()
	dataVolume := t.TempDir()

	dataPath := filepath.Join(dataVolume, "apiConfig.json")
	if err := os.WriteFile(dataPath, nil, 0644); err != nil {
		t.Fatalf("seed empty data-volume file: %v", err)
	}

	appPath := filepath.Join(appTree, "apiConfig.json")
	if err := os.Symlink(dataPath, appPath); err != nil {
		t.Fatalf("Symlink: %v", err)
	}

	if err := WriteFileAtomic(appPath, []byte(`{"a":1}`), 0644); err != nil {
		t.Fatalf("WriteFileAtomic: %v", err)
	}

	info, err := os.Lstat(appPath)
	if err != nil {
		t.Fatalf("Lstat: %v", err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("appPath is no longer a symlink after WriteFileAtomic -- it was replaced with a plain file, detaching it from the data volume")
	}

	dest, err := os.Readlink(appPath)
	if err != nil {
		t.Fatalf("Readlink: %v", err)
	}
	if dest != dataPath {
		t.Fatalf("symlink now points to %q, want %q", dest, dataPath)
	}

	got, err := os.ReadFile(dataPath)
	if err != nil {
		t.Fatalf("ReadFile(dataPath): %v", err)
	}
	if string(got) != `{"a":1}` {
		t.Fatalf("data-volume file has %q, want %q -- write did not land on the persistent copy", got, `{"a":1}`)
	}

	// A second write (the "settings changed again after restart" case)
	// must keep working the same way.
	if err := WriteFileAtomic(appPath, []byte(`{"a":2}`), 0644); err != nil {
		t.Fatalf("second WriteFileAtomic: %v", err)
	}
	got, err = os.ReadFile(dataPath)
	if err != nil {
		t.Fatalf("ReadFile(dataPath) after second write: %v", err)
	}
	if string(got) != `{"a":2}` {
		t.Fatalf("data-volume file has %q after second write, want %q", got, `{"a":2}`)
	}
}
