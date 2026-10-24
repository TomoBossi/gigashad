// Package gopher is an endless field of Go gophers nodding their heads,
// adapted from "Gopher Golang" by cepalle (https://www.shadertoy.com/view/wttfDj).
package gopher

import (
	_ "embed"
	"math"

	"github.com/hajimehoshi/ebiten/v2"

	"gigashad/shad"
)

//go:embed gopher.kage
var kage []byte

type scene struct {
	fly shad.Flyer
}

// Register: the gopher shader has no flags of its own.
func Register(shad.Flags) func() shad.Def { return New }

// New renders on the GPU unless -backend cpu is given; the field is the same
// on both, and the player collides with the gophers.
func New() shad.Def {
	s := &scene{}
	s.fly.Keys = shad.DefaultFlyKeys()
	s.fly.Keys.Down = shad.Keys(ebiten.KeyMeta)
	s.reset()
	return shad.Def{
		CPU:     shader{},
		GPU:     &shad.GPU{Variants: []shad.Kage{{Source: kage}}},
		Backend: func(*shad.Frame) shad.Backend { return shad.OnGPU },
		Field:   field{},
		Update:  s.fly.Update,
		Data:    s.fly.Data,
		Bindings: append(shad.StandardBindings(), shad.Binding{
			Keys: []shad.Key{ebiten.KeyR}, Help: "reset to the startup state",
			Action: func(c *shad.Context) { s.reset() },
		}),
	}
}

func (s *scene) reset() {
	p := shad.DefaultPlayer()
	p.Pos = shad.V3(0, 0, -3)
	p.Fwd = shad.V3(0, 0, 1)
	p.Scale = 0.25
	p.FovY = 2 * math.Atan(0.5) // the original's focal length of 1
	s.fly.Player = p
}

// Materials.
const (
	matWhite = 1
	matBlack = 3
	matFur   = 4
	matTan   = 5
)

// field is the gophers as a distance field. Time doesn't enter collisions:
// the nodding is a few degrees.
type field struct{}

func (field) Dist(p shad.Vec3, _ float64) float64 {
	d, _ := distMat(p, 0)
	return d
}

type shader struct{}

func (shader) Fragment(frag shad.Vec2, f *shad.Frame, out *shad.Sample) {
	cam := &f.Cam
	rd := cam.Ray(frag, f.Res)
	ro := cam.Pos
	col := shad.V3(0.4, 0.4, 0.9).Mul(1 - rd.Y)

	t, steps := 0.0, 0
	for ; steps < 256; steps++ {
		ds, _ := distMat(ro.Add(rd.Mul(t)), f.Time)
		t += ds
		if t > 100 || math.Abs(ds) < 0.001 {
			break
		}
	}
	out.Steps = steps

	if t < 100 {
		p := ro.Add(rd.Mul(t))
		n := normal(p, f.Time)
		_, mat := distMat(p, f.Time)
		switch mat {
		case matWhite:
			col = shad.V3(0.9, 0.9, 0.9)
		case matBlack:
			col = shad.V3(0.05, 0.05, 0.05)
		case matFur:
			col = shad.V3(0.1, 0.5, 0.5)
		default:
			col = shad.V3(0.8, 0.6, 0.2)
		}
		sunDir := shad.V3(4, 4, -3).Norm()
		occ := occlusion(p, n, f.Time)
		sun := shad.Clamp(n.Dot(sunDir), 0, 1)
		sky := shad.Clamp(0.5+0.5*n.Y, 0, 1)
		ind := shad.Clamp(n.Dot(sunDir.MulV(shad.V3(-1, 0, -1)).Norm()), 0, 1)
		lin := shad.V3(1.64, 1.27, 0.99).Mul(sun)
		lin = lin.Add(shad.V3(0.16, 0.20, 0.28).Mul(sky * occ))
		lin = lin.Add(shad.V3(0.40, 0.28, 0.20).Mul(ind * occ))
		col = col.MulV(lin)
		out.Depth, out.Pos, out.Normal, out.ID = t, p, n, mat
	}

	col = col.Pow(0.4545)
	fog := 1 - math.Exp(-t*0.1*0.25)
	out.Color = col.Mix(shad.V3(0.95, 0.95, 1.0), shad.Clamp(fog, 0, 1))
}

func normal(p shad.Vec3, time float64) shad.Vec3 {
	d, _ := distMat(p, time)
	const e = 0.001
	dx, _ := distMat(p.Sub(shad.V3(e, 0, 0)), time)
	dy, _ := distMat(p.Sub(shad.V3(0, e, 0)), time)
	dz, _ := distMat(p.Sub(shad.V3(0, 0, e)), time)
	return shad.V3(d-dx, d-dy, d-dz).Norm()
}

func occlusion(p, n shad.Vec3, time float64) float64 {
	occ, sca := 0.0, 1.0
	for i := range 5 {
		h := 0.01 + 0.11*float64(i)/4
		d, _ := distMat(p.Add(n.Mul(h)), time)
		occ += (h - d) * sca
		sca *= 0.95
	}
	return shad.Clamp(1-2*occ, 0, 1)
}

// rot rotates (x, y) the way GLSL's v *= mat2(c, -s, s, c) does.
func rot(x, y, a float64) (float64, float64) {
	s, c := math.Sincos(a)
	return x*c - y*s, x*s + y*c
}

func repeat(v, spacing float64) float64 {
	m := math.Mod(v+0.5*spacing, spacing)
	if m < 0 {
		m += spacing
	}
	return m - 0.5*spacing
}

// smin is a cubic smooth minimum.
func smin(a, b, k float64) float64 {
	h := math.Max(k-math.Abs(a-b), 0) / k
	return math.Min(a, b) - h*h*h*k/6
}

func sdEllipsoid(p, r shad.Vec3) float64 {
	k0 := shad.V3(p.X/r.X, p.Y/r.Y, p.Z/r.Z).Len()
	k1 := shad.V3(p.X/(r.X*r.X), p.Y/(r.Y*r.Y), p.Z/(r.Z*r.Z)).Len()
	return k0 * (k0 - 1) / k1
}

// sdStick is a capsule from a to b whose radius blends from r1 to r2.
func sdStick(p, a, b shad.Vec3, r1, r2 float64) float64 {
	pa, ba := p.Sub(a), b.Sub(a)
	h := shad.Clamp(pa.Dot(ba)/ba.Dot(ba), 0, 1)
	return pa.Sub(ba.Mul(h)).Len() - shad.Mix(r1, r2, h*h*(3-2*h))
}

func sdBox(p, b shad.Vec3) float64 {
	q := p.Abs().Sub(b)
	return q.Max(0).Len() + math.Min(math.Max(q.X, math.Max(q.Y, q.Z)), 0)
}

func sdTorus(p shad.Vec3, r, t float64) float64 {
	return math.Hypot(math.Hypot(p.X, p.Z)-r, p.Y) - t
}

func sdCappedCylinder(p shad.Vec3, h, r float64) float64 {
	dx, dy := math.Hypot(p.X, p.Z)-h, math.Abs(p.Y)-r
	return math.Min(math.Max(dx, dy), 0) + math.Hypot(math.Max(dx, 0), math.Max(dy, 0))
}

// sdEar is a short torus with a disc in it.
func sdEar(p shad.Vec3) float64 {
	p.X, p.Z = rot(p.X, p.Z, 1.6)
	q := shad.V3(p.Y, p.X, p.Z)
	return math.Min(sdTorus(q, 0.15, 0.05), sdCappedCylinder(q.Sub(shad.V3(0, -0.025, 0)), 0.15, 0.025))
}

func sdFoot(p shad.Vec3) float64 {
	a := shad.V3(0.6, -0.5, -0.1)
	return math.Min(
		sdStick(p, a, a.Sub(shad.V3(0.1, 0.07, 0.4)), 0.15, 0.1),
		sdStick(p, a, a.Sub(shad.V3(0, 0.07, 0.4)), 0.15, 0.1),
	)
}

// distMat is the distance to the nearest gopher and its material there.
func distMat(p shad.Vec3, time float64) (float64, int) {
	p = shad.V3(repeat(p.X, 5), repeat(p.Y, 5), repeat(p.Z, 5))
	alpha := math.Sin(time) / 10
	alphaBody := math.Sin(time-1) / 10

	body := p
	body.Z, body.Y = rot(body.Z, body.Y, alphaBody)
	dBody := sdEllipsoid(body.Sub(shad.V3(0, -1, 0)), shad.V3(0.65, 0.5, 0.5))

	// The head nods about a point between head and body.
	p = p.Sub(shad.V3(0, -0.5, 0))
	p.Z, p.Y = rot(p.Z, p.Y, alpha)
	p = p.Add(shad.V3(0, -0.5, 0))

	dHead := sdEllipsoid(p, shad.V3(0.65, 0.5, 0.5))
	dBodyHead := smin(dHead, dBody, 1.5)

	// The nose is a capsule bent along x.
	np := p.Sub(shad.V3(0, -0.20, -0.56))
	c, s := math.Cos(-2*np.X), math.Sin(-2*np.X)
	np.X, np.Y = c*np.X+s*np.Y, -s*np.X+c*np.Y
	np.X -= shad.Clamp(np.X, -0.1, 0.1)
	dNose := np.Len() - 0.08

	dTruffle := sdEllipsoid(p.Sub(shad.V3(0, -0.15, -0.61)), shad.V3(0.07, 0.05, 0.05))

	arm := body
	arm.X = math.Abs(arm.X)
	a := shad.V3(0.6, -0.5, -0.1)
	dArm := sdStick(arm, a, a.Sub(shad.V3(-0.3, 0.3, 0)), 0.1, 0.15)

	p.X = math.Abs(p.X)
	dTeeth := sdBox(p.Sub(shad.V3(0.050, -0.30, -0.57)), shad.V3(0.03, 0.06, 0.006)) - 0.02
	dEar := sdEar(p.Sub(shad.V3(0.50, 0.30, 0.15)))
	dEyeW := p.Sub(shad.V3(0.3, 0, -0.4)).Len() - 0.25
	dEyeB := p.Sub(shad.V3(0.3, 0, -0.46)).Len() - 0.20

	foot := body
	foot.X = math.Abs(foot.X)
	dFoot := sdFoot(foot.Sub(shad.V3(-0.25, -0.95, 0.25)))

	d := dBodyHead
	for _, v := range []float64{dEyeW, dEyeB, dEar, dNose, dTruffle, dTeeth, dArm, dFoot} {
		d = math.Min(d, v)
	}
	// Later matches win, as in the original.
	m := 0
	for _, c := range []struct {
		d   float64
		mat int
	}{
		{dEyeW, matWhite}, {dEyeB, matBlack}, {dBodyHead, matFur}, {dEar, matFur},
		{dNose, matTan}, {dTruffle, matBlack}, {dTeeth, matWhite}, {dArm, matFur}, {dFoot, matTan},
	} {
		if c.d == d {
			m = c.mat
		}
	}
	return d, m
}
