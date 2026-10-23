package fractal

import (
	"math"

	"gigashad/shad"
)

// kageExplorer is Explorer.Fragment for the GPU. The scene's Kage code
// (Explorer.GPU) must define, mirroring its Go Formula and Material:
//
//	func de(p vec3, iters int, trapFrom int) (float, vec4)
//	func itersFor(eps float) int
//	func material(pos, normal, dir vec3, trap vec4, ao, glow float) vec3
//	func background(dir vec3, glow float) vec3
const kageExplorer = `
var Detail float
var ViewDist float
var Steps float
var StepMul float
var PixelLOD float
var TrapFrom float
var Fog vec3

func normalAt(p vec3, eps float, iters int) vec3 {
	h := eps * 0.5
	a := vec3(1, -1, -1)
	b := vec3(-1, -1, 1)
	c := vec3(-1, 1, -1)
	e := vec3(1, 1, 1)
	da, _ := de(p+a*h, iters, 0)
	db, _ := de(p+b*h, iters, 0)
	dc, _ := de(p+c*h, iters, 0)
	dd, _ := de(p+e*h, iters, 0)
	return normalize(a*da + b*db + c*dc + e*dd)
}

func aoAt(p vec3, n vec3, eps float, iters int) float {
	h1 := eps * 4
	h2 := eps * 24
	d1, _ := de(p+n*h1, iters, 0)
	d2, _ := de(p+n*h2, iters, 0)
	occ := max(0, h1-d1)/h1 + 0.5*max(0, h2-d2)/h2
	return clamp(1-occ*0.8, 0, 1)
}

func Fragment(dstPos vec4, srcPos vec2, color vec4) vec4 {
	rd := camRay(fragCoord(dstPos))
	ro := CamPos
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
		d, trap = de(ro+rd*t, itersFor(eps), trapFrom)
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
	p := ro + rd*t
	iters := itersFor(eps)
	n := normalAt(p, eps, iters)
	col := material(p, n, rd, trap, aoAt(p, n, eps, iters), glow/steps)
	return vec4(mix(col, bg, fade), 1)
}
`

// Kage implements shad.GPUShader.
func (e *Explorer) Kage() []byte {
	if e.GPU == nil {
		return nil
	}
	return append([]byte(kageExplorer), e.GPU...)
}

// Uniforms implements shad.GPUUniforms.
func (e *Explorer) Uniforms(f *shad.Frame, u map[string]any) {
	u["Detail"] = float32(e.Detail)
	u["ViewDist"] = float32(e.ViewDist)
	u["Steps"] = float32(e.Steps)
	u["StepMul"] = float32(e.StepMul)
	u["PixelLOD"] = float32(e.PixelLOD)
	u["TrapFrom"] = float32(e.trapFrom(f.Cam.Scale))
	u["Fog"] = shad.Vec3f(e.Fog)
}

// Handoff implements shad.GPUHandoff: it falls from 1 to 0 over the decade of
// scale above GPUMinScale.
func (e *Explorer) Handoff(f *shad.Frame) float64 {
	if e.GPUMinScale <= 0 {
		return 1
	}
	return math.Log10(f.Cam.Scale / e.GPUMinScale)
}

// PreferGPU implements shad.GPUPreferrer: the GPU while its 32-bit floats can
// still resolve the detail the observer sees, the CPU below GPUMinScale.
func (e *Explorer) PreferGPU(f *shad.Frame) bool {
	return e.GPU != nil && f.Cam.Scale >= e.GPUMinScale
}
