package shad

import "math"

// Shader is the CPU equivalent of a Shadertoy mainImage: it is called once per
// pixel per frame, concurrently from several goroutines, so Fragment must not
// mutate shared state. Per-frame state belongs in Def.Update.
type Shader interface {
	// Fragment shades the pixel whose center is frag (in pixels, origin at the
	// bottom-left like gl_FragCoord) and fills out. out arrives zeroed except
	// for Depth, which is +Inf.
	Fragment(frag Vec2, f *Frame, out *Sample)
}

// Sample is what a fragment produces. Color is all the screen needs; the rest
// is a G-buffer that game logic can read back after the frame (Frame.Prev),
// which is impossible with a GPU shader without a readback round-trip.
type Sample struct {
	Color  Vec3    // linear-ish RGB in [0,1], written to screen as-is
	Depth  float64 // distance along the view ray to what this pixel shows; +Inf for nothing
	Pos    Vec3    // world position of the hit
	Normal Vec3    // surface normal at the hit
	ID     int     // shader-defined material/object id
	Steps  int     // shader-defined cost metric (e.g. march steps)

	reused uint8 // frames this sample has been carried over by Renderer.Reuse
}

// Field is implemented by shaders whose world is a signed distance field. It is
// what makes the world queryable anywhere, on or off screen: collisions,
// ground checks, line of sight, AI, etc.
type Field interface {
	// Dist returns the (approximately) signed distance from p to the nearest
	// surface. detail is the smallest feature size the caller cares about;
	// fractal fields use it to pick an iteration count, plain SDFs ignore it.
	Dist(p Vec3, detail float64) float64
}

// Frame holds the uniforms of one frame plus a read-only view of the previous
// frame's G-buffer.
type Frame struct {
	Time  float64 // seconds since start (iTime)
	Delta float64 // seconds since the previous frame (iTimeDelta)
	Index int     // frame number (iFrame)
	Res   Vec2    // render resolution in pixels (iResolution.xy)
	// LOD is the resolution to choose level of detail from (hit thresholds,
	// iteration counts, anything sized by the pixel footprint). The app
	// moves it slowly toward Res, so when dynamic resolution steps, detail
	// drifts gradually instead of jumping. Zero means Res.
	LOD     Vec2
	Cam     Camera // the view; Def.Update sets it
	Sliders [4]float64
	Prev    *GBuffer // previous frame's samples; nil on the first frame

}

// Camera is a pinhole camera. Scale is the observer's size: shaders that
// support it render and move in units of Scale, so shrinking reveals detail
// at the same cost (see package fractal).
type Camera struct {
	Pos            Vec3
	Right, Up, Fwd Vec3
	FovY           float64 // vertical field of view, radians
	Scale          float64
	// Fisheye blends from a rectilinear projection (0: straight lines stay
	// straight) to an equidistant one (1: the angle from the view axis grows
	// in proportion to the distance from the image center, as in
	// Mandelbulb3D).
	Fisheye float64
}

// Ray returns the unit direction through pixel frag.
func (c *Camera) Ray(frag, res Vec2) Vec3 {
	k := math.Tan(c.FovY/2) * 2 / res.Y
	u := (frag.X - res.X/2) * k
	v := (frag.Y - res.Y/2) * k
	if c.Fisheye != 0 {
		if r := math.Hypot(u, v); r > 0 {
			theta := Mix(math.Atan(r), r/k*c.FovY/res.Y, c.Fisheye)
			s := math.Tan(math.Min(theta, 1.55)) / r
			u, v = u*s, v*s
		}
	}
	return c.Fwd.Add(c.Right.Mul(u)).Add(c.Up.Mul(v)).Norm()
}

// Project is the inverse of Ray: the pixel position (CPU convention) that
// direction v, relative to the camera, shows up at, or ok=false if it is
// behind the camera.
func (c *Camera) Project(v Vec3, res Vec2) (frag Vec2, ok bool) {
	z := v.Dot(c.Fwd)
	if z <= 0 {
		return Vec2{}, false
	}
	k := math.Tan(c.FovY/2) * 2 / res.Y
	u, w := v.Dot(c.Right)/z, v.Dot(c.Up)/z // rectilinear image-plane coordinates
	if c.Fisheye != 0 {
		// Ray maps image-plane radius r to the angle
		// theta(r) = mix(atan(r), r/k·FovY/H, Fisheye); find the r giving
		// this direction's angle (theta is increasing, Newton converges fast).
		if t := math.Hypot(u, w); t > 0 {
			theta := math.Atan(t)
			a := c.FovY / (res.Y * k)
			r := theta / Mix(1, a, c.Fisheye)
			for range 6 {
				f := Mix(math.Atan(r), a*r, c.Fisheye) - theta
				df := Mix(1/(1+r*r), a, c.Fisheye)
				r -= f / df
			}
			u, w = u*r/t, w*r/t
		}
	}
	return Vec2{u/k + res.X/2, w/k + res.Y/2}, true
}

// PixelAngle is the angular size of one pixel, handy for cone-based level of
// detail (a hit is "close enough" once the distance is below t*PixelAngle).
func (c *Camera) PixelAngle(res Vec2) float64 {
	return math.Tan(c.FovY/2) * 2 / res.Y
}

// LODRes returns f.LOD, or f.Res if it isn't set.
func (f *Frame) LODRes() Vec2 {
	if f.LOD.Y > 0 {
		return f.LOD
	}
	return f.Res
}

// GBuffer is a frame worth of samples, row 0 at the bottom.
type GBuffer struct {
	W, H    int
	Samples []Sample
}

// At returns the sample at pixel (x, y), y up. Out of range coordinates clamp.
func (g *GBuffer) At(x, y int) *Sample {
	x = max(0, min(g.W-1, x))
	y = max(0, min(g.H-1, y))
	return &g.Samples[y*g.W+x]
}

// Center returns the sample under the crosshair.
func (g *GBuffer) Center() *Sample { return g.At(g.W/2, g.H/2) }
