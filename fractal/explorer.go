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
	// iters iterations, and returns an orbit trap for coloring gathered from
	// iteration trapFrom on (or from the last iterations, if p escapes
	// sooner).
	DE(p shad.Vec3, iters, trapFrom int) (dist float64, trap shad.Vec4)
	// Iters returns how many iterations resolve features of size eps.
	Iters(eps float64) int
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
	// TrapRef is the observer size whose coloring uses the orbit trap from
	// the first iteration. Smaller observers skip one iteration per level of
	// detail they gained, so coloring always describes structure at their own
	// scale instead of structure far larger than them.
	TrapRef float64

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
	d, _ := e.F.DE(p, e.F.Iters(detail), 0)
	return d
}

func (e *Explorer) Fragment(frag shad.Vec2, f *shad.Frame, out *shad.Sample) {
	cam := &f.Cam
	rd := cam.Ray(frag, f.Res)
	ro := cam.Pos
	s := cam.Scale
	minEps := e.Detail * s
	cone := cam.PixelAngle(f.Res) * e.PixelLOD
	maxT := e.ViewDist * s
	trapFrom := e.trapFrom(s)

	t := 0.0
	glow := 0.0
	hit := false
	var d, eps float64
	var trap shad.Vec4
	steps := 0
	for steps < e.Steps {
		steps++
		eps = math.Max(minEps, t*cone)
		d, trap = e.F.DE(ro.Add(rd.Mul(t)), e.F.Iters(eps), trapFrom)
		if d < eps {
			hit = true
			break
		}
		glow += eps / (d + eps)
		t += d * e.StepMul
		if t > maxT {
			break
		}
	}

	fade := shad.Smoothstep(0.35, 1, t/maxT)
	bg := e.Fog
	if e.Background != nil {
		bg = e.Background(rd, glow/float64(steps), f)
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
	n := e.normal(p, eps, iters)
	h := Hit{
		Pos: p, Normal: n, Dir: rd, T: t, TRel: t / s, Trap: trap,
		AO: e.ao(p, n, eps, iters), Steps: steps, Glow: glow / float64(steps),
	}
	col := e.Material(&h, f)
	out.Color = col.Mix(bg, fade)
	out.Depth = t
	out.Pos = p
	out.Normal = n
	out.Steps = steps
	out.ID = 1
}

// trapFrom is the first iteration the orbit trap covers for an observer of
// size s (see TrapRef).
func (e *Explorer) trapFrom(s float64) int {
	return max(0, e.F.Iters(s)-e.F.Iters(e.TrapRef))
}

// normal uses the tetrahedron technique: four DE samples instead of six.
func (e *Explorer) normal(p shad.Vec3, eps float64, iters int) shad.Vec3 {
	h := eps * 0.5
	k := [4]shad.Vec3{shad.V3(1, -1, -1), shad.V3(-1, -1, 1), shad.V3(-1, 1, -1), shad.V3(1, 1, 1)}
	var n shad.Vec3
	for _, v := range k {
		d, _ := e.F.DE(p.Add(v.Mul(h)), iters, 0)
		n = n.Add(v.Mul(d))
	}
	return n.Norm()
}

// ao compares the DE a few hit-thresholds out along the normal with the
// distance actually travelled: occluded points see the surface sooner.
func (e *Explorer) ao(p, n shad.Vec3, eps float64, iters int) float64 {
	occ := 0.0
	w := 1.0
	for _, k := range [2]float64{4, 24} {
		h := eps * k
		d, _ := e.F.DE(p.Add(n.Mul(h)), iters, 0)
		occ += w * math.Max(0, h-d) / h
		w *= 0.5
	}
	return shad.Clamp(1-occ*0.8, 0, 1)
}

// LogIters is the usual Iters for an escape-time IFS: base iterations plus
// enough to shrink features of size eps to unit size when each iteration
// scales by k.
func LogIters(eps, k float64, base, maxIters int) int {
	n := base + int(math.Ceil(math.Log(1/eps)/math.Log(k)))
	return max(1, min(n, maxIters))
}
