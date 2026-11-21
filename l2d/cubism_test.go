package l2d

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCubismDiscoveryAndLaunchValidation(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "Live2D Cubism 5.3")
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	exe := filepath.Join(dir, "CubismEditor5.exe")
	os.WriteFile(exe, []byte("not-executable"), 0600)
	os.WriteFile(filepath.Join(dir, "CubismEditorSetup.exe"), []byte("installer"), 0600)
	found := discoverCubismIn([]string{root, root})
	if len(found) != 1 || found[0] != exe {
		t.Fatalf("wrong candidates %v", found)
	}
	for _, p := range []string{"relative.exe", filepath.Join(root, "OtherEditor.exe"), filepath.Join(root, "CubismEditorSetup.exe"), filepath.Join(root, "CubismEditorMissing.exe")} {
		if LaunchCubism(p) == nil {
			t.Fatalf("accepted invalid editor %s", p)
		}
	}
}
