package oolacile

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"testing"
	"time"

	"gigashad/shad"
)

// TestReuse walks and turns for a second at 24 fps with partial shading on,
// then compares the last frame to a full render of that view. Reused pixels
// keep the sub-pixel position they were shaded at, which on fractal detail
// alone accounts for a mean difference of about 3.5/255; much more than that
// means stale content. Set GIGASHAD_SHOTS to a directory to also save the
// frames.
func TestReuse(t *testing.T) {
	s := New()
	p := s.(shad.Spawner).Spawn()
	const w, h = 240, 135
	run := func(reuse int) (*shad.Renderer, time.Duration) {
		q := p
		r := shad.NewRenderer(w, h)
		r.Reuse = reuse
		var took time.Duration
		for i := range 24 {
			q.Pos = q.Pos.Add(q.Fwd.Mul(3 * q.Scale / 24))
			q.Look(0.015, 0.005, 0.01)
			f := &shad.Frame{Player: &q, Cam: q.Camera()}
			t0 := time.Now()
			r.Render(s, f)
			if i > 0 {
				took += time.Since(t0)
			}
		}
		if dir := os.Getenv("GIGASHAD_SHOTS"); dir != "" {
			shad.SavePNG(filepath.Join(dir, fmt.Sprintf("reuse%d.png", reuse)), r.RGBA, w, h)
		}
		return r, took / 23
	}
	full, fullTook := run(1)
	for _, n := range []int{2, 4} {
		part, partTook := run(n)
		var diff float64
		for i := range full.RGBA {
			diff += math.Abs(float64(full.RGBA[i]) - float64(part.RGBA[i]))
		}
		diff /= float64(len(full.RGBA))
		t.Logf("reuse %d: %v per frame vs %v  reused %.0f%%  mean diff %.2f/255", n, partTook, fullTook, 100*part.Reused(), diff)
		if diff > 5 {
			t.Errorf("reuse %d drifted from a full render", n)
		}
	}
}

// BenchmarkFrame renders the spawn view at the default resolution.
func BenchmarkFrame(b *testing.B) {
	s := New()
	p := s.(shad.Spawner).Spawn()
	r := shad.NewRenderer(240, 135)
	r.Workers = 3
	f := &shad.Frame{Player: &p, Cam: p.Camera()}
	for b.Loop() {
		r.Render(s, f)
	}
}
