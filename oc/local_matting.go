package oc

import (
	"context"
	"errors"
	"image"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

var localMattingSlot = make(chan struct{}, 1)

// localMatte invokes only the provisioned local model, without network download.
func localMatte(input string, device ...string) (image.Image, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	select {
	case localMattingSlot <- struct{}{}:
		defer func() { <-localMattingSlot }()
	case <-ctx.Done():
		return nil, errors.New("本地 AI 去背景繁忙超时")
	}
	python, err := filepath.Abs(filepath.Join(".local", "matting-venv", "Scripts", "python.exe"))
	if err != nil {
		return nil, err
	}
	script, _ := filepath.Abs(filepath.Join("scripts", "remove-background.py"))
	models, _ := filepath.Abs(filepath.Join(".local", "matting-models"))
	if _, err = os.Stat(python); err != nil {
		return nil, errors.New("本地 AI 去背景环境未安装，请检查 .local/matting-venv")
	}
	f, err := os.CreateTemp(filepath.Dir(input), "matte-*.png")
	if err != nil {
		return nil, err
	}
	output := f.Name()
	f.Close()
	defer os.Remove(output)
	selected := "gpu"
	if len(device) > 0 && device[0] == "cpu" {
		selected = "cpu"
	}
	cmd := exec.CommandContext(ctx, python, script, "--input", input, "--output", output, "--model-dir", models, "--device", selected)
	if err = cmd.Run(); err != nil {
		return nil, errors.New("本地 AI 去背景失败：请检查本地模型和运行环境（不自动下载、不重复付费请求）")
	}
	data, err := os.ReadFile(output)
	if err != nil {
		return nil, err
	}
	if len(data) > MaxImageBytes {
		return nil, errors.New("本地去背景结果过大")
	}
	return decode(data)
}
