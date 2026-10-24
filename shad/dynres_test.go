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
			w := c.width(true)
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

// A sudden jump in cost (an expensive region coming into view) is absorbed
// within a couple of frames, and the size doesn't jitter with timing noise.
func TestResControllerReactsFast(t *testing.T) {
	const budget = 1.0 / 30
	c := newResController(400, 96, 640, budget)
	costPerPixel := 0.9 * budget / (400 * 225)
	frame := func(noise float64) float64 {
		w := c.width(true)
		took := float64(w) * float64(w) * 9 / 16 * costPerPixel * noise
		c.observe(took)
		return took
	}
	for i := range 100 {
		frame(1 + 0.08*math.Sin(float64(i)*1.7)) // ±8% noise
	}
	sizes := map[int]bool{}
	for i := range 100 {
		frame(1 + 0.08*math.Sin(float64(i)*1.7))
		sizes[c.cur] = true
	}
	if len(sizes) > 1 {
		t.Errorf("with timing noise only, the width took %d different values", len(sizes))
	}

	costPerPixel *= 3
	var over int
	for i := range 30 {
		if took := frame(1); took > budget*1.15 && i >= resOverFrames+1 {
			over++
		}
	}
	if over > 0 {
		t.Errorf("after a 3x cost jump, %d frames were still over budget once it reacted", over)
	}
}

// Occasional one-frame spikes (a GC pause, saving a screenshot) don't
// ratchet the width down, even when the camera mostly sits still (when the
// width may shrink but not grow): it stays near what the steady cost allows.
func TestResControllerIgnoresSpikes(t *testing.T) {
	const budget = 1.0 / 30
	c := newResController(400, 96, 640, budget)
	costPerPixel := budget / (400 * 225) // 400 wide fits exactly
	for i := range 3000 {
		moving := i%300 < 30 // a second of movement every ten
		w := c.width(moving)
		took := float64(w) * float64(w) * 9 / 16 * costPerPixel
		if i%40 == 0 {
			took *= 4 // a hiccup
		}
		c.observe(took)
	}
	if c.cur < 360 {
		t.Errorf("after occasional spikes the width fell to %d; the steady cost allows 400", c.cur)
	}
}
