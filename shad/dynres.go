package shad

import "math"

// resController picks the render width that keeps frames within a time
// budget. Cost is roughly proportional to the pixel count, so the width that
// would just fit is w·sqrt(budget/took); the controller eases toward it and
// only resizes in coarse steps, so buffers aren't reallocated every frame.
type resController struct {
	target   float64 // ideal width, smoothed
	cur      int     // width in use
	min, max int
	budget   float64 // seconds of render time per frame
	cooldown int     // frames until the next resize is allowed
}

const (
	resStep     = 16 // widths are multiples of this
	resCooldown = 8  // frames between resizes
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
	c.target += (ideal - c.target) * 0.3
	c.target = math.Max(float64(c.min), math.Min(float64(c.max), c.target))
}

// width returns the width to render the next frame at.
func (c *resController) width() int {
	if c.cooldown > 0 {
		c.cooldown--
		return c.cur
	}
	if w := c.quantize(c.target); w != c.cur {
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
