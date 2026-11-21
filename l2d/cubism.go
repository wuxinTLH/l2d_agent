package l2d

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// DiscoverCubism only checks bounded editor locations; never downloads or launches.
func DiscoverCubism() []string {
	roots := []string{}
	if cwd, err := os.Getwd(); err == nil {
		roots = append(roots, filepath.Join(cwd, "Live2DCubism"))
	}
	roots = append(roots, os.Getenv("ProgramFiles"), os.Getenv("ProgramFiles(x86)"), os.Getenv("LOCALAPPDATA"))
	return discoverCubismIn(roots)
}
func cubismEditorName(path string) bool {
	name := strings.ToLower(filepath.Base(path))
	return strings.HasPrefix(name, "cubismeditor") && strings.HasSuffix(name, ".exe") && !strings.Contains(name, "setup") && !strings.Contains(name, "install")
}
func discoverCubismIn(roots []string) []string {
	out := []string{}
	seen := map[string]bool{}
	for _, root := range roots {
		if root == "" {
			continue
		}
		for _, pattern := range []string{"CubismEditor*.exe", "Live2D Cubism*/CubismEditor*.exe", "Live2D Cubism*/app/CubismEditor*.exe", "Programs/Live2D Cubism*/CubismEditor*.exe"} {
			matches, _ := filepath.Glob(filepath.Join(root, filepath.FromSlash(pattern)))
			for _, path := range matches {
				abs, err := filepath.Abs(path)
				if err != nil || !cubismEditorName(abs) {
					continue
				}
				key := strings.ToLower(abs)
				st, err := os.Stat(abs)
				if err == nil && st.Mode().IsRegular() && !seen[key] {
					out = append(out, abs)
					seen[key] = true
				}
			}
		}
	}
	return out
}
func LaunchCubism(path string) error {
	if !filepath.IsAbs(path) || !cubismEditorName(path) {
		return errors.New("configure an absolute CubismEditor*.exe editor path, not an installer")
	}
	st, err := os.Stat(path)
	if err != nil || !st.Mode().IsRegular() {
		return errors.New("Cubism Editor executable not found")
	}
	cmd := exec.Command(path)
	cmd.Dir = filepath.Dir(path)
	if err = cmd.Start(); err != nil {
		return errors.New("Cubism Editor launch failed")
	}
	go cmd.Wait()
	return nil
}
