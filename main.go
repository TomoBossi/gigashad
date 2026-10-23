package main

import (
	"flag"
	"fmt"
	"os"
	"slices"
	"strconv"
	"strings"

	"gigashad/shad"
	"gigashad/shaders/oolacile"
	"gigashad/shaders/plasma"
)

// Each shader is a shad.Shader (CPU), a shad.GPUShader, or both.
var shaders = map[string]func() any{
	"oolacile": oolacile.New,
	"plasma":   plasma.New,
}

func main() {
	name := flag.String("shader", "oolacile", "shader to run: "+strings.Join(names(), ", "))
	backend := flag.String("backend", "auto", "where to render: auto (the shader decides), cpu or gpu")
	width := flag.Int("width", 240, "CPU render width in pixels; the window stretches it")
	gpuWidth := flag.Int("gpuwidth", 960, "GPU render width in pixels")
	ar := flag.String("ar", "16:9", "aspect ratio as w:h")
	fps := flag.Int("fps", 24, "frame rate cap")
	threads := flag.Int("threads", 3, "CPU cores to render with; 0 uses all of them")
	reuse := flag.Int("reuse", 4, "on scenes that don't change over time, shade only 1 pixel in this many per frame (1, 2 or 4) and reuse the previous frame for the rest")
	fullscreen := flag.Bool("fullscreen", false, "start fullscreen")
	shot := flag.String("shot", "", "render one frame to this PNG and exit (GPU shots briefly open a window)")
	shotTime := flag.Float64("time", 0, "time in seconds for -shot")
	bench := flag.Int("bench", 0, "render this many frames from the spawn point as fast as possible, print the time per frame, and exit")
	flag.Parse()

	mk, ok := shaders[*name]
	if !ok {
		fail("unknown shader %q; have: %s", *name, strings.Join(names(), ", "))
	}
	ratio, err := parseRatio(*ar)
	if err != nil {
		fail("bad -ar: %v", err)
	}
	if *width <= 0 || *gpuWidth <= 0 {
		fail("-width and -gpuwidth must be positive")
	}
	h := max(1, int(float64(*width)/ratio+0.5))
	gpuH := max(1, int(float64(*gpuWidth)/ratio+0.5))
	be, ok := map[string]shad.Backend{"auto": shad.Auto, "cpu": shad.CPU, "gpu": shad.GPU}[*backend]
	if !ok {
		fail("-backend must be auto, cpu or gpu")
	}
	s := mk()
	if *shot != "" && !shad.StartsOnGPU(s, be) {
		cpuShader, ok := s.(shad.Shader)
		if !ok {
			fail("this shader has no CPU version")
		}
		took, err := shad.Shot(cpuShader, *width, h, *shotTime, *shot)
		if err != nil {
			fail("%v", err)
		}
		fmt.Printf("rendered %dx%d in %v -> %s\n", *width, h, took, *shot)
		return
	}

	app, err := shad.NewApp(s, shad.Options{
		Width: *width, Height: h, GPUWidth: *gpuWidth, GPUHeight: gpuH, Backend: be,
		FPS: *fps, Threads: *threads, Reuse: *reuse, Fullscreen: *fullscreen,
		Title: "gigashad: " + *name, ShotDir: "shots", ShotPath: *shot, ShotTime: *shotTime, Bench: *bench,
	})
	if err != nil {
		fail("%v", err)
	}
	if err := app.Run(); err != nil {
		fail("%v", err)
	}
}

func names() []string {
	var n []string
	for k := range shaders {
		n = append(n, k)
	}
	slices.Sort(n)
	return n
}

func parseRatio(s string) (float64, error) {
	a, b, ok := strings.Cut(s, ":")
	if !ok {
		return 0, fmt.Errorf("expected w:h")
	}
	w, err1 := strconv.ParseFloat(a, 64)
	h, err2 := strconv.ParseFloat(b, 64)
	if err1 != nil || err2 != nil || w <= 0 || h <= 0 {
		return 0, fmt.Errorf("expected two positive numbers")
	}
	return w / h, nil
}

func fail(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
