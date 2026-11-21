package oc

import (
	"image"
	"image/color"
	"image/draw"
)

// repairPartBackground recognizes one or two compact light-neutral border colors
// (including painted transparency checkerboards), then removes only matching
// edge-connected pixels. It neither globally keys white nor infers a semantic mask.
func repairPartBackground(im image.Image) (image.Image, bool) {
	b := im.Bounds()
	w, h := b.Dx(), b.Dy()
	if w < 3 || h < 3 {
		return im, false
	}
	sample := func(x, y int) color.NRGBA { return color.NRGBAModel.Convert(im.At(x, y)).(color.NRGBA) }
	// A fixed histogram avoids retaining/sorting border samples on large images.
	var histogram [256]int
	border := 0
	add := func(x, y int) {
		border++
		if level, ok := partBackgroundNeutralLevel(sample(x, y)); ok {
			histogram[level]++
		}
	}
	for x := b.Min.X; x < b.Max.X; x++ {
		add(x, b.Min.Y)
		add(x, b.Max.Y-1)
	}
	for y := b.Min.Y + 1; y < b.Max.Y-1; y++ {
		add(b.Min.X, y)
		add(b.Max.X-1, y)
	}
	low, high, ok := partBackgroundPalette(histogram, border)
	if !ok {
		return im, false
	}
	match := func(c color.NRGBA) bool {
		level, neutral := partBackgroundNeutralLevel(c)
		// Between two recognized modes, allow neutral antialias blends at tile
		// seams. Border recognition still requires compact modes, not a gradient.
		return neutral && level >= low-18 && level <= high+18
	}
	out := image.NewNRGBA(b)
	draw.Draw(out, b, im, b.Min, draw.Src)
	// Scanline flood fill schedules runs rather than a giant per-pixel queue.
	// Cleared alpha is also the visited marker; the input remains untouched.
	type point struct{ x, y int }
	stack := []point{}
	removed := 0
	eligible := func(x, y int) bool {
		return x >= b.Min.X && x < b.Max.X && y >= b.Min.Y && y < b.Max.Y && match(out.NRGBAAt(x, y))
	}
	flood := func(seed point) {
		stack = append(stack, seed)
		for len(stack) > 0 {
			p := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			if !eligible(p.x, p.y) {
				continue
			}
			left := p.x
			for eligible(left-1, p.y) {
				left--
			}
			for x := left; eligible(x, p.y); x++ {
				c := out.NRGBAAt(x, p.y)
				c.A = 0
				out.SetNRGBA(x, p.y, c)
				removed++
				for _, ny := range []int{p.y - 1, p.y + 1} {
					if eligible(x, ny) && (x == left || !eligible(x-1, ny)) {
						stack = append(stack, point{x, ny})
					}
				}
			}
		}
	}
	for x := b.Min.X; x < b.Max.X; x++ {
		flood(point{x, b.Min.Y})
		flood(point{x, b.Max.Y - 1})
	}
	for y := b.Min.Y + 1; y < b.Max.Y-1; y++ {
		flood(point{b.Min.X, y})
		flood(point{b.Max.X - 1, y})
	}
	// Blank/near-empty extractions are not successes, including empty checkers.
	if removed == 0 || removed*100 > w*h*99 {
		return im, false
	}
	return out, true
}

// Restrict repair to opaque, light neutral backgrounds. Small channel differences
// tolerate compression/antialias noise without admitting saturated artwork.
func partBackgroundNeutralLevel(c color.NRGBA) (int, bool) {
	level := (int(c.R) + int(c.G) + int(c.B)) / 3
	neutral := absByte(c.R, c.G) <= 20 && absByte(c.G, c.B) <= 20 && absByte(c.R, c.B) <= 20
	return level, c.A >= 250 && level >= 128 && neutral
}

func partBackgroundPalette(histogram [256]int, border int) (low, high int, ok bool) {
	neutral := 0
	for _, n := range histogram {
		neutral += n
	}
	if neutral*100 < border*90 {
		return 0, 0, false
	}
	covered := 0
	for mode := 0; mode < 2; mode++ {
		best, center := 0, 0
		for c := 128; c < 256; c++ {
			n := 0
			for i := max(128, c-12); i <= min(255, c+12); i++ {
				n += histogram[i]
			}
			if n > best {
				best, center = n, c
			}
		}
		// A second color must have material support, not a few border artifacts.
		if best == 0 || (mode == 1 && best*100 < neutral*8) {
			break
		}
		weighted := 0
		for i := max(128, center-12); i <= min(255, center+12); i++ {
			weighted += i * histogram[i]
			histogram[i] = 0
		}
		level := weighted / best
		if mode == 0 {
			low, high = level, level
		} else {
			low, high = min(low, level), max(high, level)
		}
		covered += best
	}
	// Reject multicolor neutrals/gradients rather than widening the palette to
	// fit arbitrary imagery. A small remainder permits antialiased tile seams.
	return low, high, covered*100 >= neutral*95 && covered*100 >= border*90
}

func absByte(a, b uint8) int {
	if a > b {
		return int(a - b)
	}
	return int(b - a)
}
