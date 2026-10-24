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
//	func de(hi, lo vec3, iters, level, eps float) (float, vec4)
//	func itersFor(eps float) float
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
var Level float
var Fog vec3

// Warp: octave wavelengths, amplitudes (0 when unused), phases relative to
// the camera, the three waves' directions, and the distance scale that
// keeps estimates safe.
var WarpLambda [6]float
var WarpAmp [6]float
var WarpPhase [18]float
var WarpDir0 vec3
var WarpDir1 vec3
var WarpDir2 vec3
var WarpDisp0 vec3
var WarpDisp1 vec3
var WarpDisp2 vec3
var WarpScale float

// warpAt is Warp.displace at q, relative to the camera.
func warpAt(q vec3) vec3 {
	d := vec3(0)
	for i := 0; i < 6; i++ {
		a := WarpAmp[i]
		if a == 0 {
			continue
		}
		k := 2 * 3.14159265358979 / WarpLambda[i]
		d += WarpDisp0 * (a * sin(k*dot(WarpDir0, q)+WarpPhase[3*i]))
		d += WarpDisp1 * (a * sin(k*dot(WarpDir1, q)+WarpPhase[3*i+1]))
		d += WarpDisp2 * (a * sin(k*dot(WarpDir2, q)+WarpPhase[3*i+2]))
	}
	return d
}

// deAt estimates the distance at q, relative to the camera, in warped space.
func deAt(q vec3, iters float, eps float) (float, vec4) {
	ph, pl := dfAdd3(CamPos, CamPosLo, q+warpAt(q), vec3(0))
	d, trap := de(ph, pl, iters, Level, eps)
	return d, trap
}

func normalAt(q vec3, eps float, iters float) vec3 {
	h := eps * 0.5
	a := vec3(1, -1, -1)
	b := vec3(-1, -1, 1)
	c := vec3(-1, 1, -1)
	e := vec3(1, 1, 1)
	da, _ := deAt(q+a*h, iters, eps)
	db, _ := deAt(q+b*h, iters, eps)
	dc, _ := deAt(q+c*h, iters, eps)
	dd, _ := deAt(q+e*h, iters, eps)
	return normalize(a*da + b*db + c*dc + e*dd)
}

func aoAt(q vec3, n vec3, eps float, iters float) float {
	h1 := eps * 4
	h2 := eps * 24
	d1, _ := deAt(q+n*h1, iters, eps)
	d2, _ := deAt(q+n*h2, iters, eps)
	occ := max(0, h1-d1)/h1 + 0.5*max(0, h2-d2)/h2
	return clamp(1-occ*0.8, 0, 1)
}

func Fragment(dstPos vec4, srcPos vec2, color vec4) vec4 {
	rd := camRay(fragCoord(dstPos))
	minEps := Detail * CamScale
	cone := pixelAngle() * PixelLOD
	maxT := ViewDist * CamScale

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
		d, trap = deAt(rd*t, itersFor(eps), eps)
		if d < eps {
			hit = true
			break
		}
		step := d * StepMul * WarpScale // see Explorer.Fragment
		x := d / eps
		glow += step / eps / (1 + x*x) // see glowAlong
		t += step
		if t > maxT {
			break
		}
	}

	fade := smoothstep(0.35, 1, t/maxT)
	glow = 1 - exp(-glow*0.05) // see glowLevel
	bg := background(rd, glow)
	if !hit {
		if steps >= Steps && t < maxT {
			bg = mix(bg, Fog, 1-fade)
		}
		return vec4(bg, 1)
	}
	q := rd * t
	iters := itersFor(eps)
	n := normalAt(q, eps, iters)
	col := material(CamPos+q, n, rd, trap, aoAt(q, n, eps, iters), glow)
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
	u["Level"] = float32(e.level(f.Cam.Scale))
	u["Fog"] = shad.Vec3f(e.Fog)
	e.Warp.uniforms(f.Cam.Pos, f.Cam.Scale, u)
	if e.GPUUniforms != nil {
		e.GPUUniforms(f, u)
	}
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
