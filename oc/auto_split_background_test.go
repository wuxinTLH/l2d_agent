package oc

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"testing"
)

func TestRepairPartBackgroundPreservesEnclosedWhite(t *testing.T) {
	im := image.NewNRGBA(image.Rect(0, 0, 32, 32))
	draw.Draw(im, im.Bounds(), image.NewUniform(color.White), image.Point{}, draw.Src)
	draw.Draw(im, image.Rect(8, 8, 24, 24), image.NewUniform(color.Black), image.Point{}, draw.Src)
	draw.Draw(im, image.Rect(10, 10, 22, 22), image.NewUniform(color.White), image.Point{}, draw.Src)
	out, ok := repairPartBackground(im)
	if !ok {
		t.Fatal("expected repair")
	}
	if _, _, _, a := out.At(0, 0).RGBA(); a != 0 {
		t.Fatal("background opaque")
	}
	if _, _, _, a := out.At(15, 15).RGBA(); a == 0 {
		t.Fatal("enclosed white artwork erased")
	}
	if err := autoSplitContent(out); err != nil {
		t.Fatal(err)
	}
}
func TestRepairPartBackgroundCheckerboard(t *testing.T) {
	for _, tile := range []int{1, 7, 16} {
		t.Run(fmt.Sprintf("tile%d", tile), func(t *testing.T) {
			// Nonzero bounds also exercise flood indexing independent of origin.
			im := image.NewNRGBA(image.Rect(11, 17, 75, 81))
			for y := 17; y < 81; y++ {
				for x := 11; x < 75; x++ {
					v := uint8(252)
					if ((x-11)/tile+(y-17)/tile)%2 == 0 {
						v = 192
						if tile == 1 {
							v = 128 // Standard middle-gray checkerboards are neutral too.
						}
					}
					im.SetNRGBA(x, y, color.NRGBA{v, v, v, 255})
				}
			}
			// Slightly tinted neutral antialias blends between the two tiles.
			for y := 17; y < 81; y++ {
				im.SetNRGBA(18, y, color.NRGBA{221, 227, 232, 255})
			}
			draw.Draw(im, image.Rect(27, 33, 59, 65), image.NewUniform(color.Black), image.Point{}, draw.Src)
			draw.Draw(im, image.Rect(30, 36, 56, 62), image.NewUniform(color.White), image.Point{}, draw.Src)
			out, ok := repairPartBackground(im)
			if !ok {
				t.Fatal("checkerboard repair rejected")
			}
			for y := 17; y < 81; y++ {
				for x := 11; x < 75; x++ {
					_, _, _, a := out.At(x, y).RGBA()
					inside := image.Pt(x, y).In(image.Rect(27, 33, 59, 65))
					if (a != 0) != inside {
						t.Fatalf("alpha at (%d,%d) = %d; foreground = %v", x, y, a, inside)
					}
				}
			}
			if im.NRGBAAt(11, 17).A != 255 {
				t.Fatal("repair mutated original")
			}
			if err := autoSplitContent(out); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestRepairPartBackgroundNoisyNeutral(t *testing.T) {
	im := image.NewNRGBA(image.Rect(0, 0, 64, 64))
	for y := 0; y < 64; y++ {
		for x := 0; x < 64; x++ {
			v := uint8(235 + (x*13+y*7)%9)
			im.SetNRGBA(x, y, color.NRGBA{v - 7, v, v + 5, 255})
		}
	}
	draw.Draw(im, image.Rect(20, 20, 44, 44), image.NewUniform(color.NRGBA{180, 40, 70, 255}), image.Point{}, draw.Src)
	out, ok := repairPartBackground(im)
	if !ok {
		t.Fatal("noisy neutral repair rejected")
	}
	if _, _, _, a := out.At(10, 10).RGBA(); a != 0 {
		t.Fatal("noisy background retained")
	}
	if _, _, _, a := out.At(30, 30).RGBA(); a != 65535 {
		t.Fatal("colored foreground removed")
	}
}

func TestRepairPartBackgroundRejectsComplexPatterns(t *testing.T) {
	cases := []struct {
		name  string
		pixel func(x, y int) color.NRGBA
	}{
		{"colored checker", func(x, y int) color.NRGBA {
			if (x/4+y/4)%2 == 0 {
				return color.NRGBA{230, 180, 220, 255}
			}
			return color.NRGBA{255, 255, 255, 255}
		}},
		{"three neutrals", func(x, y int) color.NRGBA {
			v := []uint8{150, 202, 254}[(x/4+y/4)%3]
			return color.NRGBA{v, v, v, 255}
		}},
		{"neutral gradient", func(x, y int) color.NRGBA {
			v := uint8(144 + x*111/63)
			return color.NRGBA{v, v, v, 255}
		}},
		{"empty checker", func(x, y int) color.NRGBA {
			v := uint8(192)
			if (x/8+y/8)%2 == 0 {
				v = 255
			}
			return color.NRGBA{v, v, v, 255}
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			im := image.NewNRGBA(image.Rect(0, 0, 64, 64))
			for y := 0; y < 64; y++ {
				for x := 0; x < 64; x++ {
					im.SetNRGBA(x, y, tc.pixel(x, y))
				}
			}
			if tc.name != "empty checker" {
				draw.Draw(im, image.Rect(20, 20, 44, 44), image.NewUniform(color.Black), image.Point{}, draw.Src)
			}
			out, ok := repairPartBackground(im)
			if ok || out != im {
				t.Fatal("unsafe complex/empty image accepted")
			}
		})
	}
}

func TestRepairPartBackgroundRejectsBlankAndColor(t *testing.T) {
	for _, c := range []color.Color{color.White, color.NRGBA{20, 180, 80, 255}} {
		im := image.NewNRGBA(image.Rect(0, 0, 20, 20))
		draw.Draw(im, im.Bounds(), image.NewUniform(c), image.Point{}, draw.Src)
		if _, ok := repairPartBackground(im); ok {
			t.Fatal("unsafe blank/color accepted")
		}
	}
}
