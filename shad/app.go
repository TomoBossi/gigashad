package shad

import (
	"errors"
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
)

// Backend selects where shaders run.
type Backend int

const (
	Auto Backend = iota // the shader's choice: CPU, unless it prefers the GPU (GPUPreferrer)
	CPU
	GPU
)

// Options configure an App.
type Options struct {
	Width, Height       int // CPU render resolution
	GPUWidth, GPUHeight int // GPU render resolution
	Backend             Backend
	FPS                 int
	Threads             int // render goroutines; 0 means one per CPU core
	Reuse               int // on static shaders, shade 1 pixel in Reuse per frame (see Renderer.Reuse)
	Fullscreen          bool
	Title               string
	ShotDir             string  // where P saves screenshots
	ShotPath            string  // if set, save the first frame here and quit
	ShotTime            float64 // Frame.Time of that frame
	Bench               int     // if set, time this many frames from the spawn point, print, and quit
}

// App owns the window, the player and the per-frame loop.
type App struct {
	opt      Options
	program  any
	shader   Shader // CPU version, or nil
	gpu      *gpuRenderer
	field    Field
	renderer *Renderer
	img      *ebiten.Image // CPU frame
	onGPU    bool          // the current frame came from the GPU
	player   Player
	spawn    Player
	frame    Frame
	start    time.Time
	last     time.Time
	mouseX   int
	mouseY   int
	mouseOK  bool
	lastCam  Camera
}

var errQuit = errors.New("quit")

// NewApp prepares an App for a shader (a Shader, a GPUShader, or both); Run
// opens the window.
func NewApp(program any, opt Options) (*App, error) {
	a := &App{opt: opt, program: program, renderer: NewRenderer(opt.Width, opt.Height)}
	a.shader, _ = program.(Shader)
	gs, hasGPU := program.(GPUShader)
	hasGPU = hasGPU && gs.Kage() != nil
	switch {
	case opt.Backend == CPU && a.shader == nil:
		return nil, fmt.Errorf("this shader has no CPU version")
	case opt.Backend == GPU && !hasGPU:
		return nil, fmt.Errorf("this shader has no GPU version")
	case a.shader == nil && !hasGPU:
		return nil, fmt.Errorf("%T is neither a Shader nor a GPUShader", program)
	}
	if hasGPU && opt.Backend != CPU {
		g, err := newGPURenderer(gs, opt.GPUWidth, opt.GPUHeight)
		if err != nil {
			return nil, err
		}
		a.gpu = g
	}
	if opt.Threads > 0 {
		a.renderer.Workers = opt.Threads
	}
	if st, ok := program.(Static); ok && st.Static() {
		a.renderer.Reuse = opt.Reuse
	}
	a.field, _ = program.(Field)
	a.spawn = DefaultPlayer()
	if sp, ok := program.(Spawner); ok {
		a.spawn = sp.Spawn()
	}
	a.player = a.spawn
	return a, nil
}

// bench renders Options.Bench frames back to back, waiting for each to
// finish (the GPU included), and prints the average.
func (a *App) bench(f *Frame, gpu bool) error {
	backend, w, h := "CPU", a.opt.Width, a.opt.Height
	if gpu {
		backend, w, h = "GPU", a.opt.GPUWidth, a.opt.GPUHeight
	}
	pix := make([]byte, w*h*4)
	t0 := time.Now()
	for range a.opt.Bench {
		if gpu {
			a.gpu.render(f)
			a.gpu.img.ReadPixels(pix)
		} else {
			a.renderer.Render(a.shader, f)
		}
	}
	per := time.Since(t0) / time.Duration(a.opt.Bench)
	fmt.Printf("%s %dx%d: %v per frame (%.0f fps)\n", backend, w, h, per.Round(time.Microsecond), 1/per.Seconds())
	return errQuit
}

// useGPU decides where this frame renders.
func (a *App) useGPU(f *Frame) bool {
	return a.gpu != nil && chooseGPU(a.program, a.opt.Backend, f)
}

func chooseGPU(program any, be Backend, f *Frame) bool {
	gs, ok := program.(GPUShader)
	if !ok || gs.Kage() == nil || be == CPU {
		return false
	}
	if _, cpu := program.(Shader); !cpu || be == GPU {
		return true
	}
	p, ok := program.(GPUPreferrer)
	return ok && p.PreferGPU(f)
}

// StartsOnGPU reports whether the first frame, from the spawn point, would
// render on the GPU.
func StartsOnGPU(program any, be Backend) bool {
	p := DefaultPlayer()
	if sp, ok := program.(Spawner); ok {
		p = sp.Spawn()
	}
	return chooseGPU(program, be, &Frame{Player: &p, Cam: p.Camera()})
}

func (a *App) Run() error {
	a.img = ebiten.NewImage(a.opt.Width, a.opt.Height)
	a.start = time.Now()
	a.last = a.start
	ebiten.SetWindowTitle(a.opt.Title)
	ebiten.SetWindowSize(1280, 1280*a.opt.Height/max(1, a.opt.Width))
	ebiten.SetWindowResizingMode(ebiten.WindowResizingModeEnabled)
	ebiten.SetFullscreen(a.opt.Fullscreen)
	ebiten.SetCursorMode(ebiten.CursorModeCaptured)
	ebiten.SetTPS(a.opt.FPS)
	ebiten.SetVsyncEnabled(true)
	err := ebiten.RunGame(a)
	if errors.Is(err, errQuit) {
		return nil
	}
	return err
}

func (a *App) Update() error {
	now := time.Now()
	dt := min(now.Sub(a.last).Seconds(), 0.1)
	a.last = now

	if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
		return errQuit
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyF) {
		ebiten.SetFullscreen(!ebiten.IsFullscreen())
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyTab) {
		a.dump()
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyP) {
		if path, err := a.screenshot(); err != nil {
			fmt.Println("screenshot:", err)
		} else {
			fmt.Println("saved", path)
		}
	}

	a.player.Step(a.readInput(dt), a.field, dt)

	f := &a.frame
	f.Time = now.Sub(a.start).Seconds()
	if a.opt.ShotPath != "" {
		f.Time = a.opt.ShotTime
	}
	f.Delta = dt
	f.Player = &a.player
	f.Cam = a.player.Camera()
	f.Prev = nil
	if f.Index > 0 && !a.onGPU {
		f.Prev = a.renderer.Last()
	}
	if u, ok := a.program.(Updater); ok {
		u.Update(f)
		f.Cam = a.player.Camera()
	}

	gpu := a.useGPU(f)
	if a.opt.Bench > 0 {
		return a.bench(f, gpu)
	}
	if st, ok := a.program.(Static); ok && st.Static() && f.Index > 0 && f.Cam == a.lastCam && gpu == a.onGPU {
		return nil // nothing changed; keep showing the last frame
	}
	a.lastCam = f.Cam
	a.onGPU = gpu
	if gpu {
		a.gpu.render(f)
	} else {
		a.renderer.Render(a.shader, f)
		a.img.WritePixels(a.renderer.RGBA)
	}
	f.Index++
	if a.opt.ShotPath != "" {
		if err := a.save(a.opt.ShotPath); err != nil {
			return err
		}
		return errQuit
	}
	return nil
}

func (a *App) readInput(dt float64) Input {
	var in Input
	key := func(k ebiten.Key) float64 {
		if ebiten.IsKeyPressed(k) {
			return 1
		}
		return 0
	}
	in.Move = V3(key(ebiten.KeyD)-key(ebiten.KeyA), key(ebiten.KeySpace)-key(ebiten.KeyMeta), key(ebiten.KeyW)-key(ebiten.KeyS))
	in.Jump = ebiten.IsKeyPressed(ebiten.KeySpace)
	in.Fast = ebiten.IsKeyPressed(ebiten.KeyShift)
	in.Grow = key(ebiten.KeyC) - key(ebiten.KeyZ)

	const turn = 1.5 // radians per second for keyboard rotation
	in.Yaw = (key(ebiten.KeyArrowRight) - key(ebiten.KeyArrowLeft)) * turn * dt
	in.Pitch = (key(ebiten.KeyArrowDown) - key(ebiten.KeyArrowUp)) * turn * dt
	const roll = 1.0 // radians per second
	in.Roll = (key(ebiten.KeyE) - key(ebiten.KeyQ)) * roll * dt

	mx, my := ebiten.CursorPosition()
	if a.mouseOK {
		const sens = 0.003
		in.Yaw += float64(mx-a.mouseX) * sens
		in.Pitch += float64(my-a.mouseY) * sens
	}
	a.mouseX, a.mouseY, a.mouseOK = mx, my, true
	return in
}

func (a *App) Draw(screen *ebiten.Image) {
	img := a.current()
	w, h := float64(img.Bounds().Dx()), float64(img.Bounds().Dy())
	sw, sh := float64(screen.Bounds().Dx()), float64(screen.Bounds().Dy())
	scale := min(sw/w, sh/h)
	op := &ebiten.DrawImageOptions{Filter: ebiten.FilterNearest}
	op.GeoM.Scale(scale, scale)
	op.GeoM.Translate((sw-scale*w)/2, (sh-scale*h)/2)
	screen.DrawImage(img, op)
}

func (a *App) current() *ebiten.Image {
	if a.onGPU {
		return a.gpu.img
	}
	return a.img
}

func (a *App) Layout(w, h int) (int, int) { return w, h }

func (a *App) screenshot() (string, error) {
	dir := a.opt.ShotDir
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	path := filepath.Join(dir, time.Now().Format("20060102-150405")+".png")
	return path, a.save(path)
}

func (a *App) save(path string) error {
	if !a.onGPU {
		return SavePNG(path, a.renderer.RGBA, a.opt.Width, a.opt.Height)
	}
	img := a.gpu.img
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

// Shot renders a single frame of a CPU shader from its spawn point without
// opening a window and saves it as a PNG. GPU shots need a window: use an App
// with Options.ShotPath.
func Shot(s Shader, w, h int, t float64, path string) (time.Duration, error) {
	r := NewRenderer(w, h)
	p := DefaultPlayer()
	if sp, ok := s.(Spawner); ok {
		p = sp.Spawn()
	}
	f := &Frame{Time: t, Player: &p, Cam: p.Camera()}
	if u, ok := s.(Updater); ok {
		u.Update(f)
		f.Cam = p.Camera()
	}
	t0 := time.Now()
	r.Render(s, f)
	return time.Since(t0), SavePNG(path, r.RGBA, w, h)
}

// dump prints the player and what the crosshair is looking at, in a form that
// can be pasted back into a shader's spawn point.
func (a *App) dump() {
	p := a.player
	fmt.Printf("Pos:   shad.V3(%.17g, %.17g, %.17g)\nFwd:   shad.V3(%.17g, %.17g, %.17g)\nUp:    shad.V3(%.17g, %.17g, %.17g)\nScale: %.17g\n",
		p.Pos.X, p.Pos.Y, p.Pos.Z, p.Fwd.X, p.Fwd.Y, p.Fwd.Z, p.Up.X, p.Up.Y, p.Up.Z, p.Scale)
	if a.frame.Index > 0 && !a.onGPU {
		c := a.renderer.Last().Center()
		if c.Depth > 1e300 {
			fmt.Println("Crosshair: nothing")
			return
		}
		fmt.Printf("Crosshair: depth %.6g (%.3g sizes), pos (%.9g, %.9g, %.9g), normal (%.3f, %.3f, %.3f), id %d, steps %d\n",
			c.Depth, c.Depth/p.Scale, c.Pos.X, c.Pos.Y, c.Pos.Z, c.Normal.X, c.Normal.Y, c.Normal.Z, c.ID, c.Steps)
		return
	}
	// The GPU frame can't be read back; ask the field instead.
	if a.field == nil {
		return
	}
	t, ok := Raycast(a.field, p.Pos, p.Fwd, p.Scale*1e4, p.Detail())
	if !ok {
		fmt.Println("Crosshair (field): nothing")
		return
	}
	hit := p.Pos.Add(p.Fwd.Mul(t))
	n := Gradient(a.field, hit, p.Detail())
	fmt.Printf("Crosshair (field): depth %.6g (%.3g sizes), pos (%.9g, %.9g, %.9g), normal (%.3f, %.3f, %.3f)\n",
		t, t/p.Scale, hit.X, hit.Y, hit.Z, n.X, n.Y, n.Z)
}
