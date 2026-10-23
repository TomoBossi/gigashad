package shad

import (
	"math"
	"runtime"
	"sync"
	"sync/atomic"
)

// Renderer runs a Shader over every pixel of a small framebuffer, splitting
// rows between all CPU cores. The window then stretches that framebuffer, so
// the cost depends only on the render resolution.
type Renderer struct {
	W, H    int
	RGBA    []byte // W*H*4, row 0 at the top, ready for upload
	cur     *GBuffer
	prev    *GBuffer
	hasPrev bool
	src     []int32   // per pixel, index of the previous sample splatted there, or -1
	srcDist []float64 // distance of that sample, +Inf for misses
	prevCam Camera
	Workers int
	// Reuse > 1 shades only one pixel in Reuse each frame, cycling through
	// a fixed pattern (2: checkerboard, 4: 2x2 block), and fills the rest from
	// the previous frame wherever the reprojection lands on them; pixels it
	// misses are shaded anyway. A sample is reused at most Reuse-1 frames in a
	// row. Only valid for Static shaders.
	Reuse int
	phase int
}

func NewRenderer(w, h int) *Renderer {
	newG := func() *GBuffer { return &GBuffer{W: w, H: h, Samples: make([]Sample, w*h)} }
	return &Renderer{
		W: w, H: h,
		RGBA:    make([]byte, w*h*4),
		cur:     newG(),
		prev:    newG(),
		src:     make([]int32, w*h),
		srcDist: make([]float64, w*h),
		Workers: runtime.NumCPU(),
	}
}

// Last returns the G-buffer of the most recently rendered frame.
func (r *Renderer) Last() *GBuffer { return r.prev }

// Render shades one frame. Rows are handed out one at a time from a shared
// counter, which keeps cores busy even when cost varies wildly across the
// image (as it does with fractals).
func (r *Renderer) Render(s Shader, f *Frame) {
	f.Res = Vec2{float64(r.W), float64(r.H)}
	if r.Reuse > 1 && r.hasPrev {
		r.reproject(&f.Cam)
	}
	var next atomic.Int64
	var wg sync.WaitGroup
	for range r.Workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				y := int(next.Add(1) - 1)
				if y >= r.H {
					return
				}
				r.renderRow(s, f, y)
			}
		}()
	}
	wg.Wait()
	r.cur, r.prev = r.prev, r.cur
	r.prevCam = f.Cam
	r.hasPrev = true
	r.phase++
}

// reproject finds, for each pixel of the current camera, the previous
// frame's sample that lands on it: surfaces by their position, misses (sky,
// fog) by their direction, as if infinitely far away. The nearest wins.
func (r *Renderer) reproject(c *Camera) {
	for i := range r.src {
		r.src[i] = -1
		r.srcDist[i] = math.Inf(1)
	}
	res := Vec2{float64(r.W), float64(r.H)}
	k := 1 / c.PixelAngle(res)
	for i := range r.prev.Samples {
		s := &r.prev.Samples[i]
		// Surfaces reproject by position; misses (sky, fog) by direction, as
		// if they were infinitely far away.
		var v Vec3
		dist := math.Inf(1)
		if s.ID != 0 {
			v = s.Pos.Sub(c.Pos)
			dist = v.Len()
		} else {
			v = r.prevCam.Ray(Vec2{float64(i%r.W) + 0.5, float64(i/r.W) + 0.5}, res)
		}
		z := v.Dot(c.Fwd)
		if z <= 0 {
			continue
		}
		x := int(v.Dot(c.Right)/z*k + float64(r.W)/2)
		y := int(v.Dot(c.Up)/z*k + float64(r.H)/2)
		if x < 0 || y < 0 || x >= r.W || y >= r.H {
			continue
		}
		j := y*r.W + x
		if r.src[j] < 0 || dist < r.srcDist[j] {
			r.src[j] = int32(i)
			r.srcDist[j] = dist
		}
	}
}

func (r *Renderer) renderRow(s Shader, f *Frame, y int) {
	row := r.cur.Samples[y*r.W : (y+1)*r.W]
	top := (r.H - 1 - y) * r.W * 4
	reuse := r.Reuse > 1 && r.hasPrev
	for x := range row {
		out := &row[x]
		src := -1
		if reuse && r.pattern(x, y) != r.phase%r.Reuse {
			src = r.reusable(x, y)
		}
		if src >= 0 {
			*out = r.prev.Samples[src]
			out.reused++
		} else {
			*out = Sample{Depth: math.Inf(1)}
			s.Fragment(Vec2{float64(x) + 0.5, float64(y) + 0.5}, f, out)
		}
		c := out.Color
		i := top + x*4
		r.RGBA[i] = toByte(c.X)
		r.RGBA[i+1] = toByte(c.Y)
		r.RGBA[i+2] = toByte(c.Z)
		r.RGBA[i+3] = 255
	}
}

// reusable returns the previous sample that reprojected onto pixel (x, y), or
// failing that onto a neighbour (forward motion magnifies the image and leaves
// gaps between reprojected samples), or -1.
func (r *Renderer) reusable(x, y int) int {
	best, bestDist := -1, math.Inf(1)
	try := func(x, y int) {
		if x < 0 || y < 0 || x >= r.W || y >= r.H {
			return
		}
		j := y*r.W + x
		i := r.src[j]
		if i < 0 || int(r.prev.Samples[i].reused) >= r.Reuse-1 {
			return
		}
		if best < 0 || r.srcDist[j] < bestDist {
			best, bestDist = int(i), r.srcDist[j]
		}
	}
	try(x, y)
	if best >= 0 {
		return best
	}
	try(x-1, y)
	try(x+1, y)
	try(x, y-1)
	try(x, y+1)
	return best
}

// pattern assigns each pixel the frame, out of every Reuse, it is shaded in.
func (r *Renderer) pattern(x, y int) int {
	switch r.Reuse {
	case 2:
		return (x + y) & 1
	case 4:
		return (x & 1) | (y&1)<<1
	}
	return (x + y*3) % r.Reuse
}

func toByte(v float64) byte {
	if !(v > 0) { // also catches NaN
		return 0
	}
	if v >= 1 {
		return 255
	}
	return byte(v*255 + 0.5)
}

// Reused reports the fraction of the last frame's pixels that the
// checkerboard carried over from the frame before instead of shading.
func (r *Renderer) Reused() float64 {
	n := 0
	for _, s := range r.prev.Samples {
		if s.reused > 0 {
			n++
		}
	}
	return float64(n) / float64(len(r.prev.Samples))
}
