package fractal

import (
	"math"

	"gigashad/shad"
)

// StepFog is Mandelbulb3D's "dynamic fog": glow from how long a ray spends
// near the surface on its way to a hit. Rays grazing edges, slipping through
// gaps or skimming thin structures run up a high count and light up; rays
// that hit head-on stay dark.
//
// The count is Mandelbulb3D's number of march steps, each a tenth of the
// distance estimate, i.e. 10·∫dt/DE along the ray. That is dimensionless, so
// the same settings work at any scale.
type StepFog struct {
	Color, Dense Vec3    // the fog's color, and the one it turns to where it's densest
	Threshold    float64 // count below which there's no fog
	DepthSlope   float64 // extra threshold per observer size of distance to the hit
	Gain         float64 // fog per count above the threshold (clamped to 0..1)
	DenseRate    float64 // fraction of Dense per count
	// Floor, in observer sizes, is the closest distance the count tells
	// apart. A ray stops marching within a pixel's footprint of the
	// surface, and the count of its approach depends on where it stops, so
	// without a floor the fog would change with the render resolution (and
	// flash whenever it changes). Closer distances count as Floor, and on a
	// hit the rest of the approach down to Floor is added as if marched.
	Floor float64
	// Reach, in observer sizes, if set, fades out the count from distances
	// well beyond it: rays brushing a surface glow, rays that only pass
	// within a moderate distance of something for a long stretch don't.
	// Some distance estimates sit at a moderate value over regions with no
	// surface (phantoms), which would otherwise glow like glass.
	Reach float64
}

type Vec3 = shad.Vec3

// stepCount is the count gathered over a march step of length step, from
// distance d0 to d1 from the surface. It integrates 10/DE assuming DE
// changes linearly over the step (its log-mean), so it doesn't depend on how
// coarsely the ray marches.
func stepCount(step, d0, d1, reach float64) float64 {
	d0, d1 = math.Max(d0, 1e-300), math.Max(d1, 1e-300)
	dm := d0
	if r := d0 / d1; math.Abs(r-1) > 1e-6 {
		dm = (d0 - d1) / math.Log(r)
	}
	c := 10 * step / dm
	if reach > 0 {
		c *= math.Exp(-dm / reach)
	}
	return c
}

// approach is the count a ray at distance d from the surface would add
// marching straight on down to floor: 10·ln(d/floor).
func approach(d, floor float64) float64 {
	if d <= floor || floor <= 0 {
		return 0
	}
	return 10 * math.Log(d/floor)
}

// color is the fog added to a pixel whose ray hit after count, at tRel
// observer sizes away.
func (sf *StepFog) color(count, tRel float64) Vec3 {
	fog := shad.Clamp((count-sf.Threshold-sf.DepthSlope*tRel)*sf.Gain, 0, 1)
	dense := shad.Clamp(math.Min(1, count*sf.DenseRate)*fog, 0, 1)
	return sf.Color.Mul(fog - dense).Add(sf.Dense.Mul(dense))
}

func (sf *StepFog) uniforms(u map[string]any) {
	var s StepFog
	on := float32(0)
	if sf != nil {
		s, on = *sf, 1
	}
	u["FogOn"] = on
	u["FogColor"], u["FogDense"] = shad.Vec3f(s.Color), shad.Vec3f(s.Dense)
	u["FogThreshold"], u["FogDepthSlope"] = float32(s.Threshold), float32(s.DepthSlope)
	u["FogGain"], u["FogDenseRate"] = float32(s.Gain), float32(s.DenseRate)
	u["FogFloor"] = float32(s.Floor)
	u["FogReach"] = float32(s.Reach)
}
