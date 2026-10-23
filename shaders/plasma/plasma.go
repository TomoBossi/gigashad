// Package plasma is a minimal Shadertoy-style 2D shader: no camera, no world,
// just a function of the pixel coordinate and time.
package plasma

import (
	"math"

	"gigashad/shad"
)

type plasma struct{}

func New() shad.Shader { return plasma{} }

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
