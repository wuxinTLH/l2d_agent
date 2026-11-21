package oc

import (
	"image"
	"image/color"
	"math"
)

func autoSplitRect(r image.Rectangle) AutoSplitBounds {
	return AutoSplitBounds{X: r.Min.X, Y: r.Min.Y, Width: r.Dx(), Height: r.Dy()}
}

// Include every nonzero-alpha pixel; never use color to trim white artwork.
func autoSplitAlphaBounds(im image.Image) image.Rectangle {
	b := im.Bounds()
	r := image.Rect(b.Max.X, b.Max.Y, b.Min.X, b.Min.Y)
	found := false
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			_, _, _, a := im.At(x, y).RGBA()
			if a != 0 {
				if !found {
					r = image.Rect(x, y, x+1, y+1)
					found = true
				} else {
					r = r.Union(image.Rect(x, y, x+1, y+1))
				}
			}
		}
	}
	if !found {
		return image.Rectangle{}
	}
	return r
}

// fitAutoSplitPart is only called with caller-supplied, validated source bounds.
// Premultiplied bilinear sampling avoids dark/colored fringes from transparent RGB.
func fitAutoSplitPart(im image.Image, canvas image.Rectangle, target AutoSplitBounds) *image.NRGBA {
	out := image.NewNRGBA(canvas)
	src := autoSplitAlphaBounds(im)
	scale := math.Min(float64(target.Width)/float64(src.Dx()), float64(target.Height)/float64(src.Dy()))
	w := max(1, min(target.Width, int(math.Round(float64(src.Dx())*scale))))
	h := max(1, min(target.Height, int(math.Round(float64(src.Dy())*scale))))
	left, top := target.X+(target.Width-w)/2, target.Y+(target.Height-h)/2
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			sx := (float64(x)+0.5)*float64(src.Dx())/float64(w) - 0.5 + float64(src.Min.X)
			sy := (float64(y)+0.5)*float64(src.Dy())/float64(h) - 0.5 + float64(src.Min.Y)
			x0, y0 := int(math.Floor(sx)), int(math.Floor(sy))
			fx, fy := sx-float64(x0), sy-float64(y0)
			var sum [4]float64
			for j := 0; j < 2; j++ {
				for i := 0; i < 2; i++ {
					px, py := max(src.Min.X, min(src.Max.X-1, x0+i)), max(src.Min.Y, min(src.Max.Y-1, y0+j))
					r, g, b, a := im.At(px, py).RGBA()
					wx, wy := 1-fx, 1-fy
					if i == 1 {
						wx = fx
					}
					if j == 1 {
						wy = fy
					}
					weight := wx * wy
					sum[0] += float64(r) * weight
					sum[1] += float64(g) * weight
					sum[2] += float64(b) * weight
					sum[3] += float64(a) * weight
				}
			}
			if sum[3] > 0 {
				c := color.NRGBA{A: uint8(math.Round(sum[3] / 257))}
				c.R = uint8(math.Round(math.Min(255, sum[0]*255/sum[3])))
				c.G = uint8(math.Round(math.Min(255, sum[1]*255/sum[3])))
				c.B = uint8(math.Round(math.Min(255, sum[2]*255/sum[3])))
				out.SetNRGBA(left+x, top+y, c)
			}
		}
	}
	return out
}
