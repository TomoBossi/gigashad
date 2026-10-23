package shad

import (
	"fmt"
	"image"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
)

// GPUShader is implemented by shaders that can also, or only, run on the GPU
// as a Kage program (ebiten's Go-like shading language, compiled to Metal on
// macOS). A shader may implement Shader, GPUShader, or both.
//
// On the GPU everything is 32-bit float and nothing comes back: there is no
// G-buffer (Frame.Prev stays nil). A Field, if the shader has one, is still
// queried on the CPU, so a shader that renders on the GPU and collides on the
// CPU must keep both versions in agreement.
type GPUShader interface {
	// Kage returns the program body, without the package clause, defining
	//
	//	func Fragment(dstPos vec4, srcPos vec2, color vec4) vec4
	//
	// on top of KagePrelude. nil means the shader has no GPU version.
	Kage() []byte
}

// GPUUniforms is implemented by GPU shaders with uniforms of their own, on top
// of the ones KagePrelude declares. Values must be float32, int, or []float32.
type GPUUniforms interface {
	Uniforms(f *Frame, u map[string]any)
}

// GPUPreferrer is implemented by shaders with both versions that want the GPU
// for some frames. Without it, a shader with both runs on the CPU. It is asked
// every frame, so a shader can switch back to the CPU when it needs the
// precision (see fractal.Explorer).
type GPUPreferrer interface {
	PreferGPU(f *Frame) bool
}

// GPUVariants is implemented by GPU shaders that compile to more than one
// program and pick one per frame, e.g. a fast one and a precise one (see the
// double-float helpers in KagePrelude). Variants are compiled once, at start.
type GPUVariants interface {
	// KageVariants returns the program bodies, as Kage would.
	KageVariants() [][]byte
	// Variant returns the index of the program to render f with.
	Variant(f *Frame) int
}

// GPUHandoff is implemented by shaders that will switch from the GPU to the
// CPU (see GPUPreferrer) and want the jump in resolution eased: Handoff
// returns 1 while far from switching, falling to 0 at the switch, and the
// GPU's resolution is brought down toward the CPU's as it falls.
type GPUHandoff interface {
	Handoff(f *Frame) float64
}

// KagePrelude starts every GPU program: pixel units, the uniforms every frame
// fills in, and helpers matching their CPU counterparts.
const KagePrelude = `//kage:unit pixels

package main

var Time float
var Resolution vec2
var CamPos vec3
var CamPosLo vec3 // CamPos + CamPosLo is the camera position to ~48 bits
var CamRight vec3
var CamUp vec3
var CamFwd vec3
var CamFovY float
var CamScale float

// fragCoord converts dstPos to the CPU's convention: pixel centers, origin at
// the bottom-left.
func fragCoord(dstPos vec4) vec2 {
	return vec2(dstPos.x, Resolution.y-dstPos.y)
}

// pixelAngle is Camera.PixelAngle.
func pixelAngle() float {
	return tan(CamFovY/2) * 2 / Resolution.y
}

// camRay is Camera.Ray.
func camRay(frag vec2) vec3 {
	uv := (frag - Resolution/2) * pixelAngle()
	return normalize(CamFwd + CamRight*uv.x + CamUp*uv.y)
}

// Double-float arithmetic: a value is a vec2 (hi, lo) of two floats with
// |lo| <= half an ulp of hi, about 48 bits of precision. It only works in
// shaders that declare
//
//	var PreciseMath float
//
// without which Metal's fast math optimises the error terms away.

func dfQuickTwoSum(a, b float) vec2 {
	s := a + b
	return vec2(s, b-(s-a))
}

func dfTwoSum(a, b float) vec2 {
	s := a + b
	bb := s - a
	return vec2(s, (a-(s-bb))+(b-bb))
}

func dfAdd(a, b vec2) vec2 {
	s := dfTwoSum(a.x, b.x)
	return dfQuickTwoSum(s.x, s.y+a.y+b.y)
}

func dfSub(a, b vec2) vec2 {
	return dfAdd(a, -b)
}

func dfSplit(a float) vec2 {
	c := 4097 * a
	hi := c - (c - a)
	return vec2(hi, a-hi)
}

func dfMul(a, b vec2) vec2 {
	p := a.x * b.x
	as := dfSplit(a.x)
	bs := dfSplit(b.x)
	e := ((as.x*bs.x - p) + as.x*bs.y + as.y*bs.x) + as.y*bs.y
	return dfQuickTwoSum(p, e+a.x*b.y+a.y*b.x)
}

// dfNeg reports a < 0.
func dfNeg(a vec2) bool {
	return a.x < 0 || (a.x == 0 && a.y < 0)
}

func dfAbs(a vec2) vec2 {
	if dfNeg(a) {
		return -a
	}
	return a
}

func dfFloor(a vec2) vec2 {
	hi := floor(a.x)
	if hi != a.x {
		return vec2(hi, 0)
	}
	return dfQuickTwoSum(hi, floor(a.y))
}

// dfAdd3 is dfAdd on each component of (ah + al) and (bh + bl).
func dfAdd3(ah, al, bh, bl vec3) (vec3, vec3) {
	s := ah + bh
	bb := s - ah
	e := (ah - (s - bb)) + (bh - bb) + al + bl
	h := s + e
	return h, e - (h - s)
}
`

// gpuRenderer draws a GPUShader into an image.
type gpuRenderer struct {
	shaders  []*ebiten.Shader
	variants GPUVariants
	img      *ebiten.Image
	uniforms map[string]any
	extra    GPUUniforms
}

func newGPURenderer(s GPUShader) (*gpuRenderer, error) {
	srcs := [][]byte{s.Kage()}
	g := &gpuRenderer{uniforms: map[string]any{}}
	if v, ok := s.(GPUVariants); ok {
		srcs = v.KageVariants()
		g.variants = v
	}
	for i, src := range srcs {
		if src == nil {
			return nil, fmt.Errorf("shader has no GPU version")
		}
		sh, err := ebiten.NewShader(append([]byte(KagePrelude), src...))
		if err != nil {
			return nil, fmt.Errorf("compiling GPU shader (variant %d): %w", i, err)
		}
		g.shaders = append(g.shaders, sh)
	}
	// Metal compiles a program on first use; do it now rather than as a
	// hitch when a variant first comes into play.
	warm := ebiten.NewImage(1, 1)
	defer warm.Deallocate()
	for _, sh := range g.shaders {
		warm.DrawRectShader(1, 1, sh, nil)
	}
	g.extra, _ = s.(GPUUniforms)
	return g, nil
}

// resize makes the target image w×h, reporting whether it changed.
func (g *gpuRenderer) resize(w, h int) bool {
	if g.img != nil && g.img.Bounds().Dx() == w && g.img.Bounds().Dy() == h {
		return false
	}
	if g.img != nil {
		g.img.Deallocate()
	}
	g.img = ebiten.NewImage(w, h)
	return true
}

// renderTimed renders and waits for the GPU to finish, so the time it returns
// is what the frame actually cost.
func (g *gpuRenderer) renderTimed(f *Frame) time.Duration {
	t0 := time.Now()
	g.render(f)
	var px [4]byte
	g.img.SubImage(image.Rect(0, 0, 1, 1)).(*ebiten.Image).ReadPixels(px[:])
	return time.Since(t0)
}

func (g *gpuRenderer) render(f *Frame) {
	w, h := g.img.Bounds().Dx(), g.img.Bounds().Dy()
	f.Res = Vec2{float64(w), float64(h)}
	f.Prev = nil
	u := g.uniforms
	c := &f.Cam
	u["Time"] = float32(f.Time)
	u["Resolution"] = []float32{float32(w), float32(h)}
	u["CamPos"] = Vec3f(c.Pos)
	u["CamPosLo"] = Vec3fLo(c.Pos)
	u["CamRight"] = Vec3f(c.Right)
	u["CamUp"] = Vec3f(c.Up)
	u["CamFwd"] = Vec3f(c.Fwd)
	u["CamFovY"] = float32(c.FovY)
	u["CamScale"] = float32(c.Scale)
	if g.extra != nil {
		g.extra.Uniforms(f, u)
	}
	sh := g.shaders[0]
	if g.variants != nil {
		sh = g.shaders[max(0, min(len(g.shaders)-1, g.variants.Variant(f)))]
	}
	g.img.DrawRectShader(w, h, sh, &ebiten.DrawRectShaderOptions{Uniforms: u})
}

// Vec3f converts a Vec3 to the form Kage vec3 uniforms take.
func Vec3f(v Vec3) []float32 { return []float32{float32(v.X), float32(v.Y), float32(v.Z)} }

// Vec3fLo is what Vec3f rounds away: Vec3f(v) + Vec3fLo(v) is v to ~48 bits,
// for the double-float helpers in KagePrelude.
func Vec3fLo(v Vec3) []float32 {
	lo := func(x float64) float32 { return float32(x - float64(float32(x))) }
	return []float32{lo(v.X), lo(v.Y), lo(v.Z)}
}
