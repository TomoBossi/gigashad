// Package oolacile is "Neo Oolacile", a Mandelbulb3D scene: an alternating
// hybrid of MixPinski4ex (a 4D Sierpinski/Menger IFS) and _SinApprox applied to
// the fourth coordinate, transcribed from the formulas' machine code.
package oolacile

import (
	"math"

	"gigashad/fractal"
	"gigashad/shad"
)

// MixPinski holds the options of MixPinski4ex and _SinApprox as the scene
// sets them.
type MixPinski struct {
	Scale          float64
	CScale         shad.Vec4 // x, y, z, w
	Rot            [16]float64
	RotOff         shad.Vec4
	SinOffset      float64 // _SinApprox: w = sin((w - SinOffset) * SinScale)
	SinScale       float64
	MaxIters       int
	BailoutSquared float64

	rotated bool // Rot is not the identity
}

func NeoOolacile() *MixPinski {
	m := &MixPinski{
		Scale:          2,
		CScale:         shad.V4(1.125, 1, 0.5, 0.5),
		Rot:            [16]float64{1, 0, 0, 0, 0, 1, 0, 0, 0, 0, 1, 0, 0, 0, 0, 1},
		RotOff:         shad.V4(0.5, 0.5, 0.5, 0.5),
		SinOffset:      -0.213281,
		SinScale:       1,
		MaxIters:       240,
		BailoutSquared: 1024,
	}
	m.Prepare()
	return m
}

// Prepare must be called after changing Rot.
func (m *MixPinski) Prepare() {
	m.rotated = m.Rot != [16]float64{1, 0, 0, 0, 0, 1, 0, 0, 0, 0, 1, 0, 0, 0, 0, 1}
}

// Iters: one MixPinski step per two iterations, each scaling by Scale, plus a
// margin for the escape radius.
func (m *MixPinski) Iters(eps float64) int {
	return fractal.LogIters(eps, math.Sqrt(m.Scale), 4, m.MaxIters)
}

// trapIters limits the orbit trap to the first iterations, so coloring follows
// the large structures instead of aliasing on sub-pixel ones.
const trapIters = 10

func (m *MixPinski) DE(p shad.Vec3, iters int) (float64, shad.Vec4) {
	x, y, z, w := p.X, p.Y, p.Z, 0.0
	deriv := 1.0
	trap := shad.V4(1e9, 1e9, 1e9, 1e9)
	r2 := x*x + y*y + z*z
	for i := 0; i < iters; i++ {
		if i&1 == 0 {
			x, y, z, w = m.mixPinski(x, y, z, w)
			deriv *= m.Scale
		} else {
			w = sinApprox((w - m.SinOffset) * m.SinScale)
		}
		r2 = x*x + y*y + z*z + w*w
		if i < trapIters {
			trap.X = min(trap.X, math.Abs(x))
			trap.Y = min(trap.Y, math.Abs(y))
			trap.Z = min(trap.Z, math.Abs(z))
			trap.W = min(trap.W, r2)
		}
		if r2 > m.BailoutSquared {
			break
		}
	}
	return math.Sqrt(r2) / math.Abs(deriv), trap
}

// mixPinski is one MixPinski4ex iteration: Sierpinski folds over all six
// coordinate pairs, a 4D rotation about RotOff, then a Sierpinski scale on x,
// y, w and a Menger scale on z. The original swaps x and y on output, and so
// does this.
func (m *MixPinski) mixPinski(x, y, z, w float64) (float64, float64, float64, float64) {
	if x+y < 0 {
		x, y = -y, -x
	}
	if x+z < 0 {
		x, z = -z, -x
	}
	if y+z < 0 {
		y, z = -z, -y
	}
	if x+w < 0 {
		x, w = -w, -x
	}
	if y+w < 0 {
		y, w = -w, -y
	}
	if z+w < 0 {
		z, w = -w, -z
	}

	if m.rotated {
		x, y, z, w = m.rotate(x, y, z, w)
	}

	s := m.Scale
	k := (s - 1) * m.CScale.Z / s
	ox := s*y - (s-1)*m.CScale.X
	oy := s*x - (s-1)*m.CScale.Y
	oz := s * (k - math.Abs(z-k))
	ow := s*w - (s-1)*m.CScale.W
	return ox, oy, oz, ow
}

// rotate applies the 4D rotation about RotOff. The offsets are applied crossed
// (y's to x and x's to y), as in the original.
func (m *MixPinski) rotate(x, y, z, w float64) (float64, float64, float64, float64) {
	x += m.RotOff.Y
	y += m.RotOff.X
	z += m.RotOff.Z
	w += m.RotOff.W
	r := &m.Rot
	nx := r[0]*x + r[1]*y + r[2]*z + r[3]*w
	ny := r[4]*x + r[5]*y + r[6]*z + r[7]*w
	nz := r[8]*x + r[9]*y + r[10]*z + r[11]*w
	nw := r[12]*x + r[13]*y + r[14]*z + r[15]*w
	x = nx - m.RotOff.Y
	y = ny - m.RotOff.X
	z = nz - m.RotOff.Z
	w = nw - m.RotOff.W
	return x, y, z, w
}

// sinApprox is _SinApprox: range-reduce to [-π/2, π/2] and use the cubic
// a - 4/27·a³, which peaks at ≈0.9967 instead of 1.
func sinApprox(a float64) float64 {
	v := a / (2 * math.Pi)
	t := 2 * (v - math.Floor(v))
	neg := false
	if t > 1 {
		t = 2 - t
		neg = !neg
	}
	a = t * math.Pi
	if a >= math.Pi/2 {
		a -= math.Pi
		neg = !neg
	}
	r := a - a*a*a*(4.0/27.0)
	if neg {
		return -r
	}
	return r
}
