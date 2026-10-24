// Package plasma is a minimal Shadertoy-style 2D shader: no camera, no world,
// just a function of the pixel coordinate and time.
package plasma

import (
	"math"

	"gigashad/shad"
)

type plasma struct{}

// Register: plasma has no flags of its own.
func Register(shad.Flags) func() shad.Def { return New }

// New: CPU and GPU versions; the GPU runs unless -backend cpu is given.
func New() shad.Def {
	return shad.Def{
		CPU:      plasma{},
		GPU:      &shad.GPU{Variants: []shad.Kage{{Source: kage}}},
		Backend:  func(*shad.Frame) shad.Backend { return shad.OnGPU },
		Bindings: shad.StandardBindings(),
	}
}

func (plasma) Fragment(frag shad.Vec2, f *shad.Frame, out *shad.Sample) {
	uv := frag.Sub(f.Res.Mul(0.5)).Div(f.Res.Y)
	t := f.Time
	v := math.Sin(uv.X*10+t) +
		math.Sin(10*(uv.X*math.Sin(t/2)+uv.Y*math.Cos(t/3))+t) +
		math.Sin(math.Sqrt(100*uv.Dot(uv)+1)+t)
	out.Color = shad.V3(
		0.5+0.5*math.Sin(math.Pi*v),
		0.5+0.5*math.Sin(math.Pi*v+2*math.Pi/3),
		0.5+0.5*math.Sin(math.Pi*v+4*math.Pi/3),
	)
}

// kage is the same shader for the GPU.
var kage = []byte(`
func Fragment(dstPos vec4, srcPos vec2, color vec4) vec4 {
	uv := (fragCoord(dstPos) - Resolution*0.5) / Resolution.y
	t := Time
	v := sin(uv.x*10+t) +
		sin(10*(uv.x*sin(t/2)+uv.y*cos(t/3))+t) +
		sin(sqrt(100*dot(uv, uv)+1)+t)
	const pi = 3.14159265
	return vec4(0.5+0.5*sin(pi*v), 0.5+0.5*sin(pi*v+2*pi/3), 0.5+0.5*sin(pi*v+4*pi/3), 1)
}
`)
