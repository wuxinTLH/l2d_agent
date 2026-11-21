//go:build !windows

package l2d

func openCubismDocument(editor, psd string) error { return launchCubism(editor, []string{psd}) }
