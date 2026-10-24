package shad

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
)

// Stock actions, for Def.Bindings.

// Quit closes the app.
func Quit(c *Context) {
	if c.app != nil {
		c.app.quit = true
	}
}

// ToggleFullscreen switches between window and fullscreen.
func ToggleFullscreen(c *Context) {
	if c.app != nil {
		ebiten.SetFullscreen(!ebiten.IsFullscreen())
	}
}

// Screenshot saves the current frame as a PNG in the shots directory.
func Screenshot(c *Context) {
	path, err := c.Screenshot("")
	report("screenshot", path, err)
}

// SaveData writes a text report of the scene (camera, render, what the
// crosshair is looking at, and Def.Data) to the shots directory and prints
// it.
func SaveData(c *Context) {
	text := c.Report()
	fmt.Print(text)
	path, err := c.SaveText("", text)
	report("scene data", path, err)
}

// StandardBindings are Esc to quit, F for fullscreen, P for a screenshot and
// Tab to save scene data.
func StandardBindings() []Binding {
	return []Binding{
		{Keys: []Key{ebiten.KeyEscape}, Help: "quit", Action: Quit},
		{Keys: []Key{ebiten.KeyF}, Help: "fullscreen", Action: ToggleFullscreen},
		{Keys: []Key{ebiten.KeyP}, Help: "screenshot", Action: Screenshot},
		{Keys: []Key{ebiten.KeyTab}, Help: "save scene data", Action: SaveData},
	}
}

func report(what, path string, err error) {
	if err != nil {
		fmt.Printf("%s: %v\n", what, err)
	} else if path != "" {
		fmt.Println("saved", path)
	}
}

// Context services. Outside the app (-shot frames) they do nothing.

// Screenshot saves the current frame as a PNG at path, or in the shots
// directory if path is empty, and returns where it went.
func (c *Context) Screenshot(path string) (string, error) {
	if c.app == nil {
		return "", nil
	}
	path, err := c.app.outPath(path, ".png")
	if err != nil {
		return "", err
	}
	return path, c.app.save(path)
}

// SaveText writes text to path, or to a timestamped file in the shots
// directory if path is empty, and returns where it went.
func (c *Context) SaveText(path, text string) (string, error) {
	if c.app == nil {
		return "", nil
	}
	path, err := c.app.outPath(path, ".txt")
	if err != nil {
		return "", err
	}
	return path, os.WriteFile(path, []byte(text), 0o644)
}

// Last returns the most recent CPU frame's G-buffer, or nil if the last frame
// came from the GPU or there wasn't one yet.
func (c *Context) Last() *GBuffer {
	if c.app == nil || c.app.onGPU || c.app.renderer == nil || c.Frame.Index == 0 {
		return nil
	}
	return c.app.renderer.Last()
}

// Field returns the shader's distance field, or nil.
func (c *Context) Field() Field {
	if c.app == nil {
		return nil
	}
	return c.app.def.Field
}

// Report describes the scene as text: what SaveData writes.
func (c *Context) Report() string {
	var b strings.Builder
	cam := &c.Frame.Cam
	fmt.Fprintf(&b, "Camera\n  pos   %.17g %.17g %.17g\n  fwd   %.17g %.17g %.17g\n  up    %.17g %.17g %.17g\n  scale %.17g\n  fovy  %.6g\n",
		cam.Pos.X, cam.Pos.Y, cam.Pos.Z, cam.Fwd.X, cam.Fwd.Y, cam.Fwd.Z, cam.Up.X, cam.Up.Y, cam.Up.Z, cam.Scale, cam.FovY)
	if a := c.app; a != nil {
		backend := "CPU"
		if a.onGPU {
			backend = "GPU"
		}
		fmt.Fprintf(&b, "Render\n  %s %dx%d, %.1f ms\n", backend, a.lastW, a.lastH, a.took.Seconds()*1e3)
	}
	b.WriteString(c.crosshair())
	if c.app != nil && c.app.def.Data != nil {
		b.WriteString(c.app.def.Data(c))
	}
	return b.String()
}

// crosshair describes what the center of the view hits: from the G-buffer
// after a CPU frame, by raycasting the field otherwise.
func (c *Context) crosshair() string {
	cam := &c.Frame.Cam
	scale := cam.Scale
	if scale == 0 {
		scale = 1
	}
	if g := c.Last(); g != nil {
		s := g.Center()
		if s.Depth > 1e300 {
			return "Crosshair\n  nothing\n"
		}
		return fmt.Sprintf("Crosshair (G-buffer)\n  depth  %.6g (%.3g sizes)\n  pos    %.9g %.9g %.9g\n  normal %.3f %.3f %.3f\n  id %d, steps %d\n",
			s.Depth, s.Depth/scale, s.Pos.X, s.Pos.Y, s.Pos.Z, s.Normal.X, s.Normal.Y, s.Normal.Z, s.ID, s.Steps)
	}
	field := c.Field()
	if field == nil {
		return ""
	}
	detail := scale / 32
	t, ok := Raycast(field, cam.Pos, cam.Fwd, scale*1e4, detail)
	if !ok {
		return "Crosshair (field)\n  nothing\n"
	}
	hit := cam.Pos.Add(cam.Fwd.Mul(t))
	n := Gradient(field, hit, detail)
	return fmt.Sprintf("Crosshair (field)\n  depth  %.6g (%.3g sizes)\n  pos    %.9g %.9g %.9g\n  normal %.3f %.3f %.3f\n",
		t, t/scale, hit.X, hit.Y, hit.Z, n.X, n.Y, n.Z)
}

func (a *App) outPath(path, ext string) (string, error) {
	if path != "" {
		return path, nil
	}
	if err := os.MkdirAll(a.opt.ShotDir, 0o755); err != nil {
		return "", err
	}
	return filepath.Join(a.opt.ShotDir, time.Now().Format("20060102-150405.000")+ext), nil
}
