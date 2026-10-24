package shad

import (
	"fmt"

	"github.com/hajimehoshi/ebiten/v2"
)

// Post is an optional finishing pass over every rendered frame, CPU or GPU:
// bloom, then grading. It runs on the GPU as part of displaying the frame;
// the CPU's samples and G-buffer are untouched, screenshots include it.
type Post struct {
	Exposure   float64 // multiplies the color; 0 means 1
	Contrast   float64 // around mid grey; 0 means 1
	Saturation float64 // 0 means 1; 0.001 is nearly grey
	Shadows    Vec3    // tint added in proportion to darkness
	Vignette   float64 // how much the corners darken, 0..1

	// Bloom adds a soft, wide copy of the bright parts of the image.
	Bloom          float64 // strength; 0 disables it
	BloomThreshold float64 // brightness above which pixels bloom
	BloomRadius    float64 // as a fraction of the image height
}

const postBlurKage = `//kage:unit pixels

package main

var Dir vec2    // (1, 0) or (0, 1)
var Radius float // in pixels of this image

func Fragment(dstPos vec4, srcPos vec2, color vec4) vec4 {
	sum := vec3(0)
	total := 0.0
	sigma := max(Radius/2, 0.5)
	for i := -24; i <= 24; i++ {
		x := float(i) * Radius / 24
		w := exp(-x * x / (2 * sigma * sigma))
		sum += imageSrc0At(srcPos+Dir*x).rgb * w
		total += w
	}
	return vec4(sum/total, 1)
}
`

const postBrightKage = `//kage:unit pixels

package main

var Threshold float

func Fragment(dstPos vec4, srcPos vec2, color vec4) vec4 {
	c := imageSrc0At(srcPos).rgb
	l := max(c.r, max(c.g, c.b))
	k := smoothstep(Threshold, Threshold+0.25, l)
	return vec4(c*k, 1)
}
`

const postGradeKage = `//kage:unit pixels

package main

var Size vec2
var Exposure float
var Contrast float
var Saturation float
var Shadows vec3
var Vignette float
var Bloom float

func Fragment(dstPos vec4, srcPos vec2, color vec4) vec4 {
	c := imageSrc0At(srcPos).rgb + imageSrc1At(srcPos).rgb*Bloom
	c *= Exposure
	luma := dot(c, vec3(0.2126, 0.7152, 0.0722))
	c = mix(vec3(luma), c, Saturation)
	c = (c-0.5)*Contrast + 0.5
	c += Shadows * (1 - smoothstep(0, 0.35, luma))
	uv := (srcPos-imageSrc0Origin())/Size - 0.5
	c *= 1 - Vignette*smoothstep(0.2, 0.75, length(uv*vec2(Size.x/Size.y, 1))*0.9)
	return vec4(clamp(c, 0, 1), 1)
}
`

// postFX applies a Post to frames.
type postFX struct {
	p                   Post
	blur, bright, grade *ebiten.Shader
	out, bright2        *ebiten.Image // the finished frame and the bright pass, at render size
	small, small2       *ebiten.Image // bloom buffers, at a quarter size
	bloom               *ebiten.Image // the blurred bloom, back at render size
}

func newPostFX(p Post) (*postFX, error) {
	fx := &postFX{p: p}
	for _, s := range []struct {
		dst **ebiten.Shader
		src string
	}{{&fx.blur, postBlurKage}, {&fx.bright, postBrightKage}, {&fx.grade, postGradeKage}} {
		sh, err := ebiten.NewShader([]byte(s.src))
		if err != nil {
			return nil, fmt.Errorf("compiling post shader: %w", err)
		}
		*s.dst = sh
	}
	return fx, nil
}

func or1(v float64) float64 {
	if v == 0 {
		return 1
	}
	return v
}

// apply processes src into fx.out and returns it.
func (fx *postFX) apply(src *ebiten.Image) *ebiten.Image {
	w, h := src.Bounds().Dx(), src.Bounds().Dy()
	sw, sh := max(1, w/4), max(1, h/4)
	fx.out = sized(fx.out, w, h)
	fx.bloom = sized(fx.bloom, w, h)
	fx.small = sized(fx.small, sw, sh)
	fx.small2 = sized(fx.small2, sw, sh)

	fx.bloom.Clear()
	if fx.p.Bloom > 0 {
		// Bright parts, downsampled, blurred both ways, scaled back up.
		fx.bright2 = sized(fx.bright2, w, h)
		bright := fx.bright2
		bright.DrawRectShader(w, h, fx.bright, &ebiten.DrawRectShaderOptions{
			Images:   [4]*ebiten.Image{src},
			Uniforms: map[string]any{"Threshold": float32(fx.p.BloomThreshold)},
		})
		op := &ebiten.DrawImageOptions{Filter: ebiten.FilterLinear}
		op.GeoM.Scale(float64(sw)/float64(w), float64(sh)/float64(h))
		fx.small.Clear()
		fx.small.DrawImage(bright, op)
		r := float32(fx.p.BloomRadius * float64(sh))
		fx.small2.DrawRectShader(sw, sh, fx.blur, &ebiten.DrawRectShaderOptions{
			Images:   [4]*ebiten.Image{fx.small},
			Uniforms: map[string]any{"Dir": []float32{1, 0}, "Radius": r},
		})
		fx.small.DrawRectShader(sw, sh, fx.blur, &ebiten.DrawRectShaderOptions{
			Images:   [4]*ebiten.Image{fx.small2},
			Uniforms: map[string]any{"Dir": []float32{0, 1}, "Radius": r},
		})
		op = &ebiten.DrawImageOptions{Filter: ebiten.FilterLinear}
		op.GeoM.Scale(float64(w)/float64(sw), float64(h)/float64(sh))
		fx.bloom.DrawImage(fx.small, op)
	}
	p := &fx.p
	fx.out.DrawRectShader(w, h, fx.grade, &ebiten.DrawRectShaderOptions{
		Images: [4]*ebiten.Image{src, fx.bloom},
		Uniforms: map[string]any{
			"Size":       []float32{float32(w), float32(h)},
			"Exposure":   float32(or1(p.Exposure)),
			"Contrast":   float32(or1(p.Contrast)),
			"Saturation": float32(or1(p.Saturation)),
			"Shadows":    Vec3f(p.Shadows),
			"Vignette":   float32(p.Vignette),
			"Bloom":      float32(p.Bloom),
		},
	})
	return fx.out
}

// sized returns img if it is w×h, or a new w×h image.
func sized(img *ebiten.Image, w, h int) *ebiten.Image {
	if img != nil && img.Bounds().Dx() == w && img.Bounds().Dy() == h {
		return img
	}
	if img != nil {
		img.Deallocate()
	}
	return ebiten.NewImage(w, h)
}
