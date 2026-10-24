package shad

import (
	"math"
	"testing"
)

// Project undoes Ray, with and without fisheye, across the whole image.
func TestProjectInvertsRay(t *testing.T) {
	res := V2(320, 200)
	for _, fe := range []float64{0, 0.5, 1} {
		p := DefaultPlayer()
		p.Fwd, p.Up = V3(0.3, 0.2, 0.9).Norm(), V3(0, 1, 0)
		p.FovY = 70 * math.Pi / 180
		p.Fisheye = fe
		c := p.Camera()
		worst := 0.0
		for y := 0.5; y < res.Y; y += 7 {
			for x := 0.5; x < res.X; x += 7 {
				frag, ok := c.Project(c.Ray(V2(x, y), res), res)
				if !ok {
					t.Fatalf("fisheye %v: pixel (%v, %v) projects behind the camera", fe, x, y)
				}
				worst = math.Max(worst, frag.Sub(V2(x, y)).Len())
			}
		}
		if worst > 1e-6 {
			t.Errorf("fisheye %v: Project(Ray(p)) is off by up to %.3g pixels", fe, worst)
		}
	}
}
