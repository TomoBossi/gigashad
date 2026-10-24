package fractal

import (
	"math"

	"gigashad/shad"
)

// Warp gently deforms space around the fractal: the formula is evaluated at
// p + displacement(p, time) instead of p. The displacement is smooth and its
// slope stays well below 1, so the map is a diffeomorphism: the fractal
// sways and stretches but never tears, merges or loses pieces.
//
// It is a sum of a few octaves of slow transverse waves, with wavelengths
// between MinRatio and MaxRatio observer sizes, each fading in and out
// smoothly with the observer's size. Each octave is fixed in world space, so
// moving doesn't drag the pattern along, and octaves much larger than the
// observer have faded out, so coarse structure never sweeps past a tiny one.
type Warp struct {
	Amp                float64 // displacement per octave, as a fraction of its wavelength
	Speed              float64 // 1 is the default pace
	MinRatio, MaxRatio float64 // octave wavelengths, in observer sizes
	Time               float64 // seconds; set every frame
}

// warpOctaves is the most octaves alive at once (log2(MaxRatio/MinRatio)+1).
const warpOctaves = 6

// The three waves of every octave: travel directions, displacement
// directions (perpendicular to travel, so each wave is divergence-free) and
// periods in seconds.
var (
	warpDir    = [3]shad.Vec3{shad.V3(1, 0.3, 0.7).Norm(), shad.V3(-0.4, 1, 0.2).Norm(), shad.V3(0.6, -0.5, 1).Norm()}
	warpDisp   [3]shad.Vec3
	warpPeriod = [3]float64{9.1, 11.3, 13.7}
)

func init() {
	for m := range warpDir {
		warpDisp[m] = warpDir[m].Cross(warpDir[(m+1)%3]).Norm()
	}
}

// octave is one live octave: its wavelength, its weight (0..1, fading with
// the observer's size) and the time-dependent phase of each wave.
type octave struct {
	lambda, weight float64
	phase          [3]float64
}

// octaves returns the octaves alive for an observer of size s, and how
// many there are.
func (w *Warp) octaves(s float64) ([warpOctaves]octave, int) {
	var o [warpOctaves]octave
	n := 0
	lo, hi := math.Log2(w.MinRatio), math.Log2(w.MaxRatio)
	// Octave j has wavelength 2^-j; x is log2 of its ratio to s.
	jmin := int(math.Ceil(math.Log2(1/s) - hi))
	for j := jmin; n < warpOctaves; j++ {
		lambda := math.Exp2(-float64(j))
		x := math.Log2(lambda / s)
		if x < lo {
			break
		}
		if x >= hi {
			continue
		}
		// A smooth bump over [lo, hi): octaves fade in and out as s changes.
		b := math.Sin(math.Pi * (x - lo) / (hi - lo))
		o[n].lambda, o[n].weight = lambda, b*b
		for m := range 3 {
			seed := math.Mod(float64(j)*0.6180339887*float64(m+1)+float64(m)*0.31, 1)
			o[n].phase[m] = 2*math.Pi*seed + 2*math.Pi*w.Time*w.Speed/warpPeriod[m]
		}
		n++
	}
	return o, n
}

// displace returns the displacement at p for an observer of size s.
func (w *Warp) displace(p shad.Vec3, o *[warpOctaves]octave, n int) shad.Vec3 {
	var d shad.Vec3
	for i := range n {
		oc := &o[i]
		k := 2 * math.Pi / oc.lambda
		a := w.Amp * oc.lambda * oc.weight
		for m := range 3 {
			d = d.Add(warpDisp[m].Mul(a * math.Sin(k*warpDir[m].Dot(p)+oc.phase[m])))
		}
	}
	return d
}

// slope bounds how much the displacement can stretch distances, so distance
// estimates can be scaled down to stay safe. (Half of it already makes rays
// overshoot visibly.)
func (w *Warp) slope(o *[warpOctaves]octave, n int) float64 {
	l := 0.0
	for i := range n {
		l += 3 * 2 * math.Pi * w.Amp * o[i].weight
	}
	return l
}

// active reports whether w deforms anything; a nil Warp doesn't.
func (w *Warp) active() bool {
	return w != nil && w.Amp != 0 && w.MaxRatio > w.MinRatio && w.MinRatio > 0
}

// uniforms passes the live octaves to the GPU, with phases taken relative to
// the camera so the GPU only works with small numbers. An inactive Warp
// passes none.
func (w *Warp) uniforms(cam shad.Vec3, s float64, u map[string]any) {
	var o [warpOctaves]octave
	n := 0
	if w.active() {
		o, n = w.octaves(s)
	}
	lambda := make([]float32, warpOctaves)
	amp := make([]float32, warpOctaves)
	phase := make([]float32, 3*warpOctaves)
	for i := range n {
		oc := &o[i]
		lambda[i] = float32(oc.lambda)
		amp[i] = float32(w.Amp * oc.lambda * oc.weight)
		if amp[i] == 0 {
			amp[i] = math.SmallestNonzeroFloat32 // 0 marks an unused octave
		}
		k := 2 * math.Pi / oc.lambda
		for m := range 3 {
			phase[3*i+m] = float32(math.Mod(k*warpDir[m].Dot(cam)+oc.phase[m], 2*math.Pi))
		}
	}
	u["WarpLambda"] = lambda
	u["WarpAmp"] = amp
	u["WarpPhase"] = phase
	scale := 1.0
	if n > 0 {
		scale = 1 / (1 + w.slope(&o, n))
	}
	u["WarpScale"] = float32(scale)
	for m := range 3 {
		u[[3]string{"WarpDir0", "WarpDir1", "WarpDir2"}[m]] = shad.Vec3f(warpDir[m])
		u[[3]string{"WarpDisp0", "WarpDisp1", "WarpDisp2"}[m]] = shad.Vec3f(warpDisp[m])
	}
}
