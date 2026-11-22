package oc

import (
	"bytes"
	"encoding/binary"
	"image/color"
	"os"
	"path/filepath"
	"testing"
)

// A zero-length resources section is legal PSD but Cubism 5.3.05 fails at
// offset 0x26 expecting a resource signature. Keep a standard ResolutionInfo.
func TestPSDCubismResolutionResource(t *testing.T) {
	s := New(t.TempDir())
	a := importPSDTestImage(t, s, 1, 1, []color.NRGBA{{255, 0, 0, 255}}, "red")
	r, err := s.ExportPSD(PSDRequest{Width: 1, Height: 1, Layers: []PSDLayer{{AssetID: a.ID}}})
	if err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(s.Root, "exports", r.ID+".psd"))
	if err != nil {
		t.Fatal(err)
	}
	if binary.BigEndian.Uint32(b[30:34]) != 28 || string(b[34:38]) != "8BIM" || binary.BigEndian.Uint16(b[38:40]) != 1005 {
		t.Fatalf("missing Cubism-compatible ResolutionInfo: %x", b[30:42])
	}
	want := []byte{0, 72, 0, 0, 0, 1, 0, 1, 0, 72, 0, 0, 0, 1, 0, 1}
	if binary.BigEndian.Uint32(b[42:46]) != 16 || !bytes.Equal(b[46:62], want) {
		t.Fatal("invalid resolution payload")
	}
	parseTestPSD(t, b)
}
