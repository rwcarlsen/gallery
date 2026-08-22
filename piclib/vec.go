package piclib

import (
	"encoding/binary"
	"math"
)

const (
	DefaultClipMin = 0.2
	DefaultFaceMin = 0.35
)

func EncodeVec(v []float32) []byte {
	b := make([]byte, 4*len(v))
	for i, x := range v {
		binary.LittleEndian.PutUint32(b[i*4:], math.Float32bits(x))
	}
	return b
}

func DecodeVec(b []byte) []float32 {
	n := len(b) / 4
	v := make([]float32, n)
	for i := 0; i < n; i++ {
		v[i] = math.Float32frombits(binary.LittleEndian.Uint32(b[i*4:]))
	}
	return v
}

func L2Normalize(v []float32) {
	var s float32
	for _, x := range v {
		s += x * x
	}
	if s == 0 {
		return
	}
	inv := float32(1 / math.Sqrt(float64(s)))
	for i := range v {
		v[i] *= inv
	}
}

func Cosine(a, b []float32) float32 {
	n := len(a)
	if len(b) < n {
		n = len(b)
	}
	var dot float32
	for i := 0; i < n; i++ {
		dot += a[i] * b[i]
	}
	return dot
}
