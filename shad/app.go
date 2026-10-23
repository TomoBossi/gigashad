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

// Options configure an App.
type Options struct {
	Width, Height int // render resolution
	FPS           int
	Threads       int // render goroutines; 0 means one per CPU core
	Reuse         int // on static shaders, shade 1 pixel in Reuse per frame (see Renderer.Reuse)
	Fullscreen    bool
	Title         string
	ShotDir       string // where P saves screenshots
}

// App owns the window, the player and the per-frame loop.
type App struct {
	opt      Options
	shader   Shader
	field    Field
	renderer *Renderer
	img      *ebiten.Image
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

// NewApp prepares an App; Run opens the window.
func NewApp(s Shader, opt Options) *App {
	a := &App{opt: opt, shader: s, renderer: NewRenderer(opt.Width, opt.Height)}
	if opt.Threads > 0 {
		a.renderer.Workers = opt.Threads
	}
	if st, ok := s.(Static); ok && st.Static() {
		a.renderer.Reuse = opt.Reuse
	}
	a.field, _ = s.(Field)
	a.spawn = DefaultPlayer()
	if sp, ok := s.(Spawner); ok {
		a.spawn = sp.Spawn()
	}
	a.player = a.spawn
	return a
}

func (a *App) Run() error {
	a.img = ebiten.NewImage(a.opt.Width, a.opt.Height)
	a.start = time.Now()
	a.last = a.start
	ebiten.SetWindowTitle(a.opt.Title)
	ebiten.SetWindowSize(1280, 1280*a.opt.Height/a.opt.Width)
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
	f.Delta = dt
	f.Player = &a.player
	f.Cam = a.player.Camera()
	if f.Index > 0 {
		f.Prev = a.renderer.Last()
	}
	if u, ok := a.shader.(Updater); ok {
		u.Update(f)
		f.Cam = a.player.Camera()
	}

	if st, ok := a.shader.(Static); ok && st.Static() && f.Index > 0 && f.Cam == a.lastCam {
		return nil // nothing changed; keep showing the last frame
	}
	a.lastCam = f.Cam
	a.renderer.Render(a.shader, f)
	a.img.WritePixels(a.renderer.RGBA)
	f.Index++
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
	sw, sh := screen.Bounds().Dx(), screen.Bounds().Dy()
	scale := min(float64(sw)/float64(a.opt.Width), float64(sh)/float64(a.opt.Height))
	op := &ebiten.DrawImageOptions{Filter: ebiten.FilterNearest}
	op.GeoM.Scale(scale, scale)
	op.GeoM.Translate((float64(sw)-scale*float64(a.opt.Width))/2, (float64(sh)-scale*float64(a.opt.Height))/2)
	screen.DrawImage(a.img, op)
}

func (a *App) Layout(w, h int) (int, int) { return w, h }

func (a *App) screenshot() (string, error) {
	dir := a.opt.ShotDir
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	path := filepath.Join(dir, time.Now().Format("20060102-150405")+".png")
	return path, SavePNG(path, a.renderer.RGBA, a.opt.Width, a.opt.Height)
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

// Shot renders a single frame from the shader's spawn point without opening a
// window and saves it as a PNG.
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
	if a.frame.Index == 0 {
		return
	}
	c := a.renderer.Last().Center()
	if c.Depth > 1e300 {
		fmt.Println("Crosshair: nothing")
		return
	}
	fmt.Printf("Crosshair: depth %.6g (%.3g sizes), pos (%.9g, %.9g, %.9g), normal (%.3f, %.3f, %.3f), id %d, steps %d\n",
		c.Depth, c.Depth/p.Scale, c.Pos.X, c.Pos.Y, c.Pos.Z, c.Normal.X, c.Normal.Y, c.Normal.Z, c.ID, c.Steps)
}
