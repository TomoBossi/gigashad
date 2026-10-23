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
`

// gpuRenderer draws a GPUShader into an image.
type gpuRenderer struct {
	shader   *ebiten.Shader
	img      *ebiten.Image
	uniforms map[string]any
	extra    GPUUniforms
}

func newGPURenderer(s GPUShader) (*gpuRenderer, error) {
	src := s.Kage()
	if src == nil {
		return nil, fmt.Errorf("shader has no GPU version")
	}
	sh, err := ebiten.NewShader(append([]byte(KagePrelude), src...))
	if err != nil {
		return nil, fmt.Errorf("compiling GPU shader: %w", err)
	}
	g := &gpuRenderer{shader: sh, uniforms: map[string]any{}}
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
	u["CamRight"] = Vec3f(c.Right)
	u["CamUp"] = Vec3f(c.Up)
	u["CamFwd"] = Vec3f(c.Fwd)
	u["CamFovY"] = float32(c.FovY)
	u["CamScale"] = float32(c.Scale)
	if g.extra != nil {
		g.extra.Uniforms(f, u)
	}
	g.img.DrawRectShader(w, h, g.shader, &ebiten.DrawRectShaderOptions{Uniforms: u})
}

// Vec3f converts a Vec3 to the form Kage vec3 uniforms take.
func Vec3f(v Vec3) []float32 { return []float32{float32(v.X), float32(v.Y), float32(v.Z)} }
