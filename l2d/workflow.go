package l2d

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Probe verifies the documented registration echo, never model state.
func Probe(ctx context.Context, endpoint string) error {
	message, _ := json.Marshal(map[string]any{"msg": 10000, "msgId": time.Now().UnixMilli()})
	_, err := Command(ctx, endpoint, message)
	return err
}

// Discover searches only common installation directories, not entire disks.
// It returns suggestions and never changes configuration or launches a program.
func Discover() []string {
	roots := []string{os.Getenv("ProgramFiles"), os.Getenv("ProgramFiles(x86)"), os.Getenv("LOCALAPPDATA")}
	if home, err := os.UserHomeDir(); err == nil {
		roots = append(roots, filepath.Join(home, "Desktop"), filepath.Join(home, "Downloads"))
	}
	return discoverIn(roots)
}
func discoverIn(roots []string) []string {
	candidates := []string{}
	seen := map[string]bool{}
	for _, root := range roots {
		if root == "" {
			continue
		}
		patterns := []string{
			filepath.Join(root, "Live2DViewerEX", "Live2DViewerEX.exe"),
			filepath.Join(root, "Live2DViewerEX*", "Live2DViewerEX.exe"),
			filepath.Join(root, "Steam", "steamapps", "common", "Live2DViewerEX", "Live2DViewerEX.exe"),
			filepath.Join(root, "Programs", "Live2DViewerEX", "Live2DViewerEX.exe"),
		}
		for _, pattern := range patterns {
			matches, _ := filepath.Glob(pattern)
			for _, path := range matches {
				abs, err := filepath.Abs(path)
				if err != nil {
					continue
				}
				key := strings.ToLower(abs)
				if st, err := os.Stat(abs); err == nil && st.Mode().IsRegular() && !seen[key] {
					candidates = append(candidates, abs)
					seen[key] = true
				}
			}
		}
	}
	return candidates
}
