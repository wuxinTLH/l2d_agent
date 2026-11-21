package oc

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"unicode/utf16"
)

type parsedPSDLayer struct {
	bounds   [4]int32
	name     string
	channels [4][]byte
	section  uint32
	blend    string
}
type psdCursor struct {
	t   *testing.T
	b   []byte
	pos int
}

func (r *psdCursor) take(n int) []byte {
	r.t.Helper()
	if n < 0 || n > len(r.b)-r.pos {
		r.t.Fatalf("truncated PSD at %d need %d", r.pos, n)
	}
	b := r.b[r.pos : r.pos+n]
	r.pos += n
	return b
}
func (r *psdCursor) u16() uint16 { return binary.BigEndian.Uint16(r.take(2)) }
func (r *psdCursor) u32() uint32 { return binary.BigEndian.Uint32(r.take(4)) }
func parseTestPSD(t *testing.T, b []byte) ([]parsedPSDLayer, [4][]byte) {
	t.Helper()
	r := psdCursor{t: t, b: b}
	if string(r.take(4)) != "8BPS" || r.u16() != 1 {
		t.Fatal("not PSD v1")
	}
	if !bytes.Equal(r.take(6), make([]byte, 6)) || r.u16() != 4 {
		t.Fatal("header reserved/channels")
	}
	h, w := int(r.u32()), int(r.u32())
	if r.u16() != 8 || r.u16() != 3 {
		t.Fatal("not RGB8")
	}
	for i := 0; i < 2; i++ {
		r.take(int(r.u32()))
	}
	sectionEnd := int(r.u32()) + r.pos
	layerEnd := int(r.u32()) + r.pos
	count := int(int16(r.u16()))
	if count >= 0 {
		t.Fatal("merged alpha flag missing")
	}
	count = -count
	layers := make([]parsedPSDLayer, count)
	lengths := make([][4]int, count)
	for i := range layers {
		for j := range layers[i].bounds {
			layers[i].bounds[j] = int32(r.u32())
		}
		if r.u16() != 4 {
			t.Fatal("expected four channels")
		}
		for j, id := range []int16{0, 1, 2, -1} {
			if int16(r.u16()) != id {
				t.Fatal("channel ID")
			}
			lengths[i][j] = int(r.u32())
		}
		if string(r.take(4)) != "8BIM" {
			t.Fatal("blend signature")
		}
		layers[i].blend = string(r.take(4))
		if (layers[i].blend != "norm" && layers[i].blend != "pass") || !bytes.Equal(r.take(4), []byte{255, 0, 0, 0}) {
			t.Fatal("blend metadata")
		}
		end := int(r.u32()) + r.pos
		r.take(int(r.u32()))
		r.take(int(r.u32()))
		n := int(r.take(1)[0])
		r.take(n)
		r.take((4 - (n+1)%4) % 4)
		if string(r.take(8)) != "8BIMluni" {
			t.Fatal("Unicode name missing")
		}
		size := int(r.u32())
		n = int(r.u32())
		if size != 4+2*n {
			t.Fatal("Unicode length")
		}
		u := make([]uint16, n)
		for j := range u {
			u[j] = r.u16()
		}
		layers[i].name = string(utf16.Decode(u))
		if r.pos < end {
			if string(r.take(8)) != "8BIMlsct" || r.u32() != 12 {
				t.Fatal("section divider metadata")
			}
			layers[i].section = r.u32()
			if (layers[i].section != 1 && layers[i].section != 3) || string(r.take(4)) != "8BIM" || string(r.take(4)) != layers[i].blend {
				t.Fatal("section divider type/blend")
			}
		}
		if r.pos != end {
			t.Fatal("layer extra length")
		}
	}
	for i := range layers {
		for j := 0; j < 4; j++ {
			if r.u16() != 0 {
				t.Fatal("expected raw")
			}
			layers[i].channels[j] = append([]byte(nil), r.take(lengths[i][j]-2)...)
			bounds := layers[i].bounds
			if len(layers[i].channels[j]) != int((bounds[2]-bounds[0])*(bounds[3]-bounds[1])) {
				t.Fatal("channel bounds mismatch")
			}
		}
	}
	if r.pos%2 != layerEnd%2 {
		r.take(1)
	}
	if r.pos != layerEnd {
		t.Fatal("layer info length")
	}
	if r.u32() != 0 || r.pos != sectionEnd {
		t.Fatal("layer/mask section length")
	}
	if r.u16() != 0 {
		t.Fatal("merged compression")
	}
	var merged [4][]byte
	for i := range merged {
		merged[i] = append([]byte(nil), r.take(w*h)...)
	}
	if r.pos != len(b) {
		t.Fatal("unexpected trailing data")
	}
	return layers, merged
}
func importPSDTestImage(t *testing.T, s *Store, w, h int, pixels []color.NRGBA, name string) Asset {
	t.Helper()
	im := image.NewNRGBA(image.Rect(0, 0, w, h))
	for i, p := range pixels {
		im.SetNRGBA(i%w, i/w, p)
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, im); err != nil {
		t.Fatal(err)
	}
	a, err := s.Import(buf.Bytes(), name)
	if err != nil {
		t.Fatal(err)
	}
	return a
}
func TestPSDStructureUnicodeAlphaOffsetsAndMerge(t *testing.T) {
	s := New(t.TempDir())
	top := importPSDTestImage(t, s, 2, 1, []color.NRGBA{{255, 0, 0, 128}, {90, 80, 70, 0}}, "top")
	bottom := importPSDTestImage(t, s, 2, 1, []color.NRGBA{{0, 0, 255, 255}, {0, 255, 0, 255}}, "底层")
	req := PSDRequest{Width: 4, Height: 2, Layers: []PSDLayer{{AssetID: top.ID, Name: "眼睛👁", X: 1, Y: 1}, {AssetID: bottom.ID, X: 1, Y: 1}}}
	result, err := s.ExportPSD(req)
	if err != nil {
		t.Fatal(err)
	}
	if !validID.MatchString(result.ID) || result.URL != "/exports/"+result.ID+".psd" || result.Width != 4 || result.Height != 2 || result.LayerCount != 2 {
		t.Fatalf("result: %+v", result)
	}
	b, err := os.ReadFile(filepath.Join(s.Root, "exports", result.ID+".psd"))
	if err != nil {
		t.Fatal(err)
	}
	layers, merge := parseTestPSD(t, b)
	// parseTestPSD exposes wire order (bottom-to-top), not UI order.
	if layers[0].name != "底层" || layers[1].name != "眼睛👁" || layers[1].bounds != [4]int32{1, 1, 2, 3} {
		t.Fatalf("names/order/bounds: %+v", layers)
	}
	if !bytes.Equal(layers[1].channels[0], []byte{255, 90}) || !bytes.Equal(layers[1].channels[3], []byte{128, 0}) {
		t.Fatal("premultiplied or lost hidden color/alpha")
	}
	// Pixel (1,1): red-over-blue. Pixel (2,1): unchanged green. Others transparent.
	for i := 0; i < 8; i++ {
		expected := [4]byte{}
		if i == 5 {
			expected = [4]byte{128, 0, 127, 255}
		}
		if i == 6 {
			expected = [4]byte{0, 255, 0, 255}
		}
		for c := 0; c < 4; c++ {
			if merge[c][i] != expected[c] {
				t.Fatalf("merge[%d][%d]=%d want %d", c, i, merge[c][i], expected[c])
			}
		}
	}
	// Negative origin retains full layer bounds and clips only the merge.
	result, err = s.ExportPSD(PSDRequest{Width: 1, Height: 1, Layers: []PSDLayer{{AssetID: bottom.ID, X: -1}}})
	if err != nil {
		t.Fatal(err)
	}
	b, _ = os.ReadFile(filepath.Join(s.Root, "exports", result.ID+".psd"))
	layers, merge = parseTestPSD(t, b)
	if layers[0].bounds != [4]int32{0, -1, 1, 1} || merge[1][0] != 255 || merge[3][0] != 255 {
		t.Fatal("negative offset clipping")
	}
	// A single translucent layer must keep a straight-color merged image.
	result, err = s.ExportPSD(PSDRequest{Width: 2, Height: 1, Layers: []PSDLayer{{AssetID: top.ID}}})
	if err != nil {
		t.Fatal(err)
	}
	b, _ = os.ReadFile(filepath.Join(s.Root, "exports", result.ID+".psd"))
	_, merge = parseTestPSD(t, b)
	if merge[0][0] != 255 || merge[3][0] != 128 {
		t.Fatal("merged straight alpha")
	}
}

// TestPSDWireOrderBottomToTop pins the wire format independently of parseTestPSD.
// Adobe's spec defines layer index 0 as bottom (image resource 1024):
// https://www.adobe.com/devnet-apps/photoshop/fileformatashtml/
// Cross-check: psd-tools PSDImage._init appends records without reversing them,
// and GroupMixin indexes from the bottom, with -1 being the top:
// https://github.com/psd-tools/psd-tools/blob/main/src/psd_tools/api/psd_image.py
// https://psd-tools.readthedocs.io/en/stable/reference/psd_tools.api.layers.html
func TestPSDWireOrderBottomToTop(t *testing.T) {
	s := New(t.TempDir())
	top := importPSDTestImage(t, s, 1, 1, []color.NRGBA{{255, 0, 0, 255}}, "T")
	middle := importPSDTestImage(t, s, 2, 1, []color.NRGBA{{0, 255, 0, 255}, {0, 255, 0, 255}}, "M")
	bottom := importPSDTestImage(t, s, 3, 1, []color.NRGBA{{0, 0, 255, 255}, {0, 0, 255, 255}, {0, 0, 255, 255}}, "B")
	req := PSDRequest{Width: 3, Height: 1, Layers: []PSDLayer{{AssetID: top.ID}, {AssetID: middle.ID}, {AssetID: bottom.ID}}}
	original := append([]PSDLayer(nil), req.Layers...)
	result, err := s.ExportPSD(req)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(req.Layers, original) {
		t.Fatal("export mutated request layer order")
	}
	b, err := os.ReadFile(filepath.Join(s.Root, "exports", result.ID+".psd"))
	if err != nil {
		t.Fatal(err)
	}
	// Fixed PSD v1 layout: 26-byte header, two empty sections, two section
	// lengths, signed layer count; then three 88-byte records (one-letter names).
	const recordsStart, recordSize = 44, 88
	const channelsStart = recordsStart + 3*recordSize
	const channelsEnd = channelsStart + 20 + 16 + 12
	const mergedStart = channelsEnd + 4 + 2 // empty global mask, raw compression
	if len(b) != mergedStart+12 {
		t.Fatalf("wire length = %d, want %d", len(b), mergedStart+12)
	}
	for i, name := range []byte{'B', 'M', 'T'} {
		record := b[recordsStart+i*recordSize : recordsStart+(i+1)*recordSize]
		if !bytes.Equal(record[66:68], []byte{1, name}) || !bytes.Equal(record[86:88], []byte{0, name}) {
			t.Fatalf("wire record %d must be %c (bottom-to-top), Pascal=%x Unicode=%x", i, name, record[66:68], record[86:88])
		}
		if got := binary.BigEndian.Uint32(record[12:16]); got != uint32(3-i) {
			t.Fatalf("wire record %d right bound = %d, want %d", i, got, 3-i)
		}
	}
	// Literal raw channel blocks, not re-encoded with production helpers:
	// bottom blue (3 pixels), middle green (2), top red (1).
	wantChannels, err := hex.DecodeString(
		"0000000000" + "0000000000" + "0000ffffff" + "0000ffffff" +
			"00000000" + "0000ffff" + "00000000" + "0000ffff" +
			"0000ff" + "000000" + "000000" + "0000ff")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(b[channelsStart:channelsEnd], wantChannels) {
		t.Fatalf("channels must follow bottom-to-top records: got %x want %x", b[channelsStart:channelsEnd], wantChannels)
	}
	// The compatibility image must still show first-input-top: red, green, blue.
	wantMerged := []byte{255, 0, 0, 0, 255, 0, 0, 0, 255, 255, 255, 255}
	if !bytes.Equal(b[mergedStart:], wantMerged) {
		t.Fatalf("merged planes = %x, want %x", b[mergedStart:], wantMerged)
	}
}

func TestPSDGroupedRecordsPreserveStackChannelsAndMerge(t *testing.T) {
	s := New(t.TempDir())
	top := importPSDTestImage(t, s, 1, 1, []color.NRGBA{{255, 0, 0, 128}}, "top")
	bottom := importPSDTestImage(t, s, 1, 1, []color.NRGBA{{0, 0, 255, 255}}, "bottom")
	req := PSDRequest{Width: 1, Height: 1, Layers: []PSDLayer{
		{AssetID: top.ID, Group: "眼睛"},
		{AssetID: bottom.ID, Name: "iris", Group: "眼睛"},
		{AssetID: bottom.ID, Group: "身体"},
		{AssetID: bottom.ID, Name: "loose"},
		// A separated run must not be regrouped across intervening pixels.
		{AssetID: bottom.ID, Name: "last", Group: "眼睛"},
	}}
	original := append([]PSDLayer(nil), req.Layers...)
	result, err := s.ExportPSD(req)
	if err != nil {
		t.Fatal(err)
	}
	if result.LayerCount != 5 || !reflect.DeepEqual(original, req.Layers) {
		t.Fatal("group markers must not count as image layers or mutate request")
	}
	b, err := os.ReadFile(filepath.Join(s.Root, "exports", result.ID+".psd"))
	if err != nil {
		t.Fatal(err)
	}
	layers, merged := parseTestPSD(t, b)
	wantNames := []string{"</Layer group>", "last", "眼睛", "loose", "</Layer group>", "bottom", "身体", "</Layer group>", "iris", "top", "眼睛"}
	wantSections := []uint32{3, 0, 1, 0, 3, 0, 1, 3, 0, 0, 1}
	if len(layers) != len(wantNames) {
		t.Fatalf("record count = %d, want %d", len(layers), len(wantNames))
	}
	depth := 0
	for i, l := range layers {
		if l.name != wantNames[i] || l.section != wantSections[i] {
			t.Fatalf("record %d = %q section %d", i, l.name, l.section)
		}
		if l.section == 0 {
			if l.bounds != [4]int32{0, 0, 1, 1} || len(l.channels[0]) != 1 || l.blend != "norm" {
				t.Fatalf("invalid pixel record %d: %+v", i, l)
			}
			want := [4]byte{0, 0, 255, 255}
			if l.name == "top" {
				want = [4]byte{255, 0, 0, 128}
			}
			for c := range want {
				if l.channels[c][0] != want[c] {
					t.Fatalf("channel order changed for %q channel %d", l.name, c)
				}
			}
			continue
		}
		if l.bounds != [4]int32{} {
			t.Fatal("folder marker has nonempty bounds")
		}
		for _, channel := range l.channels {
			if len(channel) != 0 {
				t.Fatal("folder marker has pixels")
			}
		}
		if l.section == 3 {
			depth++
		} else {
			depth--
			if l.blend != "pass" {
				t.Fatal("folder must preserve pass-through compositing")
			}
		}
		if depth < 0 {
			t.Fatal("unbalanced folder markers")
		}
	}
	if depth != 0 {
		t.Fatal("unclosed folder")
	}
	for c, want := range []byte{128, 0, 127, 255} {
		if merged[c][0] != want {
			t.Fatalf("first input no longer topmost: merged channel %d = %d", c, merged[c][0])
		}
	}
}

func TestPSDGroupNamesAndJSONField(t *testing.T) {
	s := New(t.TempDir())
	a := importPSDTestImage(t, s, 1, 1, []color.NRGBA{{1, 2, 3, 255}}, "a")
	for _, group := range []string{"bad\x00name", string([]byte{0xff}), strings.Repeat("x", 4097)} {
		_, err := s.ExportPSD(PSDRequest{Width: 1, Height: 1, Layers: []PSDLayer{{AssetID: a.ID, Group: group}}})
		if err == nil {
			t.Fatalf("accepted invalid group %q", group)
		}
	}
	var layer PSDLayer
	if err := json.Unmarshal([]byte(`{"assetId":"id","group":"前发"}`), &layer); err != nil || layer.Group != "前发" {
		t.Fatalf("group JSON contract: %+v %v", layer, err)
	}
	b, err := json.Marshal(PSDLayer{AssetID: "id"})
	if err != nil || bytes.Contains(b, []byte(`"group"`)) {
		t.Fatalf("empty group should be optional: %s %v", b, err)
	}
}

func TestPSDManualSizePreservesGroupsOrderAndSource(t *testing.T) {
	s := New(t.TempDir())
	top := importPSDTestImage(t, s, 2, 1, []color.NRGBA{{255, 0, 0, 255}, {0, 255, 0, 0}}, "original")
	bottom := importPSDTestImage(t, s, 1, 1, []color.NRGBA{{0, 0, 255, 255}}, "bottom")
	path, _ := s.Path(top.ID)
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	req := PSDRequest{Width: 3, Height: 3, Layers: []PSDLayer{
		{AssetID: top.ID, Name: "缩放👁", Group: "眼睛", X: -1, Y: 1, Width: 3, Height: 2},
		{AssetID: bottom.ID, Group: "眼睛", Width: 3, Height: 3},
	}}
	original := append([]PSDLayer(nil), req.Layers...)
	result, err := s.ExportPSD(req)
	if err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(s.Root, "exports", result.ID+".psd"))
	if err != nil {
		t.Fatal(err)
	}
	layers, merged := parseTestPSD(t, b)
	if result.Width != 3 || result.Height != 3 || result.LayerCount != 2 || len(layers) != 4 {
		t.Fatalf("result/records: %+v %+v", result, layers)
	}
	if layers[0].section != 3 || layers[1].name != "bottom" || layers[2].name != "缩放👁" || layers[3].section != 1 || layers[3].name != "眼睛" {
		t.Fatalf("group/order/name: %+v", layers)
	}
	if layers[1].bounds != [4]int32{0, 0, 3, 3} || layers[2].bounds != [4]int32{1, -1, 3, 2} {
		t.Fatalf("manual bounds: %+v", layers)
	}
	// The middle sample blends opaque red with hidden green at zero alpha.
	// It must remain straight red at half alpha, not develop a green fringe.
	for c, want := range [][]byte{{255, 255, 0, 255, 255, 0}, {0, 0, 0, 0, 0, 0}, {0, 0, 0, 0, 0, 0}, {255, 128, 0, 255, 128, 0}} {
		if !bytes.Equal(layers[2].channels[c], want) {
			t.Fatalf("resized channel %d = %v want %v", c, layers[2].channels[c], want)
		}
	}
	for i := 0; i < 9; i++ {
		want := [4]byte{0, 0, 255, 255}
		if i == 3 || i == 6 { // negative X clips the first resized pixel
			want = [4]byte{128, 0, 127, 255}
		}
		for c := range want {
			if merged[c][i] != want[c] {
				t.Fatalf("resized merge[%d][%d] = %d want %d", c, i, merged[c][i], want[c])
			}
		}
	}
	after, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(before, after) || !reflect.DeepEqual(original, req.Layers) {
		t.Fatal("manual sizing mutated source PNG or request")
	}
	if a, err := s.Read(top.ID); err != nil || !reflect.DeepEqual(a, top) {
		t.Fatalf("manual sizing mutated asset metadata: %+v %v", a, err)
	}
}

func TestPSDManualSizeJSONAndValidation(t *testing.T) {
	var layer PSDLayer
	if err := json.Unmarshal([]byte(`{"assetId":"id","width":3,"height":2}`), &layer); err != nil || layer.Width != 3 || layer.Height != 2 {
		t.Fatalf("dimension JSON contract: %+v %v", layer, err)
	}
	b, err := json.Marshal(PSDLayer{AssetID: "id"})
	if err != nil || bytes.Contains(b, []byte(`"width"`)) || bytes.Contains(b, []byte(`"height"`)) {
		t.Fatalf("zero dimensions should be optional: %s %v", b, err)
	}
	s := New(t.TempDir())
	a := importPSDTestImage(t, s, 2, 1, []color.NRGBA{{255, 0, 0, 255}, {0, 255, 0, 0}}, "a")
	for _, dimensions := range [][2]int{{1, 0}, {0, 1}, {-1, 1}, {1, -1}, {-1, -1}, {8193, 1}, {1, 8193}, {8192, 8192}} {
		_, err := s.ExportPSD(PSDRequest{Width: 1, Height: 1, Layers: []PSDLayer{{AssetID: a.ID, Width: dimensions[0], Height: dimensions[1]}}})
		if err == nil || !strings.Contains(err.Error(), "output dimensions") {
			t.Fatalf("dimensions %v: %v", dimensions, err)
		}
	}
	// Each 40MP layer is valid; the additional original-size layer must put
	// their OUTPUT total over 80MP, even though the source assets are tiny.
	_, err = s.ExportPSD(PSDRequest{Width: 1, Height: 1, Layers: []PSDLayer{
		{AssetID: a.ID, Width: 8000, Height: 5000},
		{AssetID: a.ID, Width: 8000, Height: 5000},
		{AssetID: a.ID},
	}})
	if err == nil || !strings.Contains(err.Error(), "80 megapixels combined") {
		t.Fatalf("output pixel budget: %v", err)
	}
	// Omitting dimensions, explicit zeroes, and explicit original size all
	// retain the original straight samples, including hidden RGB.
	for _, dimensions := range [][2]int{{0, 0}, {2, 1}} {
		result, err := s.ExportPSD(PSDRequest{Width: 2, Height: 1, Layers: []PSDLayer{{AssetID: a.ID, Width: dimensions[0], Height: dimensions[1]}}})
		if err != nil {
			t.Fatal(err)
		}
		b, err := os.ReadFile(filepath.Join(s.Root, "exports", result.ID+".psd"))
		if err != nil {
			t.Fatal(err)
		}
		layers, _ := parseTestPSD(t, b)
		if layers[0].bounds != [4]int32{0, 0, 1, 2} || !bytes.Equal(layers[0].channels[1], []byte{0, 255}) {
			t.Fatalf("original pixels lost for %v: %+v", dimensions, layers)
		}
	}
}

func TestPSDManualDownsizeAndTransparentImage(t *testing.T) {
	for _, tc := range []struct {
		name   string
		pixels []color.NRGBA
		want   [4]byte
	}{
		{"alpha edge", []color.NRGBA{{255, 0, 0, 255}, {0, 255, 0, 0}}, [4]byte{255, 0, 0, 128}},
		{"transparent", []color.NRGBA{{255, 0, 0, 0}, {0, 255, 0, 0}}, [4]byte{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := New(t.TempDir())
			a := importPSDTestImage(t, s, 2, 1, tc.pixels, "a")
			result, err := s.ExportPSD(PSDRequest{Width: 1, Height: 1, Layers: []PSDLayer{{AssetID: a.ID, Width: 1, Height: 1}}})
			if err != nil {
				t.Fatal(err)
			}
			b, err := os.ReadFile(filepath.Join(s.Root, "exports", result.ID+".psd"))
			if err != nil {
				t.Fatal(err)
			}
			layers, merged := parseTestPSD(t, b)
			for c, want := range tc.want {
				if layers[0].channels[c][0] != want || merged[c][0] != want {
					t.Fatalf("channel %d layer=%d merge=%d want=%d", c, layers[0].channels[c][0], merged[c][0], want)
				}
			}
		})
	}
}

func TestPSDValidationAndFailureCleanup(t *testing.T) {
	s := New(t.TempDir())
	a := importPSDTestImage(t, s, 2, 1, []color.NRGBA{{1, 2, 3, 255}}, "a")
	good := PSDRequest{Width: 1, Height: 1, Layers: []PSDLayer{{AssetID: a.ID}}}
	cases := []PSDRequest{{Width: 0, Height: 1, Layers: good.Layers}, {Width: 8193, Height: 1, Layers: good.Layers}, {Width: 8192, Height: 8192, Layers: good.Layers}, {Width: 1, Height: 1}, {Width: 1, Height: 1, Layers: make([]PSDLayer, 65)}, {Width: 1, Height: 1, Layers: []PSDLayer{{AssetID: "../x"}}}, {Width: 1, Height: 1, Layers: []PSDLayer{{AssetID: ID()}}}, {Width: 1, Height: 1, Layers: []PSDLayer{{AssetID: a.ID, X: 8193}}}, {Width: 1, Height: 1, Layers: []PSDLayer{{AssetID: a.ID, Name: "bad\x00name"}}}}
	for i, req := range cases {
		if _, err := s.ExportPSD(req); err == nil {
			t.Fatalf("accepted case %d", i)
		}
	}
	p, _ := s.Path(a.ID)
	if err := os.WriteFile(p, []byte("invalid PNG"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := s.ExportPSD(good); err == nil {
		t.Fatal("accepted corrupt image")
	}
	entries, err := os.ReadDir(filepath.Join(s.Root, "exports"))
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatal("failed export left output")
	}
}
func TestPSDCombinedPixelBudget(t *testing.T) {
	s := New(t.TempDir())
	a := importPSDTestImage(t, s, 2048, 2048, nil, "large")
	req := PSDRequest{Width: 1, Height: 1}
	for i := 0; i < 20; i++ {
		req.Layers = append(req.Layers, PSDLayer{AssetID: a.ID})
	}
	if _, err := s.ExportPSD(req); err == nil {
		t.Fatal("accepted >80M combined layer pixels")
	}
	// Manual shrinking counts output pixels, not the aggregate original size.
	for i := range req.Layers {
		req.Layers[i].Width, req.Layers[i].Height = 1, 1
	}
	if _, err := s.ExportPSD(req); err != nil {
		t.Fatalf("small output rejected for large source total: %v", err)
	}
}
func TestAssetDeleteAndReadPreserveSiblings(t *testing.T) {
	s := New(t.TempDir())
	a, err := s.Import(fixture(t), "parent")
	if err != nil {
		t.Fatal(err)
	}
	split, err := s.Split(a.ID, []Part{{Name: "part", Width: 2, Height: 2}})
	if err != nil {
		t.Fatal(err)
	}
	sibling, err := s.Import(fixture(t), "sibling")
	if err != nil {
		t.Fatal(err)
	}
	if got, err := s.Read(a.ID); err != nil || !reflect.DeepEqual(a, got) {
		t.Fatalf("read: %+v %v", got, err)
	}
	manifest := filepath.Join(s.Root, split.ID+".manifest.json")
	original, err := os.ReadFile(manifest)
	if err != nil {
		t.Fatal(err)
	}
	if err = s.Delete(a.ID); err != nil {
		t.Fatal(err)
	}
	for _, ext := range []string{".png", ".json"} {
		if _, err = os.Stat(filepath.Join(s.Root, a.ID+ext)); !errors.Is(err, os.ErrNotExist) {
			t.Fatal("associated file not deleted")
		}
	}
	if err = s.Delete(a.ID); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("missing sentinel: %v", err)
	}
	for _, id := range []string{sibling.ID, split.Parts[0].ID} {
		if _, err = s.Read(id); err != nil {
			t.Fatal("deleted sibling", err)
		}
	}
	now, err := os.ReadFile(manifest)
	if err != nil || !bytes.Equal(original, now) {
		t.Fatal("manifest modified")
	}
	list, err := s.List()
	if err != nil || len(list) != 2 {
		t.Fatalf("phantom list: %+v %v", list, err)
	}
	for _, id := range []string{"../secret", "", a.ID + ".png", split.ID + ".manifest"} {
		if err = s.Delete(id); err == nil {
			t.Fatalf("unsafe ID %q", id)
		}
	}
}
func TestAssetDeletePreflightFailureLeavesAssetIntact(t *testing.T) {
	s := New(t.TempDir())
	a, err := s.Import(fixture(t), "a")
	if err != nil {
		t.Fatal(err)
	}
	p, _ := s.Path(a.ID)
	if err = os.Remove(p); err != nil {
		t.Fatal(err)
	}
	if err = s.Delete(a.ID); !errors.Is(err, os.ErrNotExist) {
		t.Fatal(err)
	}
	if _, err = os.Stat(filepath.Join(s.Root, a.ID+".json")); err != nil {
		t.Fatal("metadata deleted on preflight failure")
	}
	if err = os.Mkdir(p, 0700); err != nil {
		t.Fatal(err)
	}
	if err = s.Delete(a.ID); err == nil {
		t.Fatal("removed directory as image")
	}
	if _, err = os.Stat(filepath.Join(s.Root, a.ID+".json")); err != nil {
		t.Fatal("metadata lost")
	}
}
