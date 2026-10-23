package oolacile

import (
	"math"
	"slices"
	"testing"

	"gigashad/shad"
)

// TestZoomIsSmooth keeps the camera still and halves the observer's size in
// small steps. Only scale-dependent things change (detail, coloring, glow,
// fog distance), so every frame should look much like the last; a pop (an
// iteration count crossing an integer) shows up as one frame-to-frame change
// far above the typical one. Before iterations and traps were fractional,
// such a pop changed the image by 1.6/255 on average in a single frame;
// blended, the change around a crossing peaks at about 0.16.
func TestZoomIsSmooth(t *testing.T) {
	e := newExplorer()
	p := startPlayer()
	target, ok := shad.Raycast(e, p.Pos, p.Fwd, 10, 1e-12)
	if !ok {
		t.Fatal("spawn view hits nothing")
	}
	hit := p.Pos.Add(p.Fwd.Mul(target))

	const frames = 60
	r := shad.NewRenderer(160, 90)
	var prev []byte
	var diffs []float64
	for i := range frames {
		p.Scale = 2e-4 * math.Pow(2, -float64(i)/frames)
		p.Pos = hit.Sub(p.Fwd.Mul(20 * 2e-4))
		r.Render(e, &shad.Frame{Cam: p.Camera()})
		if prev != nil {
			var d float64
			for k := range prev {
				d += math.Abs(float64(prev[k]) - float64(r.RGBA[k]))
			}
			diffs = append(diffs, d/float64(len(prev)))
		}
		prev = slices.Clone(r.RGBA)
	}
	sorted := slices.Clone(diffs)
	slices.Sort(sorted)
	median, worst := sorted[len(sorted)/2], sorted[len(sorted)-1]
	t.Logf("frame-to-frame change: median %.2f, worst %.2f", median, worst)
	if worst > 0.5 {
		t.Errorf("zooming pops: a frame changes by %.2f/255 on average", worst)
	}
}
