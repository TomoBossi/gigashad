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
	Glow        float64 // strength of the dynamic fog glow; 0 turns it off
}

func DefaultOptions() Options { return Options{Breath: 1, BreathSpeed: 1, Glow: 1} }

// Register declares Neo Oolacile's flags and returns its constructor.
func Register(fl shad.Flags) func() shad.Def {
	d := DefaultOptions()
	breath := fl.Float64("breath", d.Breath, "how much the fractal breathes; 0 holds it still")
	speed := fl.Float64("breathspeed", d.BreathSpeed, "breathing pace; 1 is the default, 2 twice as fast")
	glow := fl.Float64("glow", d.Glow, "strength of the glow; 0 turns it off")
	return func() shad.Def { return New(Options{Breath: *breath, BreathSpeed: *speed, Glow: *glow}) }
}

func New(opt Options) shad.Def {
	s := &scene{e: newExplorer(), opt: opt}
	if opt.Glow == 0 {
		s.e.StepFog = nil
	} else {
		sf := stepFog
		sf.Gain *= opt.Glow
		s.e.StepFog = &sf
	}

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
	d.Post = &post
	d.CaptureMouse = true // the mouse looks around
	d.Controls = s.fly.Keys.Help()
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
		Fog:             shad.V3(0.01, 0.025, 0.06),
	}
	e.StepFog = &stepFog
	e.Material = material
	e.Background = func(dir shad.Vec3, glow float64, fr *shad.Frame) shad.Vec3 {
		return e.Fog.Add(shad.V3(0.1, 0.5, 0.8).Mul(glow * glow * 0.6))
	}
	return e
}

// startPlayer is the view stored in the original scene file.
func startPlayer() shad.Player {
	p := shad.DefaultPlayer()
	p.Pos, p.Fwd, p.Up = startPos, startFwd, startUp
	p.Scale = 0.002
	p.FovY = 60 * math.Pi / 180
	// Mandelbulb3D turns each ray by an angle proportional to the pixel's
	// distance from the image center: an equidistant camera.
	p.Fisheye = 1
	return p
}

var (
	lightDir = shad.V3(0.3, 0.8, 0.5).Norm()
	teal     = shad.V3(0.045, 0.33, 0.40)
	deep     = shad.V3(0.02, 0.06, 0.16)
	moss     = shad.V3(0.06, 0.30, 0.28)
	cyan     = shad.V3(0.2, 0.9, 1.0)
)

func material(h *fractal.Hit, f *shad.Frame) shad.Vec3 {
	// Base color from the orbit trap: the planes the folds keep returning to
	// pick out the panel-and-circuit look.
	tz := shad.Smoothstep(0, 0.15, h.Trap.Z)
	tx := shad.Smoothstep(0, 0.4, h.Trap.X)
	base := deep.Mix(teal, tz).Mix(moss, 0.6*(1-tx))

	n := h.Normal
	diff := math.Max(0, n.Dot(lightDir))
	head := math.Max(0, -n.Dot(h.Dir))
	spec := math.Pow(math.Max(0, h.Dir.Sub(n.Mul(2*n.Dot(h.Dir))).Dot(lightDir)), 16)

	// A generous ambient term keeps surfaces colored between the glows.
	col := base.Mul(0.33 + 0.6*diff + 0.4*head).Mul(0.2 + 0.8*h.AO)
	col = col.Add(shad.S3(0.3 * spec * h.AO))
	col = col.Add(cyan.Mul(0.15 * h.Glow))
	return col
}

// stepFog is the scene file's dynamic fog: a fog color of RGB 23/203/255 and
// a white second color (not squared: the file leaves that option off), its
// slider positions (fog offset 104, amount 85) turned into a count threshold
// and gain, and a depth term from the fog's Z offset, in observer sizes of
// the scene's own 0.002. Our distance estimates and step counts don't match
// Mandelbulb3D's exactly, so gain and threshold are calibrated against the
// original render (with Reach, the count is far smaller than
// Mandelbulb3D's, hence the much higher gain and lower threshold).
var stepFog = fractal.StepFog{
	Color:      shad.V3(23, 203, 255).Div(255),
	Dense:      shad.V3(1, 1, 1),
	Threshold:  0.2 * 1.2 * (137 - math.Sqrt(104)*11.313708) * (2.2 / 0.1) * 0.28,
	DepthSlope: (2.2 / 0.1) * 0.7 / (181.788224833822 + 0.995421852006086) * (128 - 11.313708) * 0.002,
	Gain:       11 * 0.35 * (85 - 53) * 0.1 * 0.00065,
	DenseRate:  0.1 * 0.015,
	Floor:      shad.DetailPerScale,
	// Only rays brushing a surface glow, not ones passing a phantom: some
	// regions report a moderate distance with no surface anywhere near.
	Reach: 0.5,
}

// post: more contrast than the raw render.
var post = shad.Post{Contrast: 1.1}
