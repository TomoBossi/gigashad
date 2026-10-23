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

// TestScaleInvariance dives toward a surface point, shrinking the observer
// 10x at a time and keeping it the same number of sizes away, and checks that
// a frame costs about the same at every scale. Set GIGASHAD_SHOTS to a
// directory to also save the frames.
func TestScaleInvariance(t *testing.T) {
	e := newExplorer()
	p := startPlayer()
	target, ok := shad.Raycast(e, p.Pos, p.Fwd, 10, 1e-12)
	if !ok {
		t.Fatal("spawn view hits nothing")
	}
	hit := p.Pos.Add(p.Fwd.Mul(target))

	r := shad.NewRenderer(320, 180)
	var base time.Duration
	for i := range 8 {
		p.Scale = 2e-3 * math.Pow(10, -float64(i))
		p.Pos = hit.Sub(p.Fwd.Mul(40 * p.Scale))
		f := &shad.Frame{Cam: p.Camera()}
		t0 := time.Now()
		r.Render(e, f)
		took := time.Since(t0)
		if i == 0 {
			base = took
		}
		hits, steps := 0, 0
		for _, s := range r.Last().Samples {
			if s.ID != 0 {
				hits++
			}
			steps += s.Steps
		}
		n := len(r.Last().Samples)
		t.Logf("scale %.0e  %4.0f ms  iters at camera %3d  hit %3.0f%%  avg steps %.1f",
			p.Scale, float64(took.Microseconds())/1000, e.F.Iters(e.Detail*p.Scale), 100*float64(hits)/float64(n), float64(steps)/float64(n))
		if dir := os.Getenv("GIGASHAD_SHOTS"); dir != "" {
			shad.SavePNG(filepath.Join(dir, fmt.Sprintf("scale%d.png", i)), r.RGBA, r.W, r.H)
		}
		if took > base*4 {
			t.Errorf("scale %.0e took %v, more than 4x the %v at the start", p.Scale, took, base)
		}
	}
}
