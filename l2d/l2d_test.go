package l2d

import (
	"os"
	"path/filepath"
	"testing"
)

func TestInspect(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "test.model3.json")
	os.WriteFile(p, []byte(`{"Version":3,"FileReferences":{"Moc":"test.moc3","Textures":["texture.png"]}}`), 0600)
	out, e := Inspect(p)
	if e != nil || out.Ready || len(out.Missing) != 2 {
		t.Fatalf("%+v %v", out, e)
	}
	os.WriteFile(filepath.Join(dir, "test.moc3"), []byte("placeholder-not-compiled-model"), 0600)
	os.WriteFile(filepath.Join(dir, "texture.png"), []byte("placeholder"), 0600)
	out, e = Inspect(p)
	if e != nil || !out.Ready {
		t.Fatalf("%+v %v", out, e)
	}
	os.WriteFile(p, []byte(`{"Version":3,"FileReferences":{"Moc":"../outside.moc3","Textures":["texture.png"]}}`), 0600)
	if _, e = Inspect(p); e == nil {
		t.Fatal("accepted traversal")
	}
	if _, e = Inspect(filepath.Join(dir, "fake.png")); e == nil {
		t.Fatal("accepted non-model")
	}
}
