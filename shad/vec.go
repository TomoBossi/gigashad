package shad

import "math"

// Vec2 is a 2D vector, the CPU stand-in for GLSL's vec2.
type Vec2 struct{ X, Y float64 }

// Vec3 is a 3D vector, the CPU stand-in for GLSL's vec3. Also used for RGB colors.
type Vec3 struct{ X, Y, Z float64 }

// Vec4 is a 4D vector, the CPU stand-in for GLSL's vec4.
type Vec4 struct{ X, Y, Z, W float64 }

func V2(x, y float64) Vec2       { return Vec2{x, y} }
func V3(x, y, z float64) Vec3    { return Vec3{x, y, z} }
func V4(x, y, z, w float64) Vec4 { return Vec4{x, y, z, w} }
func S3(s float64) Vec3          { return Vec3{s, s, s} }

func (a Vec2) Add(b Vec2) Vec2    { return Vec2{a.X + b.X, a.Y + b.Y} }
func (a Vec2) Sub(b Vec2) Vec2    { return Vec2{a.X - b.X, a.Y - b.Y} }
func (a Vec2) Mul(s float64) Vec2 { return Vec2{a.X * s, a.Y * s} }
func (a Vec2) MulV(b Vec2) Vec2   { return Vec2{a.X * b.X, a.Y * b.Y} }
func (a Vec2) Div(s float64) Vec2 { return Vec2{a.X / s, a.Y / s} }
func (a Vec2) DivV(b Vec2) Vec2   { return Vec2{a.X / b.X, a.Y / b.Y} }
func (a Vec2) Dot(b Vec2) float64 { return a.X*b.X + a.Y*b.Y }
func (a Vec2) Len() float64       { return math.Sqrt(a.Dot(a)) }
func (a Vec2) Norm() Vec2         { return a.Div(a.Len()) }
func (a Vec2) Abs() Vec2          { return Vec2{math.Abs(a.X), math.Abs(a.Y)} }
func (a Vec2) Fract() Vec2        { return Vec2{Fract(a.X), Fract(a.Y)} }
func (a Vec2) Rot(angle float64) Vec2 {
	s, c := math.Sincos(angle)
	return Vec2{c*a.X - s*a.Y, s*a.X + c*a.Y}
}

func (a Vec3) Add(b Vec3) Vec3            { return Vec3{a.X + b.X, a.Y + b.Y, a.Z + b.Z} }
func (a Vec3) Sub(b Vec3) Vec3            { return Vec3{a.X - b.X, a.Y - b.Y, a.Z - b.Z} }
func (a Vec3) Mul(s float64) Vec3         { return Vec3{a.X * s, a.Y * s, a.Z * s} }
func (a Vec3) MulV(b Vec3) Vec3           { return Vec3{a.X * b.X, a.Y * b.Y, a.Z * b.Z} }
func (a Vec3) Div(s float64) Vec3         { return a.Mul(1 / s) }
func (a Vec3) Dot(b Vec3) float64         { return a.X*b.X + a.Y*b.Y + a.Z*b.Z }
func (a Vec3) Len2() float64              { return a.Dot(a) }
func (a Vec3) Len() float64               { return math.Sqrt(a.Dot(a)) }
func (a Vec3) Neg() Vec3                  { return Vec3{-a.X, -a.Y, -a.Z} }
func (a Vec3) Abs() Vec3                  { return Vec3{math.Abs(a.X), math.Abs(a.Y), math.Abs(a.Z)} }
func (a Vec3) MaxC() float64              { return math.Max(a.X, math.Max(a.Y, a.Z)) }
func (a Vec3) Max(s float64) Vec3         { return Vec3{math.Max(a.X, s), math.Max(a.Y, s), math.Max(a.Z, s)} }
func (a Vec3) Min(s float64) Vec3         { return Vec3{math.Min(a.X, s), math.Min(a.Y, s), math.Min(a.Z, s)} }
func (a Vec3) Mix(b Vec3, t float64) Vec3 { return a.Add(b.Sub(a).Mul(t)) }
func (a Vec3) Clamp01() Vec3              { return Vec3{Clamp(a.X, 0, 1), Clamp(a.Y, 0, 1), Clamp(a.Z, 0, 1)} }
func (a Vec3) Pow(e float64) Vec3 {
	return Vec3{math.Pow(a.X, e), math.Pow(a.Y, e), math.Pow(a.Z, e)}
}
func (a Vec3) Norm() Vec3 {
	l := a.Len()
	if l == 0 {
		return a
	}
	return a.Mul(1 / l)
}
func (a Vec3) Cross(b Vec3) Vec3 {
	return Vec3{a.Y*b.Z - a.Z*b.Y, a.Z*b.X - a.X*b.Z, a.X*b.Y - a.Y*b.X}
}

// RotAxis rotates a around the unit vector axis by angle radians (Rodrigues).
func (a Vec3) RotAxis(axis Vec3, angle float64) Vec3 {
	s, c := math.Sincos(angle)
	return a.Mul(c).Add(axis.Cross(a).Mul(s)).Add(axis.Mul(axis.Dot(a) * (1 - c)))
}

func (a Vec4) Add(b Vec4) Vec4    { return Vec4{a.X + b.X, a.Y + b.Y, a.Z + b.Z, a.W + b.W} }
func (a Vec4) Mul(s float64) Vec4 { return Vec4{a.X * s, a.Y * s, a.Z * s, a.W * s} }
func (a Vec4) XYZ() Vec3          { return Vec3{a.X, a.Y, a.Z} }

// Scalar helpers with GLSL names and semantics.

func Clamp(x, lo, hi float64) float64 { return math.Max(lo, math.Min(hi, x)) }
func Mix(a, b, t float64) float64     { return a + (b-a)*t }
func Fract(x float64) float64         { return x - math.Floor(x) }
func Step(edge, x float64) float64 {
	if x < edge {
		return 0
	}
	return 1
}
func Smoothstep(e0, e1, x float64) float64 {
	t := Clamp((x-e0)/(e1-e0), 0, 1)
	return t * t * (3 - 2*t)
}
