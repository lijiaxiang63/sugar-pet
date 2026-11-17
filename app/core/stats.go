package core

import (
	"math"
	"sort"
	"strconv"
)

// fmtG formats a number like Python's "{:g}" / "{:.6g}".
func fmtG(v float64) string { return strconv.FormatFloat(v, 'g', 6, 64) }

// normSF is the upper tail of the standard normal distribution.
func normSF(z float64) float64 { return 0.5 * math.Erfc(z/math.Sqrt2) }

// normPPF is the quantile function of the standard normal distribution
// (Wichura's algorithm AS 241, accurate to about 1e-16).
func normPPF(p float64) float64 {
	if math.IsNaN(p) || p < 0 || p > 1 {
		return math.NaN()
	}
	if p == 0 {
		return math.Inf(-1)
	}
	if p == 1 {
		return math.Inf(1)
	}
	q := p - 0.5
	if math.Abs(q) <= 0.425 {
		r := 0.180625 - q*q
		num := (((((((2509.0809287301226727*r+33430.575583588128105)*r+67265.770927008700853)*r+
			45921.953931549871457)*r+13731.693765509461125)*r+1971.5909503065514427)*r+
			133.14166789178437745)*r + 3.387132872796366608)
		den := (((((((5226.495278852545925*r+28729.085735721942674)*r+39307.89580009271061)*r+
			21213.794301586595867)*r+5394.1960214247511077)*r+687.1870074920579083)*r+
			42.313330701600911252)*r + 1.0)
		return q * num / den
	}
	r := p
	if q > 0 {
		r = 1 - p
	}
	r = math.Sqrt(-math.Log(r))
	var x float64
	if r <= 5 {
		r -= 1.6
		num := (((((((7.7454501427834140764e-4*r+0.0227238449892691845833)*r+0.24178072517745061177)*r+
			1.27045825245236838258)*r+3.64784832476320460504)*r+5.7694972214606914055)*r+
			4.6303378461565452959)*r + 1.42343711074968357734)
		den := (((((((1.05075007164441684324e-9*r+5.475938084995344946e-4)*r+0.0151986665636164571966)*r+
			0.14810397642748007459)*r+0.68976733498510000455)*r+1.6763848301838038494)*r+
			2.05319162663775882187)*r + 1.0)
		x = num / den
	} else {
		r -= 5
		num := (((((((2.01033439929228813265e-7*r+2.71155556874348757815e-5)*r+0.0012426609473880784386)*r+
			0.026532189526576123093)*r+0.29656057182850489123)*r+1.7848265399172913358)*r+
			5.4637849111641143699)*r + 6.6579046435011037772)
		den := (((((((2.04426310338993978564e-15*r+1.4215117583164458887e-7)*r+1.8463183175100546818e-5)*r+
			7.868691311456132591e-4)*r+0.0148753612908506148525)*r+0.13692988092273580531)*r+
			0.59983220655588793769)*r + 1.0)
		x = num / den
	}
	if q < 0 {
		return -x
	}
	return x
}

// betaInc is the regularised incomplete beta function I_x(a, b).
func betaInc(a, b, x float64) float64 {
	if x <= 0 {
		return 0
	}
	if x >= 1 {
		return 1
	}
	la, _ := math.Lgamma(a)
	lb, _ := math.Lgamma(b)
	lab, _ := math.Lgamma(a + b)
	front := math.Exp(lab - la - lb + a*math.Log(x) + b*math.Log1p(-x))
	if x < (a+1)/(a+b+2) {
		return front * betaCF(a, b, x) / a
	}
	return 1 - front*betaCF(b, a, 1-x)/b
}

// betaCF evaluates the continued fraction of the incomplete beta function
// (modified Lentz's method).
func betaCF(a, b, x float64) float64 {
	const tiny = 1e-300
	qab, qap, qam := a+b, a+1, a-1
	c, d := 1.0, 1-qab*x/qap
	if math.Abs(d) < tiny {
		d = tiny
	}
	d = 1 / d
	h := d
	for m := 1; m <= 500; m++ {
		fm := float64(m)
		aa := fm * (b - fm) * x / ((qam + 2*fm) * (a + 2*fm))
		d = 1 + aa*d
		if math.Abs(d) < tiny {
			d = tiny
		}
		c = 1 + aa/c
		if math.Abs(c) < tiny {
			c = tiny
		}
		d = 1 / d
		h *= d * c
		aa = -(a + fm) * (qab + fm) * x / ((a + 2*fm) * (qap + 2*fm))
		d = 1 + aa*d
		if math.Abs(d) < tiny {
			d = tiny
		}
		c = 1 + aa/c
		if math.Abs(c) < tiny {
			c = tiny
		}
		d = 1 / d
		del := d * c
		h *= del
		if math.Abs(del-1) < 3e-16 {
			break
		}
	}
	return h
}

// tSF is the upper tail of Student's t distribution.
func tSF(t, df float64) float64 {
	if math.IsNaN(t) {
		return math.NaN()
	}
	if t < 0 {
		return 1 - tSF(-t, df)
	}
	if math.IsInf(t, 1) {
		return 0
	}
	return 0.5 * betaInc(df/2, 0.5, df/(df+t*t))
}

// tLogSF is the log of the upper tail of Student's t distribution for a t far
// from 0, where the tail itself may be too small for a number.
func tLogSF(t, df float64) float64 {
	a, b, x := df/2, 0.5, df/(df+t*t)
	if x >= (a+1)/(a+b+2) {
		return math.Log(tSF(t, df))
	}
	la, _ := math.Lgamma(a)
	lb, _ := math.Lgamma(b)
	lab, _ := math.Lgamma(a + b)
	return math.Log(0.5) + lab - la - lb + a*math.Log(x) + b*math.Log1p(-x) + math.Log(betaCF(a, b, x)/a)
}

// normLogSF is the log of the upper tail of the standard normal distribution.
func normLogSF(z float64) float64 {
	if z < 25 {
		return math.Log(normSF(z))
	}
	// The asymptotic series of Mills' ratio.
	u := 1 / (z * z)
	series := 1 - u*(1-3*u*(1-5*u*(1-7*u*(1-9*u*(1-11*u*(1-13*u))))))
	return -0.5*z*z - math.Log(z) - 0.5*math.Log(2*math.Pi) + math.Log(series)
}

// normISFLog is the z whose upper tail in the standard normal distribution
// has the log logp, for a tail too small for normPPF.
func normISFLog(logp float64) float64 {
	z := math.Sqrt(-2 * logp)
	for i := 0; i < 50; i++ {
		tail := normLogSF(z)
		// Newton's step: the slope of the log of the tail is minus the hazard.
		hazard := math.Exp(-0.5*z*z - 0.5*math.Log(2*math.Pi) - tail)
		step := (tail - logp) / hazard
		z += step
		if math.Abs(step) < 1e-15*z {
			break
		}
	}
	return z
}

// tPPF is the quantile function of Student's t distribution.
func tPPF(p, df float64) float64 {
	if math.IsNaN(p) || p < 0 || p > 1 {
		return math.NaN()
	}
	if p == 0.5 {
		return 0
	}
	if p < 0.5 {
		return -tISF(p, df)
	}
	return tISF(1-p, df)
}

// tISF finds t >= 0 whose upper tail is q (0 < q <= 0.5).
func tISF(q, df float64) float64 {
	if q <= 0 {
		return math.Inf(1)
	}
	lo, hi := 0.0, 1.0
	for tSF(hi, df) > q {
		lo, hi = hi, hi*2
		if hi > 1e300 {
			return math.Inf(1)
		}
	}
	for i := 0; i < 200; i++ {
		mid := 0.5 * (lo + hi)
		if mid == lo || mid == hi {
			break
		}
		if tSF(mid, df) > q {
			lo = mid
		} else {
			hi = mid
		}
	}
	return 0.5 * (lo + hi)
}

// percentile of sorted values, with linear interpolation like NumPy.
func percentileSorted(sorted []float64, q float64) float64 {
	n := len(sorted)
	if n == 0 {
		return math.NaN()
	}
	pos := q / 100 * float64(n-1)
	lo := int(math.Floor(pos))
	if lo >= n-1 {
		return sorted[n-1]
	}
	frac := pos - float64(lo)
	return sorted[lo] + (sorted[lo+1]-sorted[lo])*frac
}

func medianSorted(sorted []float64) float64 {
	n := len(sorted)
	if n == 0 {
		return math.NaN()
	}
	if n%2 == 1 {
		return sorted[n/2]
	}
	return 0.5 * (sorted[n/2-1] + sorted[n/2])
}

// histogramMode returns the most frequent value of a sample: the peak of
// its histogram smoothed with a Gaussian kernel, refined by a parabola fit.
func histogramMode(values []float32) (float64, error) {
	const nBins = 2048
	vals := make([]float64, 0, len(values))
	for _, v := range values {
		if f := float64(v); !math.IsNaN(f) && !math.IsInf(f, 0) {
			vals = append(vals, f)
		}
	}
	if len(vals) < 100 {
		return 0, errorf("too few voxels to estimate a histogram mode")
	}
	sort.Float64s(vals)
	lo, hi := percentileSorted(vals, 0.5), percentileSorted(vals, 99.5)
	if hi <= lo {
		return lo, nil
	}
	median := medianSorted(vals)
	dev := make([]float64, len(vals))
	for i, v := range vals {
		dev[i] = math.Abs(v - median)
	}
	sort.Float64s(dev)
	spread := 1.4826 * medianSorted(dev)
	bandwidth := (hi - lo) / 50
	if spread > 0 {
		bandwidth = 0.25 * spread
	}
	edges := make([]float64, nBins+1)
	for i := range edges {
		edges[i] = lo + (hi-lo)*float64(i)/nBins
	}
	edges[nBins] = hi
	counts := make([]float64, nBins)
	for _, v := range vals {
		if v < lo || v > hi {
			continue
		}
		k := int((v - lo) / (hi - lo) * nBins)
		if k >= nBins {
			k = nBins - 1
		}
		if v < edges[k] {
			k--
		} else if k != nBins-1 && v >= edges[k+1] {
			k++
		}
		counts[k]++
	}
	width := edges[1] - edges[0]
	density := gaussianFilter1D(counts, bandwidth/width)
	k := 0
	for i, d := range density {
		if d > density[k] {
			k = i
		}
	}
	centre := 0.5 * (edges[k] + edges[k+1])
	if k > 0 && k < nBins-1 {
		y0, y1, y2 := density[k-1], density[k], density[k+1]
		if denom := y0 - 2*y1 + y2; denom != 0 {
			return centre + 0.5*(y0-y2)/denom*width, nil
		}
	}
	return centre, nil
}

// invertMatrix inverts a small square matrix by Gauss-Jordan elimination
// with partial pivoting.  It reports false when the matrix is singular.
func invertMatrix(a [][]float64) ([][]float64, bool) {
	n := len(a)
	work := make([][]float64, n)
	scale := 0.0
	for i := range a {
		work[i] = make([]float64, 2*n)
		copy(work[i], a[i])
		work[i][n+i] = 1
		for _, v := range a[i] {
			scale = math.Max(scale, math.Abs(v))
		}
	}
	for col := 0; col < n; col++ {
		pivot := col
		for r := col + 1; r < n; r++ {
			if math.Abs(work[r][col]) > math.Abs(work[pivot][col]) {
				pivot = r
			}
		}
		if math.Abs(work[pivot][col]) <= 1e-12*scale {
			return nil, false
		}
		work[col], work[pivot] = work[pivot], work[col]
		p := work[col][col]
		for c := range work[col] {
			work[col][c] /= p
		}
		for r := 0; r < n; r++ {
			if r == col {
				continue
			}
			f := work[r][col]
			if f == 0 {
				continue
			}
			for c := range work[r] {
				work[r][c] -= f * work[col][c]
			}
		}
	}
	out := make([][]float64, n)
	for i := range out {
		out[i] = work[i][n:]
	}
	return out, true
}

// gram returns X'X for a design matrix (rows x columns).
func gram(x [][]float64) [][]float64 {
	p := len(x[0])
	out := make([][]float64, p)
	for a := 0; a < p; a++ {
		out[a] = make([]float64, p)
		for b := 0; b < p; b++ {
			var s float64
			for _, row := range x {
				s += row[a] * row[b]
			}
			out[a][b] = s
		}
	}
	return out
}

// fullRank reports whether the columns of a design matrix are independent.
func fullRank(x [][]float64) bool {
	n, p := len(x), len(x[0])
	if n < p {
		return false
	}
	// Scale the columns so the test does not depend on their units.
	norm := make([]float64, p)
	for c := 0; c < p; c++ {
		for _, row := range x {
			norm[c] += row[c] * row[c]
		}
		norm[c] = math.Sqrt(norm[c])
		if norm[c] == 0 {
			return false
		}
	}
	scaled := make([][]float64, n)
	for r, row := range x {
		scaled[r] = make([]float64, p)
		for c := range row {
			scaled[r][c] = row[c] / norm[c]
		}
	}
	// Modified Gram-Schmidt: a dependent column leaves (almost) nothing.
	for c := 0; c < p; c++ {
		for prev := 0; prev < c; prev++ {
			var dot float64
			for r := 0; r < n; r++ {
				dot += scaled[r][c] * scaled[r][prev]
			}
			for r := 0; r < n; r++ {
				scaled[r][c] -= dot * scaled[r][prev]
			}
		}
		var length float64
		for r := 0; r < n; r++ {
			length += scaled[r][c] * scaled[r][c]
		}
		length = math.Sqrt(length)
		if length < 1e-10 {
			return false
		}
		for r := 0; r < n; r++ {
			scaled[r][c] /= length
		}
	}
	return true
}

// pinv returns the pseudo-inverse (columns x rows) of a full-rank design matrix.
func pinv(x [][]float64) ([][]float64, bool) {
	inv, ok := invertMatrix(gram(x))
	if !ok {
		return nil, false
	}
	n, p := len(x), len(x[0])
	out := make([][]float64, p)
	for j := 0; j < p; j++ {
		out[j] = make([]float64, n)
		for i := 0; i < n; i++ {
			var s float64
			for k := 0; k < p; k++ {
				s += inv[j][k] * x[i][k]
			}
			out[j][i] = s
		}
	}
	return out, true
}
