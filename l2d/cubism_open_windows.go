//go:build windows

package l2d

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
)

// The official batch launcher forwards its first argument to the Java editor.
// The native EXE launcher may discard document arguments in some installations.
func openCubismDocument(editor, psd string) error {
	bat := strings.TrimSuffix(editor, filepath.Ext(editor)) + ".bat"
	st, err := os.Stat(bat)
	if err != nil || !st.Mode().IsRegular() {
		return errors.New("official Cubism Editor batch launcher missing; download the PSD and use File > Open in Cubism")
	}
	// cmd expands percent/environment variables even inside quotes. Reject rather
	// than interpreting user-controlled installation paths as command syntax.
	for _, p := range []string{bat, psd} {
		if strings.ContainsAny(p, "\"\r\n%!") {
			return errors.New("Cubism document path contains unsupported shell characters; use File > Open manually")
		}
	}
	cmd := exec.Command("cmd.exe")
	cmd.SysProcAttr = &syscall.SysProcAttr{CmdLine: `cmd.exe /d /s /c ""` + bat + `" "` + psd + `""`}
	cmd.Dir = filepath.Dir(editor)
	if err = cmd.Start(); err != nil {
		return errors.New("Cubism document launcher failed; use File > Open manually")
	}
	go cmd.Wait()
	return nil
}
