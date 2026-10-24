package shad

import "math"

// resController picks the render width that keeps frames within a time
// budget. Cost is roughly proportional to the pixel count, so the width that
// would just fit is w·sqrt(budget/took). Going over budget shrinks the width
// at once (after a few frames, so hiccups don't count), to keep the frame
// rate steady when the view gets expensive;
// coming in under it grows the width slowly, in coarse steps, so the size
// doesn't oscillate and buffers aren't reallocated every frame.
type resController struct {
	target   float64 // ideal width, smoothed
	cur      int     // width in use
	min, max int
	budget   float64 // seconds of render time per frame
	cooldown int     // frames until the next resize is allowed
	urgent   bool    // frames are over budget: shrink right away
	over     int     // consecutive frames over budget
}

// resOverFrames is how many frames in a row must run over budget before the
// width drops at once: a heavier view does that, a one-off hiccup (a GC
// pause, saving a screenshot) doesn't, and reacting to hiccups would
// ratchet the width down for good while the camera sits still (when it may
// shrink but not grow).
const resOverFrames = 3

const (
	resStep     = 16 // widths are multiples of this
	resCooldown = 30 // frames after a resize before growing again
	resHold     = 2  // steps the target must move before a non-urgent resize
)

func newResController(start, lo, hi int, budget float64) *resController {
	c := &resController{min: lo, max: hi, budget: budget}
	c.target = float64(c.clamp(start))
	c.cur = c.quantize(c.target)
	return c
}

func (c *resController) clamp(w int) int { return max(c.min, min(c.max, w)) }

func (c *resController) quantize(w float64) int {
	return c.clamp(int(math.Round(w/resStep)) * resStep)
}

// observe feeds the render time of a frame rendered at the current width.
func (c *resController) observe(took float64) {
	if took <= 0 {
		return
	}
	ideal := float64(c.cur) * math.Sqrt(c.budget/took)
	if took > c.budget*1.1 {
		c.over++
	} else {
		c.over = 0
	}
	if c.over >= resOverFrames {
		// Over budget for real: go straight there, with a little margin,
		// and resize next frame regardless of the cooldown.
		c.target = ideal * 0.95
		c.urgent = true
		c.over = 0
	} else {
		// Ease toward the ideal; a single hiccup moves it only a little.
		c.target += (math.Min(ideal, 2*float64(c.cur)) - c.target) * 0.1
	}
	c.target = math.Max(float64(c.min), math.Min(float64(c.max), c.target))
}

// width returns the width to render the next frame at. Unless the last
// frame was over budget, it only changes when canResize, which the app
// passes while the camera moves: a resize re-samples the image on a new
// pixel grid, which motion hides and a still view doesn't.
func (c *resController) width(canResize bool) int {
	w := c.quantize(c.target)
	if c.urgent && w < c.cur {
		c.urgent = false
		c.cur = w
		c.cooldown = resCooldown
		return c.cur
	}
	c.urgent = false
	if !canResize {
		return c.cur
	}
	if c.cooldown > 0 {
		c.cooldown--
		return c.cur
	}
	// Every resize re-samples the image on a new pixel grid, which reads as
	// jitter, so ignore small drifts of the target.
	if w >= c.cur+resHold*resStep || w <= c.cur-resHold*resStep {
		c.cur = w
		c.cooldown = resCooldown
	}
	return c.cur
}

// setMax changes the upper bound, e.g. when the window is resized.
func (c *resController) setMax(hi int) {
	c.max = max(c.min, hi)
	c.target = math.Min(c.target, float64(c.max))
	c.cur = c.clamp(c.cur)
}
