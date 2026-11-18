package core

import (
	"image"
	"math"
	"sort"
)

// Pictures for the app: axial slices of an image on the 2 mm atlas grid,
// in gray, with a map or a mask in colour over them.  The clinical report
// has its own figure (Montage), which the Python package draws the same.

// PictureOptions say how Picture draws.  The zero value draws the ten
// slices of the report, five to a row.
type PictureOptions struct {
	Slices  []int   // axial slices, as indices into the atlas grid
	Columns int     // slices to a row
	Zoom    int     // pixels to a voxel
	White   float64 // the uptake shown as white; 0: the brightest brain tissue of the image
	From    float64 // the |value| of the overlay at which its colour starts; 0: 2
	Full    float64 // the |value| at which the colour saturates; 0: 5
	Solid   bool    // the overlay is a mask: voxels above zero get one tint
	Outline bool    // draw the outline of the template's brain over the image
}

// Colours of a mask and of the template's outline.
var (
	pictureTint    = [3]float64{232, 120, 78}
	pictureOutline = [3]byte{94, 214, 166}
)

// templateUptake is a picture of the template to draw maps on when there is
// no image: white matter bright, gray matter darker, as a T1 image.
func templateUptake(a *atlasData) []float32 {
	out := make([]float32, len(a.gm))
	for i := range out {
		out[i] = 0.95*a.wm[i] + 0.55*a.gm[i]
	}
	return out
}

// brightest is the 99.5th percentile of the image inside the brain, which a
// picture shows as white.
func brightest(uptake []float32, brain []bool) float64 {
	var sample []float64
	for i := 0; i < len(uptake); i += 5 {
		if brain[i] && uptake[i] > 0 {
			sample = append(sample, float64(uptake[i]))
		}
	}
	if len(sample) == 0 {
		return 1
	}
	sort.Float64s(sample)
	return percentileSorted(sample, 99.5)
}

// Picture draws axial slices of img in gray with overlay in colour: warm
// where it is positive and cool where negative, or one tint for a mask.
// Without img it draws on the template; overlay may be nil.  Slices are in
// radiological convention: the patient's left is on the right.
func Picture(img, overlay *Volume, opt PictureOptions) *image.RGBA {
	a := atlasOnce()
	slices, columns, zoom := opt.Slices, opt.Columns, opt.Zoom
	if len(slices) == 0 {
		slices = figureSlices
	}
	if columns <= 0 {
		columns = figureColumns
	}
	if zoom <= 0 {
		zoom = figureZoom
	}
	from, full := opt.From, opt.Full
	if from <= 0 {
		from = figureFrom
	}
	if full <= from {
		full = max(figureFull, from+1)
	}
	brain := builtin("parenchyma")
	var uptake []float32
	white := opt.White
	if img != nil {
		uptake = finite(resampleTo(img.Data[:img.NVox()], img.Dim, img.Aff, a.dim, a.aff))
		if white <= 0 {
			white = brightest(uptake, brain)
		}
	} else {
		uptake, white = templateUptake(a), 1
	}
	var score []float32
	if overlay != nil {
		score = finite(resampleTo(overlay.Data[:overlay.NVox()], overlay.Dim, overlay.Aff, a.dim, a.aff))
	}

	nx, ny, nz := a.dim[0], a.dim[1], a.dim[2]
	width, height := nx*zoom, ny*zoom
	nRows := (len(slices) + columns - 1) / columns
	out := image.NewRGBA(image.Rect(0, 0, columns*width+(columns-1)*figureGap, nRows*height+(nRows-1)*figureGap))
	for i := 3; i < len(out.Pix); i += 4 {
		out.Pix[i] = 255
	}
	level := func(v float64) byte { return byte(math.Floor(clamp01(v)*255.0 + 0.5)) }
	edge := func(x, y, k int) bool {
		if !brain[x+nx*(y+ny*k)] {
			return false
		}
		return x == 0 || y == 0 || x == nx-1 || y == ny-1 ||
			!brain[x-1+nx*(y+ny*k)] || !brain[x+1+nx*(y+ny*k)] || !brain[x+nx*(y-1+ny*k)] || !brain[x+nx*(y+1+ny*k)]
	}
	for n, k := range slices {
		if k < 0 || k >= nz {
			continue
		}
		y0, x0 := (n/columns)*(height+figureGap), (n%columns)*(width+figureGap)
		set := func(r, c int, red, green, blue byte) {
			p := out.PixOffset(x0+c, y0+r)
			out.Pix[p], out.Pix[p+1], out.Pix[p+2] = red, green, blue
		}
		// Rows from anterior to posterior, columns from the patient's right to left.
		for r := 0; r < height; r++ {
			for c := 0; c < width; c++ {
				x, y := nx-1-c/zoom, ny-1-r/zoom
				v := x + nx*(y+ny*k)
				gray := level(float64(uptake[v]) / white)
				s := 0.0
				if score != nil {
					s = float64(score[v])
				}
				switch {
				case opt.Solid && s > 0.5:
					g := float64(gray)
					set(r, c, byte(0.72*pictureTint[0]+0.28*g), byte(0.72*pictureTint[1]+0.28*g), byte(0.72*pictureTint[2]+0.28*g))
				case !opt.Solid && s >= from:
					set(r, c, 255, level((s-from)/(full-from)), 0)
				case !opt.Solid && s <= -from:
					set(r, c, 0, level((-s-from)/(full-from)), 255)
				case opt.Outline && edge(x, y, k):
					set(r, c, pictureOutline[0], pictureOutline[1], pictureOutline[2])
				default:
					set(r, c, gray, gray, gray)
				}
			}
		}
		top := height/2 - 7*zoom/2
		for _, mark := range []struct {
			letter byte
			left   int
		}{{'R', 4}, {'L', width - 4 - 5*zoom}} {
			for r, line := range figureGlyphs[mark.letter] {
				for c := range line {
					if line[c] != '1' {
						continue
					}
					for dy := 0; dy < zoom; dy++ {
						for dx := 0; dx < zoom; dx++ {
							set(top+r*zoom+dy, mark.left+c*zoom+dx, figureLetter, figureLetter, figureLetter)
						}
					}
				}
			}
		}
	}
	return out
}

// TemplateImage is a made-up FDG-PET image on the atlas grid, gray matter
// twice as bright as white matter, for demonstrations.
func TemplateImage() *Volume {
	a := atlasOnce()
	vol := NewVolume(a.dim, a.aff)
	for i := range vol.Data {
		vol.Data[i] = a.gm[i] + 0.5*a.wm[i]
	}
	return vol
}
