// Package fractal renders distance-estimated fractals so that they can be
// explored at any scale with constant cost.
//
// Everything the renderer does is measured in units of the observer's size
// (Camera.Scale): the hit threshold, the view distance, the normal and
// ambient-occlusion offsets. A self-similar fractal looks the same at every
// scale, so a scale-relative renderer does the same amount of work at every
// scale: shrinking reveals finer detail, but the world fades out sooner in
// absolute terms and movement (also scale-relative) gets slower.
//
// The one thing that does grow is the iteration count: resolving features of
// size eps takes about log(1/eps)/log(k) iterations for a fractal that
// contracts by k per iteration. Iterations are chosen per ray step from the
// detail actually needed at that distance, so far-away geometry stays cheap.
package fractal

import (
	"math"

	"gigashad/shad"
)

// Formula is a distance estimator.
type Formula interface {
	// DE estimates the distance from p to the fractal surface after at most
	// iters iterations, and returns an orbit trap for coloring.
	//
	// level is the iteration at which the fractal's features are the
	// observer's size (Iters(scale) - Iters(1)). Whatever should look the
	// same at every scale keys off it: which iterations the coloring
	// samples, which ones animate.
	//
	// Both are fractional: the estimate and the trap must change smoothly
	// with them, or zooming makes detail and color pop.
	DE(p shad.Vec3, iters, level float64) (dist float64, trap shad.Vec4)
	// Iters returns how many iterations resolve features of size eps,
	// smoothly in eps.
	Iters(eps float64) float64
}

// Hit describes where a view ray landed, for Material.
type Hit struct {
	Pos, Normal, Dir shad.Vec3
	T                float64 // distance along the ray, absolute units
	TRel             float64 // distance along the ray, in observer sizes
	Trap             shad.Vec4
	AO               float64 // 1 = fully open, 0 = fully occluded
	Steps            int
	Glow             float64 // accumulated near misses along the ray, 0..
}

// Explorer is a Shader and a Field for a Formula.
type Explorer struct {
	F Formula
	// Detail is the hit threshold in observer sizes at the camera; further
	// away the threshold grows with the pixel footprint.
	Detail float64
	// ViewDist is how far rays travel, in observer sizes.
	ViewDist float64
	// Steps caps ray-marching steps per pixel.
	Steps int
	// StepMul scales each step (below 1 for estimators that overshoot).
	StepMul float64
	// PixelLOD multiplies the pixel footprint used as the far hit threshold.
	PixelLOD float64
	// StepFog, if set, adds Mandelbulb3D-style dynamic fog (see StepFog).
	StepFog *StepFog
	// Warp, if set, deforms space smoothly over time (see Warp).
	Warp *Warp

	// GPUUniforms fills in the GPU formula's own uniforms, if it has any.
	GPUUniforms func(f *shad.Frame, u map[string]any)

	// GPU is the optional Kage version of F, Material and Background (see
	// kageExplorer for what it must define). With it, the explorer renders on
	// the GPU while the observer is at least GPUMinScale, and on the CPU,
	// in 64-bit floats, below that.
	GPU         []byte
	GPUMinScale float64
	// GPUFastMinScale is the smallest observer the GPU renders with fast,
	// plain-float math; below it, it switches to precise double-float.
	GPUFastMinScale float64

	Material   func(h *Hit, f *shad.Frame) shad.Vec3 // surface color
	Background func(dir shad.Vec3, glow float64, f *shad.Frame) shad.Vec3
	Fog        shad.Vec3 // color rays fade into near ViewDist
}

// Def returns the rendering half of a shader definition: CPU and GPU
// versions, the choice between them, and the field. The scene adds the rest
// (camera, bindings).
func (e *Explorer) Def() shad.Def {
	d := shad.Def{
		CPU:    e,
		Field:  e,
		Static: func() bool { return true },
	}
	if e.GPU != nil {
		d.GPU = &shad.GPU{
			Variants: e.kageVariants(),
			Variant:  e.variant,
			Uniforms: e.uniforms,
		}
		d.Backend = e.backend
		d.Handoff = e.handoff
	}
	return d
}

// Dist implements shad.Field.
func (e *Explorer) Dist(p shad.Vec3, detail float64) float64 {
	s := detail / shad.DetailPerScale
	sp := e.space(s)
	iters, level := e.F.Iters(detail), e.level(s)
	de := func(q shad.Vec3) float64 {
		d, _ := sp.de(q, iters, level)
		return d
	}
	d := de(p)
	if d > confirmRange*s || d < detail {
		return d
	}
	// Distance estimates can underestimate a lot: some regions report a
	// near-constant distance with no surface anywhere near, which would
	// make invisible walls. Confirm that a surface is really there the way
	// rendering finds one, by marching toward it until the estimate drops
	// below detail; if that doesn't happen within confirmRange, report that
	// far instead.
	h := detail * 0.5
	g := shad.V3(
		de(p.Add(shad.V3(h, 0, 0)))-de(p.Sub(shad.V3(h, 0, 0))),
		de(p.Add(shad.V3(0, h, 0)))-de(p.Sub(shad.V3(0, h, 0))),
		de(p.Add(shad.V3(0, 0, h)))-de(p.Sub(shad.V3(0, 0, h))),
	)
	if g == (shad.Vec3{}) {
		return d
	}
	dir := g.Norm().Neg()
	t := 0.0
	for range confirmSteps {
		dt := de(p.Add(dir.Mul(t)))
		if dt < detail {
			return d // a real surface, about d away
		}
		t += dt
		if t > confirmRange*s {
			break
		}
	}
	return math.Max(d, t)
}

// confirmRange (observer sizes) and confirmSteps bound Dist's check that a
// near surface is real.
const (
	confirmRange = 3.0
	confirmSteps = 32
)

// space evaluates the formula in space as Warp deforms it for an observer of
// size s.
type space struct {
	e     *Explorer
	oct   [warpOctaves]octave
	n     int
	scale float64 // shrinks march steps so they stay safe under the deformation
}

func (e *Explorer) space(s float64) space {
	sp := space{e: e, scale: 1}
	if e.Warp.active() {
		sp.oct, sp.n = e.Warp.octaves(s)
		sp.scale = 1 / (1 + e.Warp.slope(&sp.oct, sp.n))
	}
	return sp
}

func (sp *space) de(p shad.Vec3, iters, level float64) (float64, shad.Vec4) {
	if sp.n > 0 {
		p = p.Add(sp.e.Warp.displace(p, &sp.oct, sp.n))
	}
	d, trap := sp.e.F.DE(p, iters, level)
	return d, trap
}

func (e *Explorer) Fragment(frag shad.Vec2, f *shad.Frame, out *shad.Sample) {
	cam := &f.Cam
	rd := cam.Ray(frag, f.Res)
	ro := cam.Pos
	s := cam.Scale
	minEps := e.Detail * s
	cone := cam.PixelAngle(f.LODRes()) * e.PixelLOD
	maxT := e.ViewDist * s
	level := e.level(s)
	sp := e.space(s)

	t := 0.0
	glow := 0.0
	count := 0.0 // StepFog's
	hit := false
	var d, eps float64
	var trap shad.Vec4
	steps := 0
	prevD, prevStep := 0.0, 0.0
	fogFloor, fogReach := 0.0, 0.0
	if e.StepFog != nil {
		fogFloor, fogReach = e.StepFog.Floor*s, e.StepFog.Reach*s
	}
	for steps < e.Steps {
		steps++
		eps = math.Max(minEps, t*cone)
		d, trap = sp.de(ro.Add(rd.Mul(t)), e.F.Iters(eps), level)
		if steps > 1 {
			count += stepCount(prevStep, math.Max(prevD, fogFloor), math.Max(d, fogFloor), fogReach)
		}
		if d < eps {
			hit = true
			count += approach(d, fogFloor)
			break
		}
		// Hits and draw decisions use the unscaled estimate, as without the
		// warp; only the step shrinks to stay safe in deformed space.
		step := d * e.StepMul * sp.scale
		glow += glowAlong(step, d, eps)
		t += step
		prevD, prevStep = d, step
		if t > maxT {
			break
		}
	}

	fade := shad.Smoothstep(0.35, 1, t/maxT)
	bg := e.Fog
	if e.Background != nil {
		bg = e.Background(rd, glowLevel(glow), f)
	}
	if !hit {
		if steps >= e.Steps && t < maxT {
			// Ran out of steps grazing a surface: treat as fog, not sky.
			bg = bg.Mix(e.Fog, 1-fade)
		}
		out.Color = bg
		out.Steps = steps
		return
	}

	p := ro.Add(rd.Mul(t))
	iters := e.F.Iters(eps)
	n := e.normal(&sp, p, eps, iters, level)
	h := Hit{
		Pos: p, Normal: n, Dir: rd, T: t, TRel: t / s, Trap: trap,
		AO: e.ao(&sp, p, n, eps, iters, level), Steps: steps, Glow: glowLevel(glow),
	}
	col := e.Material(&h, f)
	if e.StepFog != nil {
		col = col.Add(e.StepFog.color(count, t/s))
	}
	out.Color = col.Mix(bg, fade)
	out.Depth = t
	out.Pos = p
	out.Normal = n
	out.Steps = steps
	out.ID = 1
}

// level is the iteration at which features are the size s (see Formula.DE).
func (e *Explorer) level(s float64) float64 {
	return e.F.Iters(s) - e.F.Iters(1)
}

// glowAlong is the glow gathered over a march step of length step at distance
// d from the surface: near misses, measured in pixel footprints (eps), glow.
// It is an integral along the ray, so it doesn't depend on how many steps
// the march happened to take.
func glowAlong(step, d, eps float64) float64 {
	x := d / eps
	return step / eps / (1 + x*x)
}

// glowLevel maps gathered glow to 0..1.
func glowLevel(g float64) float64 {
	return 1 - math.Exp(-g*0.05)
}

// normal uses the tetrahedron technique: four DE samples instead of six.
func (e *Explorer) normal(sp *space, p shad.Vec3, eps, iters, level float64) shad.Vec3 {
	h := eps * 0.5
	k := [4]shad.Vec3{shad.V3(1, -1, -1), shad.V3(-1, -1, 1), shad.V3(-1, 1, -1), shad.V3(1, 1, 1)}
	var n shad.Vec3
	for _, v := range k {
		d, _ := sp.de(p.Add(v.Mul(h)), iters, level)
		n = n.Add(v.Mul(d))
	}
	return n.Norm()
}

// ao compares the DE a few hit-thresholds out along the normal with the
// distance actually travelled: occluded points see the surface sooner.
func (e *Explorer) ao(sp *space, p, n shad.Vec3, eps, iters, level float64) float64 {
	occ := 0.0
	w := 1.0
	for _, k := range [2]float64{4, 24} {
		h := eps * k
		d, _ := sp.de(p.Add(n.Mul(h)), iters, level)
		occ += w * math.Max(0, h-d) / h
		w *= 0.5
	}
	return shad.Clamp(1-occ*0.8, 0, 1)
}

// LogIters is the usual Iters for an escape-time IFS: base iterations plus
// enough to shrink features of size eps to unit size when each iteration
// scales by k.
func LogIters(eps, k, base float64, maxIters int) float64 {
	n := base + math.Log(1/eps)/math.Log(k)
	return math.Max(1, math.Min(n, float64(maxIters)))
}
