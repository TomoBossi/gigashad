package explorer

import (
	"testing"

	"github.com/hajimehoshi/ebiten/v2"

	"gigashad/shad"
)

// Flying ahead from the start for a few seconds never ends up inside the
// fractal.
func TestFlyingStaysOutside(t *testing.T) {
	d := New()
	f := shad.FirstFrame(&d, 0)
	for i := range 30 * 5 {
		f.Time += 1.0 / 30
		shad.RunFrame(&d, f, shad.ScriptedInput(1.0/30, ebiten.KeyW, ebiten.KeyShift))
		if dist := d.Field.Dist(f.Cam.Pos, f.Cam.Scale/32); dist < 0 {
			t.Fatalf("frame %d: inside the fractal (%.3f)", i, dist)
		}
	}
}

// Holding = narrows the view and - widens it.
func TestZoomKeys(t *testing.T) {
	d := New()
	f := shad.FirstFrame(&d, 0)
	start := f.Cam.FovY
	for range 30 {
		shad.RunFrame(&d, f, shad.ScriptedInput(1.0/30, ebiten.KeyEqual))
	}
	if f.Cam.FovY >= start {
		t.Errorf("= didn't narrow the view: %.3f -> %.3f", start, f.Cam.FovY)
	}
	for range 60 {
		shad.RunFrame(&d, f, shad.ScriptedInput(1.0/30, ebiten.KeyMinus))
	}
	if f.Cam.FovY <= start {
		t.Errorf("- didn't widen the view past the start: %.3f -> %.3f", start, f.Cam.FovY)
	}
}
