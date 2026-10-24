package oolacile

import (
	"testing"

	"github.com/hajimehoshi/ebiten/v2"

	"gigashad/shad"
)

// Without movement input the player must not move: not while growing, not
// while shrinking, and not while resting against a wall.
func TestPlayerStaysPutWithoutMoveInput(t *testing.T) {
	e := newExplorer()
	const dt = 1.0 / 24

	for _, grow := range []float64{1, -1} {
		p := startPlayer()
		start := p.Pos
		for range 24 * 6 {
			p.Step(shad.Intent{Grow: grow}, e, dt)
		}
		if moved := p.Pos.Sub(start).Len(); moved != 0 {
			t.Errorf("grow %v for 6s moved the player %.3g (%.2g sizes)", grow, moved, moved/p.Scale)
		}
	}

	// Fly forward into the wall the spawn view faces, then let go.
	p := startPlayer()
	for range 24 * 20 {
		p.Step(shad.Intent{Move: shad.V3(0, 0, 1), Fast: true}, e, dt)
	}
	if d := e.Dist(p.Pos, p.Detail()); d < p.Radius()*0.9 {
		t.Errorf("player ended inside geometry: dist %.3g, radius %.3g", d, p.Radius())
	}
	rest := p.Pos
	for range 24 * 3 {
		p.Step(shad.Intent{}, e, dt)
	}
	if moved := p.Pos.Sub(rest).Len(); moved > p.Scale*1e-3 {
		t.Errorf("player drifted %.2g sizes while resting against a wall", moved/p.Scale)
	}
}

// R puts everything back as it was at startup.
func TestReset(t *testing.T) {
	d := New(DefaultOptions())
	f := shad.FirstFrame(&d, 0)
	start := f.Cam

	for range 24 {
		shad.RunFrame(&d, f, shad.ScriptedInput(1.0/24, ebiten.KeyW, ebiten.KeyZ, ebiten.KeyE, ebiten.KeyArrowLeft))
		f.Time += 1.0 / 24
	}
	if f.Cam == start {
		t.Fatal("flying, shrinking and turning didn't change the camera")
	}
	shad.RunFrame(&d, f, shad.ScriptedInput(1.0/24, ebiten.KeyR))
	if f.Cam != start {
		t.Errorf("after R the camera is %+v, want the startup %+v", f.Cam, start)
	}
}

// Every movement and look key does something on its own.
func TestEveryControlKeyWorks(t *testing.T) {
	k := keys()
	controls := map[string][]shad.Key{
		"forward": k.Forward, "back": k.Back, "left": k.Left, "right": k.Right,
		"up": k.Up, "down": k.Down, "grow": k.Grow, "shrink": k.Shrink,
		"yaw left": k.YawLeft, "yaw right": k.YawRight, "pitch up": k.PitchUp,
		"pitch down": k.PitchDown, "roll left": k.RollLeft, "roll right": k.RollRight,
	}
	for name, ks := range controls {
		if len(ks) == 0 {
			t.Errorf("%s has no key", name)
			continue
		}
		for _, key := range ks {
			d := New(DefaultOptions())
			f := shad.FirstFrame(&d, 0)
			start := f.Cam
			for range 6 {
				shad.RunFrame(&d, f, shad.ScriptedInput(1.0/30, key))
			}
			if f.Cam == start {
				t.Errorf("%s (key %v) didn't change the camera", name, key)
			}
		}
	}
}

// With no input the camera stays exactly the same, bit for bit: the app
// treats any change as movement.
func TestCameraStillWithoutInput(t *testing.T) {
	d := New(DefaultOptions())
	f := shad.FirstFrame(&d, 0)
	for i := range 30 {
		prev := f.Cam
		shad.RunFrame(&d, f, shad.ScriptedInput(1.0/30))
		if f.Cam != prev {
			t.Fatalf("frame %d: the camera changed with no input", i)
		}
	}
}
