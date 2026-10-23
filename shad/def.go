package shad

import "github.com/hajimehoshi/ebiten/v2"

// Def defines a shader: what renders it, how it reacts to input, and what the
// engine may assume to save work. Only one of CPU and GPU is required.
type Def struct {
	// CPU renders on the CPU, in 64-bit floats, with a readable G-buffer.
	CPU Shader
	// GPU renders on the GPU (see GPU).
	GPU *GPU
	// Backend picks where a frame renders when both CPU and GPU are set; nil
	// means the CPU. The -backend flag overrides it.
	Backend func(f *Frame) Backend
	// Handoff, when the shader switches from the GPU to the CPU at some point
	// (as Backend decides), returns 1 while far from the switch, falling to 0
	// at it; the GPU's resolution is eased toward the CPU's as it falls.
	Handoff func(f *Frame) float64

	// Static reports whether the image depends only on Frame.Cam, not on time
	// or anything else. While it does, frames with an unchanged camera are
	// skipped and the CPU reuses pixels between frames (see Renderer.Reuse).
	// nil means never.
	Static func() bool

	// Field is the world as a distance field, if there is one: what collision
	// and visibility queries use, on or off screen.
	Field Field

	// Update runs once per frame before rendering, on one goroutine: read
	// input, move the camera (set c.Frame.Cam), advance any state. It also
	// runs, with no input, before -shot and -bench frames.
	Update func(c *Context)
	// Bindings map inputs to actions, checked every frame before Update.
	Bindings []Binding
	// Data adds shader-specific lines to SaveData's report.
	Data func(c *Context) string
}

// GPU is the GPU version of a shader, as one or more Kage programs
// (ebiten's Go-like shading language, compiled to Metal on macOS).
//
// On the GPU everything is 32-bit float and nothing comes back: there is no
// G-buffer (Frame.Prev stays nil). A Field is still queried on the CPU, so a
// shader that renders on the GPU and collides on the CPU must keep both
// versions in agreement.
type GPU struct {
	// Variants are compiled at start. Each is a program body, without the
	// package clause, defining
	//
	//	func Fragment(dstPos vec4, srcPos vec2, color vec4) vec4
	//
	// on top of KagePrelude.
	Variants []Kage
	// Variant picks the variant to render a frame with; nil means 0.
	Variant func(f *Frame) int
	// Uniforms fills in the program's own uniforms, on top of KagePrelude's.
	// Values must be float32, int, or []float32.
	Uniforms func(f *Frame, u map[string]any)
}

// Kage is one GPU program.
type Kage struct {
	Source []byte
	// Precise compiles without Metal's fast math, which otherwise lets the
	// compiler reassociate float arithmetic. The double-float helpers in
	// KagePrelude need it; it costs about a third of the speed, so a shader
	// that only sometimes needs the precision should offer a fast variant
	// too.
	Precise bool
}

// Backend selects where a frame renders.
type Backend int

const (
	Auto Backend = iota // the shader's choice (Def.Backend)
	OnCPU
	OnGPU
)

// Key is a keyboard key: use ebiten's Key constants (ebiten.KeyW, ...).
type Key = ebiten.Key

// Binding maps a key, or a combination of keys, to an action.
type Binding struct {
	// Keys must all be held, and the last one triggers the action.
	Keys []Key
	// Hold fires the action every frame the keys are held, instead of once
	// per press.
	Hold bool
	// Help describes the action, for listings.
	Help   string
	Action func(c *Context)
}

// Context is what Update and actions get: the frame being prepared, this
// frame's input, and the app's services.
type Context struct {
	Frame *Frame
	Input *Input
	app   *App // nil for -shot frames, where app services do nothing
}

// RunFrame runs one frame of def's logic outside the app, as the app would:
// bindings, then Update. For tests and scripted runs; app services (Field,
// Screenshot, ...) do nothing.
func RunFrame(def *Def, f *Frame, in *Input) {
	c := &Context{Frame: f, Input: in}
	for i := range def.Bindings {
		if b := &def.Bindings[i]; b.fired(in) {
			b.Action(c)
		}
	}
	if def.Update != nil {
		def.Update(c)
	}
}
