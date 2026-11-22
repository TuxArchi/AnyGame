// SPDX-License-Identifier: GPL-2.0-only

// Command icon draws AnyGame's application icon.
package main

import (
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
	"path/filepath"
)

const (
	size     = 512
	samples  = 4
	canvas   = size * samples
	inset    = 16.0
	radius   = 112.0
	screenIn = 104.0
	screenR  = 66.0
)

func main() {
	output := filepath.Join("ui", "assets", "icon.png")
	if len(os.Args) > 1 {
		output = os.Args[1]
	}
	full := image.NewNRGBA(image.Rect(0, 0, canvas, canvas))
	purple := color.NRGBA{0x8b, 0x5c, 0xf6, 0xff}
	cyan := color.NRGBA{0x22, 0xd3, 0xee, 0xff}
	dark := color.NRGBA{0x08, 0x0a, 0x0f, 0xff}
	white := color.NRGBA{0xf7, 0xf5, 0xff, 0xff}
	for y := 0; y < canvas; y++ {
		for x := 0; x < canvas; x++ {
			px := (float64(x) + 0.5) / samples
			py := (float64(y) + 0.5) / samples
			outer := roundedRect(px, py, inset, inset, size-inset, size-inset, radius)
			if outer > 0.5 {
				continue
			}
			t := ((px - inset) + (py - inset)) / (2 * (size - 2*inset))
			t = math.Max(0, math.Min(1, t))
			tint := mix(purple, cyan, t)
			light := 1 - 0.10*math.Abs(px-size/2)/(size/2)
			tint = scale(tint, light)
			screen := roundedRect(px, py, screenIn, screenIn, size-screenIn, size-screenIn, screenR)
			if screen <= 0.5 {
				tint = mix(tint, dark, 0.90)
			}
			if insideTriangle(px, py) {
				tint = white
			}
			full.SetNRGBA(x, y, tint)
		}
	}
	result := downsample(full, samples)
	if err := os.MkdirAll(filepath.Dir(output), 0o755); err != nil {
		panic(err)
	}
	file, err := os.Create(output)
	if err != nil {
		panic(err)
	}
	if err := png.Encode(file, result); err != nil {
		file.Close()
		panic(err)
	}
	if err := file.Close(); err != nil {
		panic(err)
	}
}

func roundedRect(px, py, left, top, right, bottom, radius float64) float64 {
	cx := (left + right) / 2
	cy := (top + bottom) / 2
	halfW := (right - left) / 2
	halfH := (bottom - top) / 2
	qx := math.Abs(px-cx) - (halfW - radius)
	qy := math.Abs(py-cy) - (halfH - radius)
	outside := math.Hypot(math.Max(qx, 0), math.Max(qy, 0))
	inside := math.Min(math.Max(qx, qy), 0)
	return outside + inside - radius
}

func insideTriangle(px, py float64) bool {
	ax, ay := 208.0, 156.0
	bx, by := 208.0, 356.0
	cx, cy := 372.0, 256.0
	d1 := (px-bx)*(ay-by) - (ax-bx)*(py-by)
	d2 := (px-cx)*(by-cy) - (bx-cx)*(py-cy)
	d3 := (px-ax)*(cy-ay) - (cx-ax)*(py-ay)
	hasNegative := d1 < 0 || d2 < 0 || d3 < 0
	hasPositive := d1 > 0 || d2 > 0 || d3 > 0
	return !(hasNegative && hasPositive)
}

func mix(a, b color.NRGBA, t float64) color.NRGBA {
	return color.NRGBA{
		R: uint8(float64(a.R)*(1-t) + float64(b.R)*t),
		G: uint8(float64(a.G)*(1-t) + float64(b.G)*t),
		B: uint8(float64(a.B)*(1-t) + float64(b.B)*t),
		A: 255,
	}
}

func scale(c color.NRGBA, factor float64) color.NRGBA {
	clamp := func(value float64) uint8 {
		if value < 0 {
			return 0
		}
		if value > 255 {
			return 255
		}
		return uint8(value)
	}
	return color.NRGBA{R: clamp(float64(c.R) * factor), G: clamp(float64(c.G) * factor), B: clamp(float64(c.B) * factor), A: 255}
}

func downsample(source *image.NRGBA, factor int) *image.NRGBA {
	width := source.Bounds().Dx() / factor
	height := source.Bounds().Dy() / factor
	result := image.NewNRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			var r, g, b, a int
			for sy := 0; sy < factor; sy++ {
				for sx := 0; sx < factor; sx++ {
					pixel := source.NRGBAAt(x*factor+sx, y*factor+sy)
					r += int(pixel.R)
					g += int(pixel.G)
					b += int(pixel.B)
					a += int(pixel.A)
				}
			}
			count := factor * factor
			result.SetNRGBA(x, y, color.NRGBA{uint8(r / count), uint8(g / count), uint8(b / count), uint8(a / count)})
		}
	}
	return result
}
