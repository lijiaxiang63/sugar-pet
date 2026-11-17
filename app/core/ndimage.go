package core

import (
	"math"
	"runtime"
	"sync"
)

// parallelFor splits [0, n) into chunks and runs fn on each concurrently.
func parallelFor(n int, fn func(lo, hi int)) {
	workers := runtime.GOMAXPROCS(0)
	if n < 4096 || workers < 2 {
		fn(0, n)
		return
	}
	if workers > n {
		workers = n
	}
	var wg sync.WaitGroup
	step := (n + workers - 1) / workers
	for lo := 0; lo < n; lo += step {
		hi := min(lo+step, n)
		wg.Add(1)
		go func() {
			defer wg.Done()
			fn(lo, hi)
		}()
	}
	wg.Wait()
}

// VoxelSizes returns the voxel sizes in mm (the column norms of the affine).
func VoxelSizes(a Affine) [3]float64 {
	var out [3]float64
	for c := 0; c < 3; c++ {
		out[c] = math.Sqrt(a[0][c]*a[0][c] + a[1][c]*a[1][c] + a[2][c]*a[2][c])
	}
	return out
}

// VoxelVolumeML returns the volume of one voxel in millilitres.
func VoxelVolumeML(a Affine) float64 {
	det := a[0][0]*(a[1][1]*a[2][2]-a[1][2]*a[2][1]) -
		a[0][1]*(a[1][0]*a[2][2]-a[1][2]*a[2][0]) +
		a[0][2]*(a[1][0]*a[2][1]-a[1][1]*a[2][0])
	return math.Abs(det) / 1000
}

// invAffine inverts an affine whose last row is (0, 0, 0, 1).
func invAffine(a Affine) (Affine, bool) {
	m := [3][3]float64{}
	for r := 0; r < 3; r++ {
		for c := 0; c < 3; c++ {
			m[r][c] = a[r][c]
		}
	}
	det := m[0][0]*(m[1][1]*m[2][2]-m[1][2]*m[2][1]) -
		m[0][1]*(m[1][0]*m[2][2]-m[1][2]*m[2][0]) +
		m[0][2]*(m[1][0]*m[2][1]-m[1][1]*m[2][0])
	var out Affine
	if det == 0 || math.IsNaN(det) {
		return out, false
	}
	inv := [3][3]float64{
		{m[1][1]*m[2][2] - m[1][2]*m[2][1], m[0][2]*m[2][1] - m[0][1]*m[2][2], m[0][1]*m[1][2] - m[0][2]*m[1][1]},
		{m[1][2]*m[2][0] - m[1][0]*m[2][2], m[0][0]*m[2][2] - m[0][2]*m[2][0], m[0][2]*m[1][0] - m[0][0]*m[1][2]},
		{m[1][0]*m[2][1] - m[1][1]*m[2][0], m[0][1]*m[2][0] - m[0][0]*m[2][1], m[0][0]*m[1][1] - m[0][1]*m[1][0]},
	}
	for r := 0; r < 3; r++ {
		for c := 0; c < 3; c++ {
			out[r][c] = inv[r][c] / det
		}
	}
	for r := 0; r < 3; r++ {
		out[r][3] = -(out[r][0]*a[0][3] + out[r][1]*a[1][3] + out[r][2]*a[2][3])
	}
	out[3][3] = 1
	return out, true
}

func mulAffine(a, b Affine) Affine {
	var out Affine
	for r := 0; r < 4; r++ {
		for c := 0; c < 4; c++ {
			var s float64
			for k := 0; k < 4; k++ {
				s += a[r][k] * b[k][c]
			}
			out[r][c] = s
		}
	}
	return out
}

// SameGrid reports whether two images share shape and affine (to 1e-3).
func SameGrid(aDim [3]int, aAff Affine, bDim [3]int, bAff Affine) bool {
	if aDim != bDim {
		return false
	}
	for r := 0; r < 4; r++ {
		for c := 0; c < 4; c++ {
			if math.Abs(aAff[r][c]-bAff[r][c]) > 1e-3+1e-5*math.Abs(bAff[r][c]) {
				return false
			}
		}
	}
	return true
}

// resampleLinear resamples src onto another grid by trilinear interpolation,
// like scipy.ndimage.affine_transform(order=1, mode="constant", cval=0):
// nothing is interpolated beyond the edges of the source.  With roundUint8
// the result is rounded as scipy rounds into a uint8 array (masks).
func resampleLinear(src []float32, sDim [3]int, sAff Affine, tDim [3]int, tAff Affine, roundUint8 bool) []float32 {
	out := make([]float32, tDim[0]*tDim[1]*tDim[2])
	inv, ok := invAffine(sAff)
	if !ok {
		return out
	}
	m := mulAffine(inv, tAff) // target voxel -> source voxel
	nx, ny := sDim[0], sDim[1]
	maxc := [3]float64{float64(sDim[0] - 1), float64(sDim[1] - 1), float64(sDim[2] - 1)}
	tnx, tny, tnz := tDim[0], tDim[1], tDim[2]
	parallelFor(tnz, func(lo, hi int) {
		for k := lo; k < hi; k++ {
			for j := 0; j < tny; j++ {
				row := (k*tny + j) * tnx
				for i := 0; i < tnx; i++ {
					var cc [3]float64
					inside := true
					for h := 0; h < 3; h++ {
						v := m[h][3]
						v += float64(m[h][0] * float64(i))
						v += float64(m[h][1] * float64(j))
						v += float64(m[h][2] * float64(k))
						if !(v >= 0 && v <= maxc[h]) {
							inside = false
							break
						}
						cc[h] = v
					}
					if !inside {
						continue
					}
					var idx [3][2]int
					var w [3][2]float64
					for h := 0; h < 3; h++ {
						f := math.Floor(cc[h])
						fr := cc[h] - f
						i0 := int(f)
						i1 := i0 + 1
						if i1 > sDim[h]-1 {
							i1 = i0 // weight is zero there
						}
						idx[h] = [2]int{i0, i1}
						w[h] = [2]float64{1 - fr, fr}
					}
					var t float64
					for a := 0; a < 2; a++ {
						for b := 0; b < 2; b++ {
							for c := 0; c < 2; c++ {
								term := float64(src[idx[0][a]+nx*(idx[1][b]+ny*idx[2][c])])
								term = float64(term * w[0][a])
								term = float64(term * w[1][b])
								term = float64(term * w[2][c])
								t += term
							}
						}
					}
					if roundUint8 {
						if t > 0 {
							t = math.Floor(t + 0.5)
						} else {
							t = 0
						}
						if t > 255 {
							t = 255
						}
					}
					out[row+i] = float32(t)
				}
			}
		}
	})
	return out
}

const fwhmToSigma = 0.42466090014400953 // 1 / (2 sqrt(2 ln 2))

// gaussKernel is scipy's Gaussian kernel: half of it, centre first.
func gaussKernel(sigma float64) []float64 {
	radius := int(4.0*sigma + 0.5)
	half := make([]float64, radius+1)
	sum := 0.0
	for x := -radius; x <= radius; x++ {
		v := math.Exp(-0.5 / (sigma * sigma) * float64(x*x))
		sum += v
		if x >= 0 {
			half[x] = v
		}
	}
	for i := range half {
		half[i] /= sum
	}
	return half
}

// gaussianFilter smooths a volume like scipy.ndimage.gaussian_filter with
// mode="constant": one axis after the other, each rounded to float32.
func gaussianFilter(data []float32, dim [3]int, sigma [3]float64) []float32 {
	out := make([]float32, len(data))
	copy(out, data)
	nx, ny, nz := dim[0], dim[1], dim[2]
	for axis := 0; axis < 3; axis++ {
		if sigma[axis] <= 1e-15 {
			continue
		}
		half := gaussKernel(sigma[axis])
		radius := len(half) - 1
		var n, stride, lines int
		switch axis {
		case 0:
			n, stride, lines = nx, 1, ny*nz
		case 1:
			n, stride, lines = ny, nx, nx*nz
		default:
			n, stride, lines = nz, nx*ny, nx*ny
		}
		parallelFor(lines, func(lo, hi int) {
			buf := make([]float64, n+2*radius)
			for line := lo; line < hi; line++ {
				var start int
				switch axis {
				case 0:
					start = line * nx
				case 1:
					start = (line/nx)*nx*ny + line%nx
				default:
					start = line
				}
				for i := 0; i < n; i++ {
					buf[radius+i] = float64(out[start+i*stride])
				}
				for i := 0; i < n; i++ {
					c := radius + i
					acc := buf[c] * half[0]
					for r := 1; r <= radius; r++ {
						acc += float64((buf[c-r] + buf[c+r]) * half[r])
					}
					out[start+i*stride] = float32(acc)
				}
			}
		})
	}
	return out
}

// gaussianFilter1D smooths a 1D signal (mode="constant").
func gaussianFilter1D(data []float64, sigma float64) []float64 {
	out := make([]float64, len(data))
	if sigma <= 1e-15 {
		copy(out, data)
		return out
	}
	half := gaussKernel(sigma)
	radius := len(half) - 1
	buf := make([]float64, len(data)+2*radius)
	copy(buf[radius:], data)
	for i := range data {
		c := radius + i
		acc := buf[c] * half[0]
		for r := 1; r <= radius; r++ {
			acc += float64((buf[c-r] + buf[c+r]) * half[r])
		}
		out[i] = acc
	}
	return out
}

// Smooth applies a Gaussian kernel given as FWHM in mm (0 = no smoothing).
func Smooth(data []float32, dim [3]int, aff Affine, fwhm float64) []float32 {
	if fwhm == 0 {
		return data
	}
	zoom := VoxelSizes(aff)
	clean := make([]float32, len(data))
	for i, v := range data {
		if f := float64(v); !math.IsNaN(f) && !math.IsInf(f, 0) {
			clean[i] = v
		}
	}
	return gaussianFilter(clean, dim, [3]float64{
		fwhm * fwhmToSigma / zoom[0], fwhm * fwhmToSigma / zoom[1], fwhm * fwhmToSigma / zoom[2],
	})
}

// offsets3 lists the neighbours of a structuring element.
type offset3 struct{ dx, dy, dz int }

var (
	cross6    = []offset3{{0, 0, 0}, {1, 0, 0}, {-1, 0, 0}, {0, 1, 0}, {0, -1, 0}, {0, 0, 1}, {0, 0, -1}}
	inPlane33 = func() []offset3 {
		var o []offset3
		for dx := -1; dx <= 1; dx++ {
			for dy := -1; dy <= 1; dy++ {
				o = append(o, offset3{dx, dy, 0})
			}
		}
		return o
	}()
)

// neighbours returns the offsets of 18- or 26-connectivity (without the centre).
func neighbours(connectivity int) []offset3 {
	var o []offset3
	for dx := -1; dx <= 1; dx++ {
		for dy := -1; dy <= 1; dy++ {
			for dz := -1; dz <= 1; dz++ {
				n := dx*dx + dy*dy + dz*dz
				if n == 0 || (connectivity == 18 && n == 3) {
					continue
				}
				o = append(o, offset3{dx, dy, dz})
			}
		}
	}
	return o
}

// binaryMorph erodes (all neighbours set) or dilates (any neighbour set);
// voxels outside the image count as unset.
func binaryMorph(mask []bool, dim [3]int, structure []offset3, iterations int, erode bool) []bool {
	nx, ny, nz := dim[0], dim[1], dim[2]
	cur := mask
	for it := 0; it < iterations; it++ {
		next := make([]bool, len(cur))
		src := cur
		parallelFor(nz, func(lo, hi int) {
			for z := lo; z < hi; z++ {
				for y := 0; y < ny; y++ {
					for x := 0; x < nx; x++ {
						result := erode
						for _, o := range structure {
							xx, yy, zz := x+o.dx, y+o.dy, z+o.dz
							set := xx >= 0 && xx < nx && yy >= 0 && yy < ny && zz >= 0 && zz < nz && src[xx+nx*(yy+ny*zz)]
							if erode && !set {
								result = false
								break
							}
							if !erode && set {
								result = true
								break
							}
						}
						next[x+nx*(y+ny*z)] = result
					}
				}
			}
		})
		cur = next
	}
	return cur
}

// labelComponents labels connected components.  Labels are numbered in the
// order scipy.ndimage.label meets them (x slowest, z fastest).
func labelComponents(mask []bool, dim [3]int, connectivity int) ([]int32, int) {
	nx, ny, nz := dim[0], dim[1], dim[2]
	labels := make([]int32, len(mask))
	nb := neighbours(connectivity)
	var stack []int32
	count := 0
	for x := 0; x < nx; x++ {
		for y := 0; y < ny; y++ {
			for z := 0; z < nz; z++ {
				start := x + nx*(y+ny*z)
				if !mask[start] || labels[start] != 0 {
					continue
				}
				count++
				labels[start] = int32(count)
				stack = append(stack[:0], int32(start))
				for len(stack) > 0 {
					p := int(stack[len(stack)-1])
					stack = stack[:len(stack)-1]
					px, py, pz := p%nx, (p/nx)%ny, p/(nx*ny)
					for _, o := range nb {
						xx, yy, zz := px+o.dx, py+o.dy, pz+o.dz
						if xx < 0 || xx >= nx || yy < 0 || yy >= ny || zz < 0 || zz >= nz {
							continue
						}
						q := xx + nx*(yy+ny*zz)
						if mask[q] && labels[q] == 0 {
							labels[q] = int32(count)
							stack = append(stack, int32(q))
						}
					}
				}
			}
		}
	}
	return labels, count
}

// cOrder calls fn for every voxel in NumPy's C order (x slowest, z fastest).
func cOrder(dim [3]int, fn func(idx, x, y, z int)) {
	nx, ny, nz := dim[0], dim[1], dim[2]
	for x := 0; x < nx; x++ {
		for y := 0; y < ny; y++ {
			for z := 0; z < nz; z++ {
				fn(x+nx*(y+ny*z), x, y, z)
			}
		}
	}
}

func countTrue(mask []bool) int {
	n := 0
	for _, m := range mask {
		if m {
			n++
		}
	}
	return n
}
