package gopher

import (
	"testing"

	"github.com/hajimehoshi/ebiten/v2"

	"gigashad/shad"
)

// Flying straight ahead from the start, into the gopher in front, the player
// slides around it and never ends up inside it.
func TestCollidesWithGopher(t *testing.T) {
	d := New()
	f := shad.FirstFrame(&d, 0)
	start := f.Cam.Pos
	for i := range 30 * 10 {
		shad.RunFrame(&d, f, shad.ScriptedInput(1.0/30, ebiten.KeyW))
		if dist := d.Field.Dist(f.Cam.Pos, f.Cam.Scale/32); dist < 0.8*f.Cam.Scale/2 {
			t.Fatalf("frame %d: inside a gopher, %.3f from its surface", i, dist)
		}
	}
	if moved := f.Cam.Pos.Sub(start); moved.X == 0 && moved.Y == 0 {
		t.Errorf("never deflected: collisions didn't happen")
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
