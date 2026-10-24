package shad

import (
	"errors"
	"fmt"
	"image"
	"image/png"
	"math"
	"os"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
)

// Default render widths, used to start the resolution controller and for
// -shot and -bench when no width is given.
const (
	DefaultCPUWidth = 240
	DefaultGPUWidth = 960
)

// Options configure an App.
type Options struct {
	// Width fixes the render width on both backends; 0 picks it every frame
	// to hold FPS.
	Width int
	// Aspect fixes the render aspect ratio (width/height), letterboxing the
	// window; 0 follows the window, so fullscreen uses the monitor's.
	Aspect  float64
	Backend Backend // overrides Def.Backend unless Auto
	FPS     int
	Threads int // render goroutines; 0 means one per CPU core
	Reuse   int // on static shaders, shade 1 pixel in Reuse per frame (see Renderer.Reuse)

	Fullscreen bool
	Title      string
	ShotDir    string  // where screenshots and scene data go
	ShotPath   string  // if set, save the first frame here and quit
	ShotTime   float64 // Frame.Time of that frame
	Bench      int     // if set, time this many frames, print, and quit
	ShowFPS    bool    // draw a frame rate counter in the bottom-right corner
}

// App owns the window and the per-frame loop: input, bindings, the shader's
// Update, then rendering on whichever backend the frame calls for.
type App struct {
	opt Options
	def *Def
	gpu *gpuRenderer

	renderer *Renderer     // CPU renderer, recreated when the size changes
	img      *ebiten.Image // CPU frame, uploaded from renderer
	cpuRes   *resController
	gpuRes   *resController
	onGPU    bool // the current frame came from the GPU
	post     *postFX
	final    *ebiten.Image // the frame on screen: the render, after Post

	winW, winH int // window size in device-independent pixels, from Layout

	frame   Frame
	input   Input
	start   time.Time
	last    time.Time
	mouseX  int
	mouseY  int
	mouseOK bool
	quit    bool
	lastCam Camera
	lastW   int
	lodW    float64 // Frame.LOD width, trailing the render width
	lastH   int
	took    time.Duration // render time of the last frame

	// The frame rate counter counts frames that reach the screen, not
	// Update or Draw calls: Draw runs at the display's rate whatever -fps is.
	frames, shown int // frames made by Update, and the last one drawn
	fpsN          int // frames drawn since fpsSince
	fpsSince      time.Time
	fps           float64
}

var errQuit = errors.New("quit")

// lodSmoothing is the time constant, in seconds, with which the level of
// detail follows the render resolution.
const lodSmoothing = 2.0

// NewApp prepares an App for def; Run opens the window.
func NewApp(def *Def, opt Options) (*App, error) {
	a := &App{opt: opt, def: def}
	switch {
	case opt.Backend == OnCPU && def.CPU == nil:
		return nil, fmt.Errorf("this shader has no CPU version")
	case opt.Backend == OnGPU && def.GPU == nil:
		return nil, fmt.Errorf("this shader has no GPU version")
	case def.CPU == nil && def.GPU == nil:
		return nil, fmt.Errorf("the shader defines neither a CPU nor a GPU version")
	}
	if def.GPU != nil && opt.Backend != OnCPU {
		g, err := newGPURenderer(def.GPU)
		if err != nil {
			return nil, err
		}
		a.gpu = g
	}
	if def.Post != nil {
		fx, err := newPostFX(*def.Post)
		if err != nil {
			return nil, err
		}
		a.post = fx
	}
	// Leave a quarter of each frame for everything else (input, Update,
	// collisions, presenting), and more at high frame rates, where frames
	// are short and that work is a larger share of them.
	share := 0.75
	if opt.FPS >= 60 {
		share = 0.65
	}
	budget := share / float64(opt.FPS)
	a.cpuRes = newResController(DefaultCPUWidth, 96, 640, budget)
	a.gpuRes = newResController(DefaultGPUWidth, 160, 3840, budget)
	if opt.Width > 0 {
		a.cpuRes = newResController(opt.Width, opt.Width, opt.Width, budget)
		a.gpuRes = a.cpuRes
	}
	return a, nil
}

func (a *App) Run() error {
	a.start = time.Now()
	a.last = a.start
	mw, mh := ebiten.Monitor().Size()
	aspect := a.opt.Aspect
	if aspect == 0 {
		aspect = float64(mw) / float64(mh)
	}
	ww := min(1280, mw*3/4)
	ebiten.SetWindowTitle(a.opt.Title)
	ebiten.SetWindowSize(ww, int(float64(ww)/aspect))
	ebiten.SetWindowResizingMode(ebiten.WindowResizingModeEnabled)
	ebiten.SetFullscreen(a.opt.Fullscreen)
	if a.def.CaptureMouse {
		ebiten.SetCursorMode(ebiten.CursorModeCaptured)
	}
	ebiten.SetTPS(a.opt.FPS)
	ebiten.SetVsyncEnabled(true)
	err := ebiten.RunGame(a)
	if errors.Is(err, errQuit) {
		return nil
	}
	return err
}

// aspect is the render aspect ratio for this frame.
func (a *App) aspect() float64 {
	if a.opt.Aspect > 0 {
		return a.opt.Aspect
	}
	if a.winW > 0 && a.winH > 0 {
		return float64(a.winW) / float64(a.winH)
	}
	mw, mh := ebiten.Monitor().Size()
	return float64(mw) / float64(mh)
}

func (a *App) Update() error {
	now := time.Now()
	dt := min(now.Sub(a.last).Seconds(), 0.1)

	// macOS releases the captured pointer when the window loses focus or
	// changes to or from fullscreen; take it back whenever we have focus.
	if a.def.CaptureMouse && ebiten.IsFocused() && ebiten.CursorMode() != ebiten.CursorModeCaptured {
		ebiten.SetCursorMode(ebiten.CursorModeCaptured)
		a.mouseOK = false // don't turn the camera by the jump back
	}
	a.last = now

	f := &a.frame
	f.Time = now.Sub(a.start).Seconds()
	if a.opt.ShotPath != "" {
		f.Time = a.opt.ShotTime
	}
	f.Delta = dt
	f.Prev = nil
	if f.Index > 0 && !a.onGPU && a.renderer != nil {
		f.Prev = a.renderer.Last()
	}

	a.readInput(dt)
	c := &Context{Frame: f, Input: &a.input, def: a.def, app: a}
	if a.opt.ShotPath != "" || a.opt.Bench > 0 {
		c.Input = &Input{} // shots and benchmarks see no input
	}
	for i := range a.def.Bindings {
		if b := &a.def.Bindings[i]; b.fired(c.Input) {
			b.Action(c)
		}
	}
	if a.quit {
		return errQuit
	}
	if a.def.Update != nil {
		a.def.Update(c)
	}

	gpu := a.useGPU(f)
	res := a.cpuRes
	if gpu {
		res = a.gpuRes
		if a.opt.Width == 0 && a.winW > 0 {
			// No point rendering finer than the window's physical pixels.
			hi := float64(a.winW) * ebiten.Monitor().DeviceScaleFactor()
			if a.def.Handoff != nil && a.def.CPU != nil {
				// Approaching a switch to the CPU: ease down to its width
				// (geometrically, so each step looks alike).
				k := Clamp(a.def.Handoff(f), 0, 1)
				lo := float64(a.cpuRes.cur)
				hi = lo * math.Pow(hi/lo, k)
			}
			res.setMax(int(hi))
		}
	}
	// Without Update there's no camera to move, but an animated image
	// changes everywhere every frame, which hides resizes just as well.
	moving := f.Index == 0 || f.Cam != a.lastCam || (a.def.Update == nil && !a.static())
	w := res.width(moving)
	h := max(1, int(math.Round(float64(w)/a.aspect())))
	// Level of detail follows the render width, but slowly (see Frame.LOD):
	// a smaller render still gets cheaper pixels, while a resize turns into a
	// gradual drift of detail instead of a jump in lighting and structure.
	if a.lodW == 0 || a.opt.ShotPath != "" || a.opt.Bench > 0 {
		a.lodW = float64(w)
	} else {
		a.lodW += (float64(w) - a.lodW) * (1 - math.Exp(-dt/lodSmoothing))
	}
	f.LOD = Vec2{a.lodW, a.lodW / a.aspect()}
	if a.opt.Bench > 0 {
		return a.bench(f, gpu, w, h)
	}

	if a.static() && f.Index > 0 && f.Cam == a.lastCam && gpu == a.onGPU && w == a.lastW && h == a.lastH {
		a.frames++ // nothing changed; the last frame is still current
		return nil
	}
	a.lastCam, a.lastW, a.lastH = f.Cam, w, h
	a.onGPU = gpu

	// Time the whole frame, render and post, waiting for the GPU to finish.
	t0 := time.Now()
	var resized bool
	if gpu {
		resized = a.gpu.resize(w, h)
		a.gpu.render(f)
	} else {
		resized = a.resizeCPU(w, h)
		a.renderer.Reuse = 1
		if a.static() {
			a.renderer.Reuse = a.opt.Reuse
		}
		a.renderer.Render(a.def.CPU, f)
		a.img.WritePixels(a.renderer.RGBA)
	}
	a.final = a.current()
	if a.post != nil {
		a.final = a.post.apply(a.final)
	}
	if gpu || a.post != nil {
		syncGPU(a.final)
	}
	took := time.Since(t0)
	// The first frame at a new size has nothing to reuse and isn't typical.
	if !resized {
		res.observe(took.Seconds())
	}
	a.took = took
	a.frames++
	f.Index++

	if a.opt.ShotPath != "" {
		if err := a.save(a.opt.ShotPath); err != nil {
			return err
		}
		return errQuit
	}
	return nil
}

func (a *App) static() bool { return a.def.Static != nil && a.def.Static() }

func (a *App) readInput(dt float64) {
	mx, my := ebiten.CursorPosition()
	_, wheel := ebiten.Wheel()
	a.input = Input{DT: dt, Wheel: wheel, live: true}
	a.input.readPads()
	if a.mouseOK {
		a.input.MouseX = float64(mx - a.mouseX)
		a.input.MouseY = float64(my - a.mouseY)
	}
	a.mouseX, a.mouseY, a.mouseOK = mx, my, true
}

// resizeCPU makes the CPU renderer w×h, reporting whether it changed.
func (a *App) resizeCPU(w, h int) bool {
	if a.renderer != nil && a.renderer.W == w && a.renderer.H == h {
		return false
	}
	a.renderer = NewRenderer(w, h)
	if a.opt.Threads > 0 {
		a.renderer.Workers = a.opt.Threads
	}
	if a.img != nil {
		a.img.Deallocate()
	}
	a.img = ebiten.NewImage(w, h)
	return true
}

// bench renders Options.Bench frames back to back, waiting for each to
// finish (the GPU included), and prints the average.
func (a *App) bench(f *Frame, gpu bool, w, h int) error {
	backend := "CPU"
	if gpu {
		backend = "GPU"
		a.gpu.resize(w, h)
	} else {
		a.resizeCPU(w, h)
	}
	var total time.Duration
	for range a.opt.Bench {
		if gpu {
			total += a.gpu.renderTimed(f)
		} else {
			t0 := time.Now()
			a.renderer.Render(a.def.CPU, f)
			total += time.Since(t0)
		}
	}
	per := total / time.Duration(a.opt.Bench)
	fmt.Printf("%s %dx%d: %v per frame (%.0f fps)\n", backend, w, h, per.Round(time.Microsecond), 1/per.Seconds())
	return errQuit
}

// useGPU decides where this frame renders.
func (a *App) useGPU(f *Frame) bool {
	return a.gpu != nil && chooseGPU(a.def, a.opt.Backend, f)
}

func chooseGPU(def *Def, be Backend, f *Frame) bool {
	switch {
	case def.GPU == nil || be == OnCPU:
		return false
	case def.CPU == nil || be == OnGPU:
		return true
	}
	return def.Backend != nil && def.Backend(f) == OnGPU
}

// StartsOnGPU reports whether the first frame would render on the GPU.
func StartsOnGPU(def *Def, be Backend) bool {
	return chooseGPU(def, be, FirstFrame(def, 0))
}

// FirstFrame prepares a frame at time t the way the app's first one is:
// Update with no input. It is meant for a fresh Def (-shot, tests): Update
// may advance the shader's state.
func FirstFrame(def *Def, t float64) *Frame {
	p := DefaultPlayer()
	f := &Frame{Time: t, Cam: p.Camera()}
	if def.Update != nil {
		def.Update(&Context{Frame: f, Input: &Input{}, def: def})
	}
	return f
}

func (a *App) Draw(screen *ebiten.Image) {
	img := a.final
	if img == nil {
		return
	}
	w, h := float64(img.Bounds().Dx()), float64(img.Bounds().Dy())
	sw, sh := float64(screen.Bounds().Dx()), float64(screen.Bounds().Dy())
	scale := min(sw/w, sh/h)
	op := &ebiten.DrawImageOptions{Filter: ebiten.FilterNearest}
	op.GeoM.Scale(scale, scale)
	op.GeoM.Translate((sw-scale*w)/2, (sh-scale*h)/2)
	screen.DrawImage(img, op)
	if a.opt.ShowFPS {
		drawFPS(screen, a.countFPS())
	}
}

// countFPS counts new frames as they're drawn and returns the rate over the
// last second or so.
func (a *App) countFPS() float64 {
	now := time.Now()
	if a.fpsSince.IsZero() {
		a.fpsSince = now
	}
	if a.frames != a.shown {
		a.shown = a.frames
		a.fpsN++
	}
	if d := now.Sub(a.fpsSince).Seconds(); d >= 1 {
		a.fps = float64(a.fpsN) / d
		a.fpsN, a.fpsSince = 0, now
	}
	return a.fps
}

// fpsText holds the counter's text, drawn once and stamped several times.
var fpsText *ebiten.Image

// drawFPS draws the frame rate in the bottom-right corner, white with a black
// outline so it reads over any image. It goes on the screen only, never into
// screenshots.
func drawFPS(screen *ebiten.Image, fps float64) {
	// ebitenutil's debug font: 6 pixels per character, 16 per line.
	const charW, lineH, pad = 6, 16, 3
	s := fmt.Sprintf("%.0f fps", fps)
	w := len(s)*charW + 2
	if fpsText == nil || fpsText.Bounds().Dx() < w {
		fpsText = ebiten.NewImage(w+4*charW, lineH)
	}
	fpsText.Clear()
	ebitenutil.DebugPrintAt(fpsText, s, 0, 0)
	b := screen.Bounds()
	x, y := float64(b.Max.X-w-pad), float64(b.Max.Y-lineH-pad)
	for dy := -1.0; dy <= 1; dy++ {
		for dx := -1.0; dx <= 1; dx++ {
			if dx == 0 && dy == 0 {
				continue
			}
			op := &ebiten.DrawImageOptions{}
			op.GeoM.Translate(x+dx, y+dy)
			op.ColorScale.Scale(0, 0, 0, 1)
			screen.DrawImage(fpsText, op)
		}
	}
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Translate(x, y)
	screen.DrawImage(fpsText, op)
}

func (a *App) current() *ebiten.Image {
	if a.onGPU {
		return a.gpu.img
	}
	return a.img
}

func (a *App) Layout(w, h int) (int, int) {
	a.winW, a.winH = w, h
	return w, h
}

func (a *App) save(path string) error {
	if a.final == nil {
		return fmt.Errorf("nothing rendered yet")
	}
	if !a.onGPU && a.post == nil {
		return SavePNG(path, a.renderer.RGBA, a.renderer.W, a.renderer.H)
	}
	img := a.final
	w, h := img.Bounds().Dx(), img.Bounds().Dy()
	pix := make([]byte, w*h*4)
	img.ReadPixels(pix)
	return SavePNG(path, pix, w, h)
}

// SavePNG writes an RGBA buffer (row 0 at the top) as a PNG.
func SavePNG(path string, rgba []byte, w, h int) error {
	img := &image.RGBA{Pix: rgba, Stride: w * 4, Rect: image.Rect(0, 0, w, h)}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return png.Encode(f, img)
}

// Shot renders a single CPU frame without opening a window and saves it as a
// PNG. GPU shots need a window: use an App with Options.ShotPath.
func Shot(def *Def, w, h int, t float64, path string) (time.Duration, error) {
	if def.CPU == nil {
		return 0, fmt.Errorf("this shader has no CPU version")
	}
	r := NewRenderer(w, h)
	f := FirstFrame(def, t)
	t0 := time.Now()
	r.Render(def.CPU, f)
	return time.Since(t0), SavePNG(path, r.RGBA, w, h)
}
