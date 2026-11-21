// Package l2d inspects existing Cubism exports and discovers or launches Cubism Editor.
// It never claims to compile drawings to moc3 or author Cubism deformations.
package l2d

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type Inspection struct {
	ModelPath  string          `json:"modelPath"`
	Ready      bool            `json:"ready"`
	References []string        `json:"references"`
	Missing    []string        `json:"missing"`
	Parameters json.RawMessage `json:"parameters,omitempty"`
	Note       string          `json:"note"`
}

func Inspect(path string) (Inspection, error) {
	abs, e := filepath.Abs(path)
	if e != nil {
		return Inspection{}, e
	}
	if !strings.HasSuffix(strings.ToLower(abs), ".model3.json") {
		return Inspection{}, errors.New("select an existing Cubism .model3.json export")
	}
	f, e := os.Open(abs)
	if e != nil {
		return Inspection{}, e
	}
	defer f.Close()
	info, e := f.Stat()
	if e != nil || info.Size() > 4<<20 {
		return Inspection{}, errors.New("model configuration too large")
	}
	var model struct {
		Version        int
		FileReferences struct {
			Moc         string
			Textures    []string
			Physics     string
			Pose        string
			UserData    string
			DisplayInfo string
			Expressions []struct{ File string }
			Motions     map[string][]struct {
				File  string
				Sound string
			}
		}
		Groups json.RawMessage
	}
	if e = json.NewDecoder(f).Decode(&model); e != nil {
		return Inspection{}, errors.New("invalid model JSON")
	}
	if model.Version != 3 || model.FileReferences.Moc == "" || len(model.FileReferences.Textures) == 0 {
		return Inspection{}, errors.New("model3 must have Version 3, Moc and Textures")
	}
	refs := []string{model.FileReferences.Moc}
	refs = append(refs, model.FileReferences.Textures...)
	for _, p := range []string{model.FileReferences.Physics, model.FileReferences.Pose, model.FileReferences.UserData, model.FileReferences.DisplayInfo} {
		if p != "" {
			refs = append(refs, p)
		}
	}
	for _, v := range model.FileReferences.Expressions {
		refs = append(refs, v.File)
	}
	for _, vs := range model.FileReferences.Motions {
		for _, v := range vs {
			refs = append(refs, v.File)
			if v.Sound != "" {
				refs = append(refs, v.Sound)
			}
		}
	}
	out := Inspection{ModelPath: abs, Ready: true, References: refs, Missing: []string{}, Parameters: model.Groups, Note: "File integrity check only. Existing moc3 must be exported by Cubism; rig, gestures, visemes and physics require authoring. Groups are bindings, not a full parameter catalog."}
	root := filepath.Dir(abs)
	realRoot, e := filepath.EvalSymlinks(root)
	if e != nil {
		return Inspection{}, e
	}
	for _, r := range refs {
		clean := filepath.Clean(filepath.FromSlash(r))
		if filepath.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) || strings.Contains(clean, ":") {
			return Inspection{}, fmt.Errorf("unsafe model reference: %s", r)
		}
		target := filepath.Join(root, clean)
		real, e := filepath.EvalSymlinks(target)
		if e == nil {
			rel, err := filepath.Rel(realRoot, real)
			if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
				return Inspection{}, errors.New("model symlink escapes model directory")
			}
		}
		st, err := os.Stat(target)
		if e != nil || err != nil || st.IsDir() || st.Size() == 0 {
			out.Missing = append(out.Missing, r)
			out.Ready = false
		}
	}
	return out, nil
}
