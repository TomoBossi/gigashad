package shad

import (
	"math"
	"testing"
)

// The controller settles on the width that fits the budget when cost grows
// with the pixel count, and follows when the scene gets more expensive.
func TestResControllerHoldsBudget(t *testing.T) {
	const budget = 1.0 / 30
	c := newResController(240, 96, 640, budget)
	costPerPixel := budget / (400 * 225) // 400 px wide fits exactly at 16:9
	run := func(frames int) (int, float64) {
		var took float64
		for range frames {
			w := c.width()
			h := float64(w) * 9 / 16
			took = float64(w) * h * costPerPixel
			c.observe(took)
		}
		return c.cur, took
	}

	w, took := run(200)
	if math.Abs(took-budget)/budget > 0.15 {
		t.Errorf("settled at width %d taking %.1f ms, budget %.1f ms", w, took*1e3, budget*1e3)
	}
	if w%resStep != 0 {
		t.Errorf("width %d is not a multiple of %d", w, resStep)
	}

	costPerPixel *= 4 // an expensive region: half the width fits
	w2, took := run(200)
	if math.Abs(took-budget)/budget > 0.15 || math.Abs(float64(w2)-float64(w)/2) > resStep {
		t.Errorf("after cost x4: width %d taking %.1f ms, want about %d", w2, took*1e3, w/2)
	}

	costPerPixel /= 1000 // trivially cheap: clamps at the maximum
	if w3, _ := run(200); w3 != 640 {
		t.Errorf("cheap scene: width %d, want the 640 maximum", w3)
	}
}
