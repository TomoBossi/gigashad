package shad

import (
	"fmt"
	"image"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
)

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
// Precise variants (see Kage): elsewhere Metal's fast math folds the error
// terms away, and the helpers cost about as much as plain float.

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

// gpuRenderer draws a GPU program into an image.
type gpuRenderer struct {
	def      *GPU
	shaders  []*ebiten.Shader
	img      *ebiten.Image
	uniforms map[string]any
}

// preciseDecl opts a program out of Metal's fast math: the patched ebiten in
// third_party compiles programs declaring this uniform without it.
const preciseDecl = "\nvar PreciseMath float\n"

func newGPURenderer(def *GPU) (*gpuRenderer, error) {
	if def == nil || len(def.Variants) == 0 {
		return nil, fmt.Errorf("shader has no GPU version")
	}
	g := &gpuRenderer{def: def, uniforms: map[string]any{}}
	for i, v := range def.Variants {
		src := []byte(KagePrelude)
		if v.Precise {
			src = append(src, preciseDecl...)
		}
		sh, err := ebiten.NewShader(append(src, v.Source...))
		if err != nil {
			return nil, fmt.Errorf("compiling GPU variant %d: %w", i, err)
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
	if g.def.Uniforms != nil {
		g.def.Uniforms(f, u)
	}
	sh := g.shaders[0]
	if g.def.Variant != nil {
		sh = g.shaders[max(0, min(len(g.shaders)-1, g.def.Variant(f)))]
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
