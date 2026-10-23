package oolacile

import (
	"testing"

	"gigashad/fractal"
	"gigashad/shad"
)

// Without movement input the player must not move: not while growing, not
// while shrinking, and not while resting against a wall.
func TestPlayerStaysPutWithoutMoveInput(t *testing.T) {
	e := New().(*fractal.Explorer)
	const dt = 1.0 / 24

	for _, grow := range []float64{1, -1} {
		p := e.Spawn()
		start := p.Pos
		for range 24 * 6 {
			p.Step(shad.Input{Grow: grow}, e, dt)
		}
		if moved := p.Pos.Sub(start).Len(); moved != 0 {
			t.Errorf("grow %v for 6s moved the player %.3g (%.2g sizes)", grow, moved, moved/p.Scale)
		}
	}

	// Fly forward into the wall the spawn view faces, then let go.
	p := e.Spawn()
	for range 24 * 20 {
		p.Step(shad.Input{Move: shad.V3(0, 0, 1), Fast: true}, e, dt)
	}
	if d := e.Dist(p.Pos, p.Detail()); d < p.Radius()*0.9 {
		t.Errorf("player ended inside geometry: dist %.3g, radius %.3g", d, p.Radius())
	}
	rest := p.Pos
	for range 24 * 3 {
		p.Step(shad.Input{}, e, dt)
	}
	if moved := p.Pos.Sub(rest).Len(); moved > p.Scale*1e-3 {
		t.Errorf("player drifted %.2g sizes while resting against a wall", moved/p.Scale)
	}
}
