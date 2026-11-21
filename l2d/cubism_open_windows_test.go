//go:build windows

package l2d

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCubismOpenRequiresOfficialBatch(t *testing.T) {
	root := t.TempDir()
	editor := filepath.Join(root, "CubismEditor5.exe")
	os.WriteFile(editor, []byte("fixture"), 0600)
	psd := filepath.Join(root, "a.psd")
	os.WriteFile(psd, []byte{'8', 'B', 'P', 'S', 0, 1}, 0600)
	err := OpenCubismPSD(editor, psd)
	if err == nil || !strings.Contains(err.Error(), "batch launcher missing") {
		t.Fatalf("expected actionable batch error, got %v", err)
	}
	bat := filepath.Join(root, "CubismEditor5.bat")
	os.WriteFile(bat, []byte("@exit /b 0"), 0600)
	unsafe := filepath.Join(root, "a%TEMP%.psd")
	os.WriteFile(unsafe, []byte{'8', 'B', 'P', 'S', 0, 1}, 0600)
	if err := OpenCubismPSD(editor, unsafe); err == nil || !strings.Contains(err.Error(), "shell characters") {
		t.Fatalf("unsafe path accepted: %v", err)
	}
}
