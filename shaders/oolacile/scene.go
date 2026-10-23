package oolacile

import (
	"math"

	"gigashad/fractal"
	"gigashad/shad"
)

// The view stored in the original scene file.
var (
	startPos = shad.V3(-0.40801883060470967, 0.4069470499383455, -0.9488125770510608)
	startFwd = shad.V3(-0.861001932153055, -0.13427906547495544, 0.4905556088802561)
	startUp  = shad.V3(-0.4346492903452675, -0.30660142211805147, -0.8468031426238001)
)

func New() shad.Shader {
	f := NeoOolacile()
	e := &fractal.Explorer{
		F:        f,
		Detail:   0.002,
		ViewDist: 600,
		Steps:    160,
		StepMul:  0.8,
		PixelLOD: 1.2,
		TrapRef:  0.002,
		Fog:      shad.V3(0.01, 0.035, 0.04),
	}
	e.Material = material
	e.Background = func(dir shad.Vec3, glow float64, fr *shad.Frame) shad.Vec3 {
		return e.Fog.Add(shad.V3(0.1, 0.6, 0.7).Mul(glow * glow * 0.6))
	}
	p := shad.DefaultPlayer()
	p.Pos, p.Fwd, p.Up = startPos, startFwd, startUp
	p.Scale = 0.002
	p.FovY = 60 * math.Pi / 180
	e.Start = p
	return e
}

var (
	lightDir = shad.V3(0.3, 0.8, 0.5).Norm()
	teal     = shad.V3(0.05, 0.32, 0.33)
	deep     = shad.V3(0.02, 0.10, 0.16)
	moss     = shad.V3(0.06, 0.30, 0.20)
	cyan     = shad.V3(0.2, 0.95, 1.0)
)

func material(h *fractal.Hit, f *shad.Frame) shad.Vec3 {
	// Base color from the orbit trap: the planes the folds keep returning to
	// pick out the panel-and-circuit look.
	tz := shad.Smoothstep(0, 0.15, h.Trap.Z)
	tx := shad.Smoothstep(0, 0.4, h.Trap.X)
	base := deep.Mix(teal, tz).Mix(moss, 0.5*(1-tx))

	// Emissive strips where the trap almost touches the y plane.
	emit := math.Exp(-h.Trap.Y*h.Trap.Y*4000) * 1.5

	n := h.Normal
	diff := math.Max(0, n.Dot(lightDir))
	head := math.Max(0, -n.Dot(h.Dir))
	spec := math.Pow(math.Max(0, h.Dir.Sub(n.Mul(2*n.Dot(h.Dir))).Dot(lightDir)), 16)

	col := base.Mul(0.25 + 0.6*diff + 0.4*head).Mul(h.AO)
	col = col.Add(shad.S3(0.3 * spec * h.AO))
	col = col.Add(cyan.Mul(emit))
	col = col.Add(cyan.Mul(0.15 * h.Glow))
	return col
}
