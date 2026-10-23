package fractal

import (
	"math"

	"gigashad/shad"
)

// kageExplorer is Explorer.Fragment for the GPU. Positions are double-floats
// (see KagePrelude), split per component into a hi and a lo vec3, so the
// formula can be evaluated far below 32-bit precision. The scene's Kage code
// (Explorer.GPU) must define, mirroring its Go Formula and Material:
//
//	func de(hi, lo vec3, iters int, trapFrom int, eps float) (float, vec4)
//	func itersFor(eps float) int
//	func material(pos, normal, dir vec3, trap vec4, ao, glow float) vec3
//	func background(dir vec3, glow float) vec3
//
// de gets the hit threshold eps so it can do in double-float only as many
// iterations as that much detail needs, and the rest in plain float.
//
// The explorer compiles two variants: a Precise one, in which the
// double-float arithmetic works, and a fast one, in which the compiler folds
// the double-float error terms away and it costs about as much as plain
// float. The fast one renders while the observer
// is at least GPUFastMinScale.
const kageExplorer = `
var Detail float
var ViewDist float
var Steps float
var StepMul float
var PixelLOD float
var TrapFrom float
var Fog vec3

func normalAt(ph, pl vec3, eps float, iters int) vec3 {
	h := eps * 0.5
	a := vec3(1, -1, -1)
	b := vec3(-1, -1, 1)
	c := vec3(-1, 1, -1)
	e := vec3(1, 1, 1)
	qh, ql := dfAdd3(ph, pl, a*h, vec3(0))
	da, _ := de(qh, ql, iters, 0, eps)
	qh, ql = dfAdd3(ph, pl, b*h, vec3(0))
	db, _ := de(qh, ql, iters, 0, eps)
	qh, ql = dfAdd3(ph, pl, c*h, vec3(0))
	dc, _ := de(qh, ql, iters, 0, eps)
	qh, ql = dfAdd3(ph, pl, e*h, vec3(0))
	dd, _ := de(qh, ql, iters, 0, eps)
	return normalize(a*da + b*db + c*dc + e*dd)
}

func aoAt(ph, pl vec3, n vec3, eps float, iters int) float {
	h1 := eps * 4
	h2 := eps * 24
	qh, ql := dfAdd3(ph, pl, n*h1, vec3(0))
	d1, _ := de(qh, ql, iters, 0, eps)
	qh, ql = dfAdd3(ph, pl, n*h2, vec3(0))
	d2, _ := de(qh, ql, iters, 0, eps)
	occ := max(0, h1-d1)/h1 + 0.5*max(0, h2-d2)/h2
	return clamp(1-occ*0.8, 0, 1)
}

func Fragment(dstPos vec4, srcPos vec2, color vec4) vec4 {
	rd := camRay(fragCoord(dstPos))
	minEps := Detail * CamScale
	cone := pixelAngle() * PixelLOD
	maxT := ViewDist * CamScale
	trapFrom := int(TrapFrom)

	t := 0.0
	glow := 0.0
	hit := false
	eps := minEps
	d := 0.0
	trap := vec4(0)
	steps := 0.0
	for i := 0; i < 1024; i++ {
		if steps >= Steps {
			break
		}
		steps += 1
		eps = max(minEps, t*cone)
		ph, pl := dfAdd3(CamPos, CamPosLo, rd*t, vec3(0))
		d, trap = de(ph, pl, itersFor(eps), trapFrom, eps)
		if d < eps {
			hit = true
			break
		}
		glow += eps / (d + eps)
		t += d * StepMul
		if t > maxT {
			break
		}
	}

	fade := smoothstep(0.35, 1, t/maxT)
	bg := background(rd, glow/steps)
	if !hit {
		if steps >= Steps && t < maxT {
			bg = mix(bg, Fog, 1-fade)
		}
		return vec4(bg, 1)
	}
	ph, pl := dfAdd3(CamPos, CamPosLo, rd*t, vec3(0))
	iters := itersFor(eps)
	n := normalAt(ph, pl, eps, iters)
	col := material(ph, n, rd, trap, aoAt(ph, pl, n, eps, iters), glow/steps)
	return vec4(mix(col, bg, fade), 1)
}
`

// kageVariants: fast, then precise.
func (e *Explorer) kageVariants() []shad.Kage {
	src := append([]byte(kageExplorer), e.GPU...)
	return []shad.Kage{{Source: src}, {Source: src, Precise: true}}
}

func (e *Explorer) variant(f *shad.Frame) int {
	if f.Cam.Scale >= e.GPUFastMinScale {
		return 0
	}
	return 1
}

func (e *Explorer) uniforms(f *shad.Frame, u map[string]any) {
	u["Detail"] = float32(e.Detail)
	u["ViewDist"] = float32(e.ViewDist)
	u["Steps"] = float32(e.Steps)
	u["StepMul"] = float32(e.StepMul)
	u["PixelLOD"] = float32(e.PixelLOD)
	u["TrapFrom"] = float32(e.trapFrom(f.Cam.Scale))
	u["Fog"] = shad.Vec3f(e.Fog)
}

// handoff falls from 1 to 0 over the decade of scale above GPUMinScale.
func (e *Explorer) handoff(f *shad.Frame) float64 {
	if e.GPUMinScale <= 0 {
		return 1
	}
	return math.Log10(f.Cam.Scale / e.GPUMinScale)
}

// backend is the GPU while it can resolve what the observer sees, the CPU
// below GPUMinScale.
func (e *Explorer) backend(f *shad.Frame) shad.Backend {
	if f.Cam.Scale >= e.GPUMinScale {
		return shad.OnGPU
	}
	return shad.OnCPU
}
