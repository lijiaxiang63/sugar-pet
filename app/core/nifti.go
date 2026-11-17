// Package core is the Go port of the sugar-pet Python package: intensity
// normalisation of brain FDG-PET images in MNI space.
//
// Volumes are stored as float32 in file order (x fastest, then y, then z),
// which is what NumPy calls Fortran order.  Wherever the Python code depends
// on NumPy's C order (labelling, argmax ties) the loops here follow it.
package core

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"strings"
)

// Affine maps voxel indices to world coordinates in millimetres.
type Affine [4][4]float64

// Volume is a 3D image, or a 4D one with NT frames.
type Volume struct {
	Dim     [3]int
	NT      int
	Data    []float32 // len = Dim[0]*Dim[1]*Dim[2]*NT
	Aff     Affine
	Descrip string
}

// NVox is the number of voxels of one frame.
func (v *Volume) NVox() int { return v.Dim[0] * v.Dim[1] * v.Dim[2] }

// Frame returns frame t (sharing memory with the volume).
func (v *Volume) Frame(t int) []float32 {
	n := v.NVox()
	return v.Data[t*n : (t+1)*n]
}

// NewVolume makes an empty 3D volume on the grid of another one.
func NewVolume(dim [3]int, aff Affine) *Volume {
	return &Volume{Dim: dim, NT: 1, Data: make([]float32, dim[0]*dim[1]*dim[2]), Aff: aff}
}

// StripExt removes the NIfTI extension of a file name.
func StripExt(name string) string {
	for _, ext := range []string{".nii.gz", ".nii", ".img", ".hdr"} {
		if strings.HasSuffix(name, ext) {
			return name[:len(name)-len(ext)]
		}
	}
	return name
}

// IsNiftiName reports whether a file name looks like a NIfTI image.
func IsNiftiName(name string) bool {
	lower := strings.ToLower(name)
	return strings.HasSuffix(lower, ".nii") || strings.HasSuffix(lower, ".nii.gz")
}

func readMaybeGzip(path string) ([]byte, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if len(raw) >= 2 && raw[0] == 0x1f && raw[1] == 0x8b {
		zr, err := gzip.NewReader(bytes.NewReader(raw))
		if err != nil {
			return nil, fmt.Errorf("%s: %w", filepath.Base(path), err)
		}
		defer zr.Close()
		out, err := io.ReadAll(zr)
		if err != nil {
			return nil, fmt.Errorf("%s: damaged gzip data (%w)", filepath.Base(path), err)
		}
		return out, nil
	}
	return raw, nil
}

// LoadNifti reads a NIfTI-1 file (.nii, .nii.gz, or an .hdr/.img pair) as
// float32, applying the scale factors of the header.
func LoadNifti(path string) (*Volume, error) {
	name := filepath.Base(path)
	hdrPath, imgPath := path, ""
	lower := strings.ToLower(path)
	switch {
	case strings.HasSuffix(lower, ".img"):
		hdrPath, imgPath = path[:len(path)-4]+".hdr", path
	case strings.HasSuffix(lower, ".hdr"):
		imgPath = path[:len(path)-4] + ".img"
	}
	buf, err := readMaybeGzip(hdrPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("%s: file not found", path)
		}
		return nil, err
	}
	return decodeNifti(buf, name, imgPath)
}

// decodeNifti decodes a NIfTI-1 file held in memory; imgPath is where the
// voxels of an .hdr/.img pair are.
func decodeNifti(buf []byte, name, imgPath string) (*Volume, error) {
	var err error
	if len(buf) < 348 {
		return nil, fmt.Errorf("%s: not a NIfTI image (file too short)", name)
	}
	var bo binary.ByteOrder = binary.LittleEndian
	switch {
	case binary.LittleEndian.Uint32(buf[0:4]) == 348:
	case binary.BigEndian.Uint32(buf[0:4]) == 348:
		bo = binary.BigEndian
	case binary.LittleEndian.Uint32(buf[0:4]) == 540 || binary.BigEndian.Uint32(buf[0:4]) == 540:
		return nil, fmt.Errorf("%s: NIfTI-2 files are not supported; convert the image to NIfTI-1", name)
	default:
		return nil, fmt.Errorf("%s: not a NIfTI image", name)
	}
	magic := string(buf[344:347])
	if magic != "n+1" && magic != "ni1" {
		return nil, fmt.Errorf("%s: Analyze 7.5 images are not supported; convert the image to NIfTI", name)
	}
	i16 := func(off int) int { return int(int16(bo.Uint16(buf[off:]))) }
	f32 := func(off int) float64 { return float64(math.Float32frombits(bo.Uint32(buf[off:]))) }

	ndim := i16(40)
	if ndim < 1 || ndim > 7 {
		return nil, fmt.Errorf("%s: damaged NIfTI header", name)
	}
	dims := [8]int{}
	for i := 1; i <= 7; i++ {
		dims[i] = 1
		if i <= ndim {
			dims[i] = i16(40 + 2*i)
		}
		if dims[i] < 1 {
			dims[i] = 1
		}
	}
	// Trailing dimensions of length one do not count.
	for ndim > 3 && dims[ndim] == 1 {
		ndim--
	}
	if ndim > 4 {
		return nil, fmt.Errorf("%s: expected a 3D image, got %d dimensions", name, ndim)
	}
	vol := &Volume{Dim: [3]int{dims[1], dims[2], dims[3]}, NT: dims[4]}
	total := vol.NVox() * vol.NT

	// Affine, chosen as nibabel does: sform, else qform, else from the voxel sizes.
	var pixdim [8]float64
	for i := range pixdim {
		pixdim[i] = f32(76 + 4*i)
	}
	qcode, scode := i16(252), i16(254)
	switch {
	case scode != 0:
		for r := 0; r < 3; r++ {
			for c := 0; c < 4; c++ {
				vol.Aff[r][c] = f32(280 + 16*r + 4*c)
			}
		}
		vol.Aff[3][3] = 1
	case qcode != 0:
		b, c, d := f32(256), f32(260), f32(264)
		a := math.Sqrt(math.Max(0, 1-(b*b+c*c+d*d)))
		rot := [3][3]float64{
			{a*a + b*b - c*c - d*d, 2 * (b*c - a*d), 2 * (b*d + a*c)},
			{2 * (b*c + a*d), a*a + c*c - b*b - d*d, 2 * (c*d - a*b)},
			{2 * (b*d - a*c), 2 * (c*d + a*b), a*a + d*d - b*b - c*c},
		}
		zoom := [3]float64{pixdim[1], pixdim[2], pixdim[3]}
		if pixdim[0] < 0 {
			zoom[2] = -zoom[2]
		}
		for r := 0; r < 3; r++ {
			for col := 0; col < 3; col++ {
				vol.Aff[r][col] = rot[r][col] * zoom[col]
			}
			vol.Aff[r][3] = f32(268 + 4*r)
		}
		vol.Aff[3][3] = 1
	default:
		// No orientation in the header: voxel sizes around the centre, x flipped.
		for r := 0; r < 3; r++ {
			zoom := pixdim[r+1]
			if zoom == 0 {
				zoom = 1
			}
			if r == 0 {
				zoom = -zoom
			}
			vol.Aff[r][r] = zoom
			vol.Aff[r][3] = -zoom * float64(vol.Dim[r]-1) / 2
		}
		vol.Aff[3][3] = 1
	}
	vol.Descrip = strings.TrimRight(string(buf[148:228]), "\x00 ")

	data := buf
	offset := int(f32(108))
	if magic == "ni1" {
		if imgPath == "" {
			return nil, fmt.Errorf("%s: this header belongs to an .hdr/.img pair; open the .img file", name)
		}
		if data, err = readMaybeGzip(imgPath); err != nil {
			return nil, err
		}
	} else if offset < 352 {
		offset = 352
	}
	datatype := i16(70)
	size := map[int]int{2: 1, 4: 2, 8: 4, 16: 4, 64: 8, 256: 1, 512: 2, 768: 4, 1024: 8, 1280: 8}[datatype]
	if size == 0 {
		return nil, fmt.Errorf("%s: unsupported data type (code %d)", name, datatype)
	}
	if offset < 0 || len(data) < offset+total*size {
		return nil, fmt.Errorf("%s: the file is shorter than its header says", name)
	}
	data = data[offset : offset+total*size]

	slope32 := math.Float32frombits(bo.Uint32(buf[112:]))
	inter32 := math.Float32frombits(bo.Uint32(buf[116:]))
	slope, inter := float64(slope32), float64(inter32)
	if slope == 0 || math.IsNaN(slope) || math.IsInf(slope, 0) {
		slope, slope32 = 1, 1
	}
	if math.IsNaN(inter) || math.IsInf(inter, 0) {
		inter, inter32 = 0, 0
	}
	scaled := slope != 1 || inter != 0

	out := make([]float32, total)
	// NumPy scales small integers and float32 in single precision and
	// everything else in double precision; do the same.
	single := func(raw float32) float32 {
		if !scaled {
			return raw
		}
		return float32(raw*slope32) + inter32
	}
	double := func(raw float64) float32 {
		if !scaled {
			return float32(raw)
		}
		return float32(float64(raw*slope) + inter)
	}
	parallelFor(total, func(lo, hi int) {
		for i := lo; i < hi; i++ {
			p := data[i*size:]
			switch datatype {
			case 2:
				out[i] = single(float32(p[0]))
			case 256:
				out[i] = single(float32(int8(p[0])))
			case 4:
				out[i] = single(float32(int16(bo.Uint16(p))))
			case 512:
				out[i] = single(float32(bo.Uint16(p)))
			case 16:
				out[i] = single(math.Float32frombits(bo.Uint32(p)))
			case 8:
				out[i] = double(float64(int32(bo.Uint32(p))))
			case 768:
				out[i] = double(float64(bo.Uint32(p)))
			case 1024:
				out[i] = double(float64(int64(bo.Uint64(p))))
			case 1280:
				out[i] = double(float64(bo.Uint64(p)))
			case 64:
				out[i] = double(math.Float64frombits(bo.Uint64(p)))
			}
		}
	})
	vol.Data = out
	return vol, nil
}

// LoadImage reads a 3D PET image (a 4D image with a single frame is accepted).
func LoadImage(path string) (*Volume, error) {
	vol, err := LoadNifti(path)
	if err != nil {
		return nil, err
	}
	if vol.NT != 1 {
		return nil, fmt.Errorf("%s: expected a 3D image, got shape (%d, %d, %d, %d)",
			path, vol.Dim[0], vol.Dim[1], vol.Dim[2], vol.NT)
	}
	return vol, nil
}

// SaveNifti writes a volume as NIfTI-1, gzipped when the name ends in .gz.
// With asUint8 the values are stored as bytes (for masks), else as float32.
func SaveNifti(path string, vol *Volume, asUint8 bool) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	hdr := make([]byte, 352)
	le := binary.LittleEndian
	put16 := func(off int, v int) { le.PutUint16(hdr[off:], uint16(int16(v))) }
	putf := func(off int, v float64) { le.PutUint32(hdr[off:], math.Float32bits(float32(v))) }
	le.PutUint32(hdr[0:], 348)
	ndim := 3
	if vol.NT > 1 {
		ndim = 4
	}
	put16(40, ndim)
	put16(42, vol.Dim[0])
	put16(44, vol.Dim[1])
	put16(46, vol.Dim[2])
	put16(48, vol.NT)
	put16(50, 1)
	put16(52, 1)
	put16(54, 1)
	if asUint8 {
		put16(70, 2)
		put16(72, 8)
	} else {
		put16(70, 16)
		put16(72, 32)
	}
	zooms := VoxelSizes(vol.Aff)
	putf(76, 1)
	for i := 0; i < 3; i++ {
		putf(80+4*i, zooms[i])
	}
	putf(92, 1)
	putf(108, 352)
	putf(112, 1) // scl_slope
	hdr[123] = 2 // millimetres
	copy(hdr[148:228], vol.Descrip)
	put16(254, 2) // sform: aligned
	for r := 0; r < 3; r++ {
		for c := 0; c < 4; c++ {
			putf(280+16*r+4*c, vol.Aff[r][c])
		}
	}
	copy(hdr[344:], "n+1\x00")

	file, err := os.Create(path)
	if err != nil {
		return err
	}
	var w io.Writer = file
	var zw *gzip.Writer
	if strings.HasSuffix(strings.ToLower(path), ".gz") {
		zw, _ = gzip.NewWriterLevel(file, gzip.BestSpeed)
		w = zw
	}
	bw := bufio.NewWriterSize(w, 1<<20)
	if _, err = bw.Write(hdr); err == nil {
		chunk := make([]byte, 0, 1<<16)
		for i, v := range vol.Data {
			if asUint8 {
				chunk = append(chunk, byte(v))
			} else {
				chunk = le.AppendUint32(chunk, math.Float32bits(v))
			}
			if len(chunk) >= 1<<16-4 || i == len(vol.Data)-1 {
				if _, err = bw.Write(chunk); err != nil {
					break
				}
				chunk = chunk[:0]
			}
		}
	}
	if err == nil {
		err = bw.Flush()
	}
	if zw != nil {
		if cerr := zw.Close(); err == nil {
			err = cerr
		}
	}
	if cerr := file.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		os.Remove(path)
	}
	return err
}

// SaveMask writes a boolean mask as a byte image.
func SaveMask(path string, mask []bool, dim [3]int, aff Affine, descrip string) error {
	vol := NewVolume(dim, aff)
	vol.Descrip = descrip
	for i, m := range mask {
		if m {
			vol.Data[i] = 1
		}
	}
	return SaveNifti(path, vol, true)
}
