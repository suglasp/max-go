// blur.go : builds a blurred widescreen backdrop from 4:3 splash artwork.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
//
// The retail game has no widescreen counterpart. A zoomed and blurred copy of the 4:3 logo fills the otherwise black side bars.
package intro

import "github.com/suglasp/max-go/internal/gfx"

const (
	// blurRadius is the box blur window half width in pixels.
	blurRadius = 10
	// blurPasses repeats the box blur; two passes come close to a Gaussian.
	blurPasses = 2
)

// blurLogoMem builds the widescreen splash backdrop from the logo image in
// memory: the logo is scaled up so it covers width x height (one uniform
// factor, the overflow cropped equally on both sides), box blurred in RGB
// space, and every pixel is mapped back to the nearest entry of the logo's
// own palette. Reusing that palette means the splash fade dims the backdrop
// and the logo together.
func blurLogoMem(logo *gfx.Buffer, pal *gfx.Palette, width, height int) *gfx.Buffer {
	if width <= 0 || height <= 0 {
		return gfx.New(0, 0)
	}
	backdrop := gfx.New(width, height)
	if logo == nil || pal == nil || logo.W <= 0 || logo.H <= 0 {
		return backdrop
	}

	// Cover scale in 16.16 fixed point: step is how far one output pixel
	// advances through the source. The smaller per axis step zooms enough to
	// fill both axes, so the other axis crops instead of squashing.
	stepX := (logo.W << 16) / width
	stepY := (logo.H << 16) / height
	step := stepX
	if stepY < stepX {
		step = stepY
	}
	srcStartX := (logo.W<<16 - width*step) / 2
	srcStartY := (logo.H<<16 - height*step) / 2

	// Expand the stretched logo into one plane per channel, in the palette's
	// own 6 bit VGA range.
	red := make([]uint8, width*height)
	green := make([]uint8, width*height)
	blue := make([]uint8, width*height)
	for y := 0; y < height; y++ {
		srcRow := logo.Row(clampIndex((srcStartY+y*step)>>16, logo.H))
		for x := 0; x < width; x++ {
			srcX := clampIndex((srcStartX+x*step)>>16, logo.W)
			r, g, b := pal.RGB(int(srcRow[srcX]))
			i := y*width + x
			red[i], green[i], blue[i] = r, g, b
		}
	}

	// Soften each channel.
	for pass := 0; pass < blurPasses; pass++ {
		boxBlurPlane(red, width, height, blurRadius)
		boxBlurPlane(green, width, height, blurRadius)
		boxBlurPlane(blue, width, height, blurRadius)
	}

	// Map the blurred colors back to palette indices. Blurring leaves few
	// distinct colors, so an RGB555 cache keeps the nearest search cheap.
	var cache [1 << 15]int16 // RGB555 key -> palette index + 1; 0 = empty slot
	for i := range backdrop.Pix {
		r, g, b := red[i], green[i], blue[i]
		key := int(r>>1)<<10 | int(g>>1)<<5 | int(b>>1)
		idx := cache[key]
		if idx == 0 {
			idx = int16(nearestPaletteIndex(pal, r, g, b)) + 1
			cache[key] = idx
		}
		backdrop.Pix[i] = uint8(idx - 1)
	}
	return backdrop
}

// boxBlurPlane blurs one color plane in place: a horizontal then a vertical
// sliding window average over 2*radius+1 samples, clamped at the edges.
func boxBlurPlane(plane []uint8, width, height, radius int) {
	if radius <= 0 || width <= 0 || height <= 0 {
		return
	}
	tmp := make([]uint8, len(plane))
	window := 2*radius + 1

	// Horizontal pass: plane -> tmp.
	for y := 0; y < height; y++ {
		row := plane[y*width : (y+1)*width]
		out := tmp[y*width : (y+1)*width]
		sum := 0
		// Prime the window centered on x = 0.
		for i := -radius; i <= radius; i++ {
			sum += int(row[clampIndex(i, width)])
		}
		for x := 0; x < width; x++ {
			out[x] = uint8(sum / window)
			sum += int(row[clampIndex(x+radius+1, width)]) - int(row[clampIndex(x-radius, width)])
		}
	}

	// Vertical pass: tmp -> plane.
	for x := 0; x < width; x++ {
		sum := 0
		// Prime the window centered on y = 0.
		for i := -radius; i <= radius; i++ {
			sum += int(tmp[clampIndex(i, height)*width+x])
		}
		for y := 0; y < height; y++ {
			plane[y*width+x] = uint8(sum / window)
			sum += int(tmp[clampIndex(y+radius+1, height)*width+x]) - int(tmp[clampIndex(y-radius, height)*width+x])
		}
	}
}

// nearestPaletteIndex finds the palette entry closest to (r, g, b) by squared
// RGB distance.
func nearestPaletteIndex(pal *gfx.Palette, r, g, b uint8) int {
	best := 0
	bestDist := 1 << 30
	for i := 0; i < gfx.PaletteSize; i++ {
		pr, pg, pb := pal.RGB(i)
		dr := int(pr) - int(r)
		dg := int(pg) - int(g)
		db := int(pb) - int(b)
		dist := dr*dr + dg*dg + db*db
		if dist < bestDist {
			bestDist = dist
			best = i
		}
	}
	return best
}

// clampIndex clamps i into [0, size).
func clampIndex(i, size int) int {
	if i < 0 {
		return 0
	}
	if i >= size {
		return size - 1
	}
	return i
}
