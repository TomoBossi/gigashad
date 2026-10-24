package oolacile

import (
	_ "embed"
	"fmt"
	"math"

	"github.com/hajimehoshi/ebiten/v2"

	"gigashad/fractal"
	"gigashad/shad"
)

// The view stored in the original scene file.
var (
	startPos = shad.V3(-0.40801883060470967, 0.4069470499383455, -0.9488125770510608)
	startFwd = shad.V3(-0.861001932153055, -0.13427906547495544, 0.4905556088802561)
	startUp  = shad.V3(-0.4346492903452675, -0.30660142211805147, -0.8468031426238001)
)

//go:embed scene.kage
var kage []byte

// scene is the shader's state: the explorer, the player, and its own clock.
type scene struct {
	e   *fractal.Explorer
	f   *MixPinski
	fly shad.Flyer
	t0  float64 // Frame.Time at the last reset
	opt Options
	now float64 // seconds since the last reset, as of the last Update
}

// Options are Neo Oolacile's own settings, also available as flags.
type Options struct {
	Breath      float64 // how much the fractal breathes; 0 holds it still
	BreathSpeed float64 // how fast; 1 is the default pace
}

func DefaultOptions() Options { return Options{Breath: 1, BreathSpeed: 1} }

// Register declares Neo Oolacile's flags and returns its constructor.
func Register(fl shad.Flags) func() shad.Def {
	d := DefaultOptions()
	breath := fl.Float64("breath", d.Breath, "how much the fractal breathes; 0 holds it still")
	speed := fl.Float64("breathspeed", d.BreathSpeed, "breathing pace; 1 is the default, 2 twice as fast")
	return func() shad.Def { return New(Options{Breath: *breath, BreathSpeed: *speed}) }
}

func New(opt Options) shad.Def {
	s := &scene{e: newExplorer(), opt: opt}
	s.f = s.e.F.(*MixPinski)
	// Breathing: a smooth deformation of space at scales 8-128 observer sizes,
	// which sways the fractal without changing its shape's topology.
	s.e.Warp = &fractal.Warp{Amp: 0.008 * opt.Breath, Speed: opt.BreathSpeed, MinRatio: 8, MaxRatio: 128}
	s.fly.Keys = keys()
	s.reset(0)
	d := s.e.Def()
	d.Update = s.update
	d.Data = s.data
	d.Static = func() bool { return s.opt.Breath == 0 }
	s.e.GPUUniforms = s.uniforms
	d.Bindings = append(shad.StandardBindings(), shad.Binding{
		Keys: []shad.Key{ebiten.KeyR}, Help: "reset to the startup state",
		Action: func(c *shad.Context) { s.reset(c.Frame.Time) },
	})
	return d
}

// keys: WASD, Space/Cmd up/down, Shift fast, mouse and arrows to look, Q/E
// roll, Z/C shrink/grow.
func keys() shad.FlyKeys {
	k := shad.DefaultFlyKeys()
	k.Down = shad.Keys(ebiten.KeyMeta)
	return k
}

func (s *scene) reset(now float64) {
	s.fly.Player = startPlayer()
	s.t0 = now
	s.breathe(0)
}

func (s *scene) update(c *shad.Context) {
	s.fly.Update(c)
	s.breathe(c.Frame.Time - s.t0)
}

// breathe advances the warp to time t (seconds since the last reset).
func (s *scene) breathe(t float64) {
	s.now = t
	s.e.Warp.Time = t
}

func (s *scene) uniforms(f *shad.Frame, u map[string]any) {
}

func (s *scene) data(c *shad.Context) string {
	return s.fly.Data(c) + fmt.Sprintf("Neo Oolacile\n  breath %g, speed %g, t %.2f s\n", s.opt.Breath, s.opt.BreathSpeed, s.now)
}

func newExplorer() *fractal.Explorer {
	e := &fractal.Explorer{
		F:        NeoOolacile(),
		Detail:   0.002,
		ViewDist: 600,
		Steps:    160,
		StepMul:  0.8,
		PixelLOD: 1.2,
		GPU:      kage,
		// The GPU's double-float iterations match the CPU's 64-bit floats
		// down to about 2e-9; below that both start to streak, the CPU a
		// little later.
		GPUMinScale: 5e-10,
		// Plain floats on the GPU match the CPU down to about 2e-5.
		GPUFastMinScale: 5e-5,
		Fog:             shad.V3(0.01, 0.035, 0.04),
	}
	e.Material = material
	e.Background = func(dir shad.Vec3, glow float64, fr *shad.Frame) shad.Vec3 {
		return e.Fog.Add(shad.V3(0.1, 0.6, 0.7).Mul(glow * glow * 0.6))
	}
	return e
}

// startPlayer is the view stored in the original scene file.
func startPlayer() shad.Player {
	p := shad.DefaultPlayer()
	p.Pos, p.Fwd, p.Up = startPos, startFwd, startUp
	p.Scale = 0.002
	p.FovY = 60 * math.Pi / 180
	return p
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
