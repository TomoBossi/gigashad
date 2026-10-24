// Package explorer is an animated kleinian-style fractal rendered with
// layered, translucent near-misses, adapted from "Fractal Explorer" by David
// Hoskins (https://www.shadertoy.com/view/4s3GW2).
package explorer

import (
	_ "embed"
	"math"
	"sync/atomic"

	"github.com/hajimehoshi/ebiten/v2"

	"gigashad/shad"
)

//go:embed explorer.kage
var kage []byte

type scene struct {
	fly  shad.Flyer
	time atomic.Uint64 // Frame.Time as float64 bits, for the field
}

// Register: the explorer shader has no flags of its own.
func Register(shad.Flags) func() shad.Def { return New }

// New renders on the GPU unless -backend cpu is given; the animated fractal
// doubles as collisions.
func New() shad.Def {
	s := &scene{}
	s.fly.Keys = shad.DefaultFlyKeys()
	s.fly.Keys.Down = shad.Keys(ebiten.KeyMeta)
	s.reset()
	return shad.Def{
		CPU:     shader{},
		GPU:     &shad.GPU{Variants: []shad.Kage{{Source: kage}}},
		Backend: func(*shad.Frame) shad.Backend { return shad.OnGPU },
		FPS:     60, // as the original ran
		Field:   field{s},
		Update: func(c *shad.Context) {
			s.time.Store(math.Float64bits(c.Frame.Time))
			s.fly.Update(c)
		},
		Data: s.fly.Data,
		Bindings: append(append(shad.StandardBindings(), shad.Binding{
			Keys: []shad.Key{ebiten.KeyR}, Help: "reset to the startup state",
			Action: func(c *shad.Context) { s.reset() },
		}), s.fly.ZoomBindings(0.3)...),
	}
}

func (s *scene) reset() {
	p := shad.DefaultPlayer()
	p.Pos = shad.V3(-13, 3, 2.5)
	p.Fwd = shad.V3(0, 0, 1)
	p.Scale = 0.2
	p.Speed = 7.5                 // about 4.5 units a second, the original's pace
	p.FovY = 2 * math.Atan(1/1.2) // the original's focal length of 1.2
	s.fly.Player = p
}

// field is the fractal at the current time.
type field struct{ s *scene }

func (f field) Dist(p shad.Vec3, _ float64) float64 {
	return dist(p, math.Float64frombits(f.s.time.Load()))
}

var csize = shad.V3(1, 1, 1.3)

func clampV(p, lo shad.Vec3) shad.Vec3 {
	return shad.V3(shad.Clamp(p.X, -lo.X, lo.X), shad.Clamp(p.Y, -lo.Y, lo.Y), shad.Clamp(p.Z, -lo.Z, lo.Z))
}

// dist is the distance estimate at p and time t.
func dist(p shad.Vec3, t float64) float64 {
	p = shad.V3(p.X, p.Z, p.Y)
	scale := 1.0
	for range 12 {
		p = clampV(p, csize).Mul(2).Sub(p)
		r2 := p.Dot(p.Add(shad.S3(math.Sin(p.Z * 0.3 * (0.5 + 0.5*math.Sin(t))))))
		k := math.Max(2/r2, 0.027)
		p = p.Mul(k)
		scale *= k
	}
	l := math.Hypot(p.X, p.Y)
	rxy := l - 1.2 + 0.2*math.Sin(t)
	n := l * p.Z
	rxy = math.Max(rxy, -n/4)
	return rxy / math.Abs(scale)
}

func hash(x, y float64) float64 {
	p3 := shad.V3(shad.Fract(x*0.1031), shad.Fract(y*0.11369), shad.Fract(x*0.13787))
	p3 = p3.Add(shad.S3(p3.Dot(shad.V3(p3.Y, p3.Z, p3.X).Add(shad.S3(19.19)))))
	return shad.Fract((p3.X + p3.Y) * p3.Z)
}

func sphereRadius(t, resY float64) float64 {
	t = t * 0.01 * (400 / resY)
	return t*t + 0.005
}

func normal(p shad.Vec3, r, t float64) shad.Vec3 {
	e := r * 0.5
	return shad.V3(
		dist(p.Add(shad.V3(e, 0, 0)), t)-dist(p.Sub(shad.V3(e, 0, 0)), t),
		dist(p.Add(shad.V3(0, e, 0)), t)-dist(p.Sub(shad.V3(0, e, 0)), t),
		dist(p.Add(shad.V3(0, 0, e)), t)-dist(p.Sub(shad.V3(0, 0, e)), t),
	).Norm()
}

var sunLight = shad.V3(0.4, 0.4, 0.3)

func shadow(ro, rd shad.Vec3, t float64) float64 {
	res, s := 1.0, 0.05
	for range 15 {
		h := dist(ro.Add(rd.Mul(s)), t)
		res = math.Min(5*h/s, res)
		s += h + 0.01
	}
	return math.Max(res, 0)
}

var background = shad.V3(0.15, 0.15, 0.17)

func lighting(mat, normal, eyeDir shad.Vec3, d, sh float64) shad.Vec3 {
	sun := sunLight.Norm()
	warm := shad.V3(1, 0.9, 0.85)
	col := mat.MulV(warm).Mul(math.Max(sun.Dot(normal), 0) * sh)
	r := eyeDir.Sub(normal.Mul(2 * normal.Dot(eyeDir)))
	col = col.Add(warm.Mul(math.Pow(math.Max(sun.Dot(r), 0), 12) * 0.5 * sh))
	col = col.Add(mat.Mul(0.2 * math.Max(r.Y, 0.2)))
	return background.Mix(col, math.Min(math.Exp(-d*d*0.015), 1))
}

func postEffects(rgb shad.Vec3, x, y float64) shad.Vec3 {
	rgb = rgb.Pow(0.45)
	grey := shad.S3(shad.V3(0.2125, 0.7154, 0.0721).Dot(rgb.Mul(1.3)))
	rgb = shad.S3(0.5).Mix(grey.Mix(rgb.Mul(1.3), 1.3), 1.4)
	rgb = rgb.Mul(0.4 + 0.6*math.Pow(180*x*y*(1-x)*(1-y), 0.35))
	return rgb.Clamp01()
}

// maxHits is how many near misses along a ray are blended.
const maxHits = 8

type shader struct{}

func (shader) Fragment(frag shad.Vec2, f *shad.Frame, out *shad.Sample) {
	cam := &f.Cam
	rd := cam.Ray(frag, f.Res)
	ro := cam.Pos
	time := f.Time

	// March, remembering the last maxHits near misses: where the estimate
	// fell within a sphere that grows with distance.
	var hitT, hitDE [maxHits]float64
	n := 0
	t := 0.05 * hash(frag.X, frag.Y)
	for j := 0; j < 120 && n < maxHits && t <= 14; j++ {
		de := dist(ro.Add(rd.Mul(t)), time)
		if de < sphereRadius(t, f.Res.Y) {
			hitT[n], hitDE[n] = t, de-0.001
			n++
		}
		t += de
	}

	col := shad.Vec3{}
	alphaAcc := 0.0
	if n > 0 {
		// Shadow from the last near miss, then blend front to back.
		sha := shadow(ro.Add(rd.Mul(hitT[n-1])), sunLight, time)
		for i := range n {
			d := hitT[i]
			r := sphereRadius(d, f.Res.Y)
			alpha := (1 - alphaAcc) * math.Min((r-hitDE[i])/r, 1)
			pos := ro.Add(rd.Mul(d))
			nrm := normal(pos, r, time)
			q := shad.V3(pos.Y, pos.Z, pos.X).Norm()
			alb := cam.Fwd.Div(10).Add(q.Div(10).MulV(shad.V3(0.4, 0.2, 0.2)))
			col = col.Add(lighting(alb, nrm, rd, d, sha).Mul(alpha))
			alphaAcc += alpha
		}
		p := ro.Add(rd.Mul(hitT[0]))
		out.Depth, out.Pos, out.Normal, out.ID = hitT[0], p, normal(p, sphereRadius(hitT[0], f.Res.Y), time), 1
	}
	col = col.Add(background.Mul(shad.Clamp(1-alphaAcc, 0, 1)))
	out.Color = postEffects(col, frag.X/f.Res.X, frag.Y/f.Res.Y).Mul(shad.Smoothstep(0, 2, time))
}
