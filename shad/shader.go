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
	Time    float64 // seconds since start (iTime)
	Delta   float64 // seconds since the previous frame (iTimeDelta)
	Index   int     // frame number (iFrame)
	Res     Vec2    // render resolution in pixels (iResolution.xy)
	Cam     Camera  // the view; Def.Update sets it
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
}

// Ray returns the unit direction through pixel frag.
func (c *Camera) Ray(frag, res Vec2) Vec3 {
	k := math.Tan(c.FovY/2) * 2 / res.Y
	u := (frag.X - res.X/2) * k
	v := (frag.Y - res.Y/2) * k
	return c.Fwd.Add(c.Right.Mul(u)).Add(c.Up.Mul(v)).Norm()
}

// PixelAngle is the angular size of one pixel, handy for cone-based level of
// detail (a hit is "close enough" once the distance is below t*PixelAngle).
func (c *Camera) PixelAngle(res Vec2) float64 {
	return math.Tan(c.FovY/2) * 2 / res.Y
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
