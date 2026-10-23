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

var shaders = map[string]func() shad.Shader{
	"oolacile": oolacile.New,
	"plasma":   plasma.New,
}

func main() {
	name := flag.String("shader", "oolacile", "shader to run: "+strings.Join(names(), ", "))
	width := flag.Int("width", 240, "render width in pixels; the window stretches it")
	ar := flag.String("ar", "16:9", "aspect ratio as w:h")
	fps := flag.Int("fps", 24, "frame rate cap")
	threads := flag.Int("threads", 3, "CPU cores to render with; 0 uses all of them")
	reuse := flag.Int("reuse", 4, "on scenes that don't change over time, shade only 1 pixel in this many per frame (1, 2 or 4) and reuse the previous frame for the rest")
	fullscreen := flag.Bool("fullscreen", false, "start fullscreen")
	shot := flag.String("shot", "", "render one frame to this PNG and exit, without a window")
	shotTime := flag.Float64("time", 0, "time in seconds for -shot")
	flag.Parse()

	mk, ok := shaders[*name]
	if !ok {
		fail("unknown shader %q; have: %s", *name, strings.Join(names(), ", "))
	}
	ratio, err := parseRatio(*ar)
	if err != nil {
		fail("bad -ar: %v", err)
	}
	if *width <= 0 {
		fail("-width must be positive")
	}
	h := max(1, int(float64(*width)/ratio+0.5))
	s := mk()

	if *shot != "" {
		took, err := shad.Shot(s, *width, h, *shotTime, *shot)
		if err != nil {
			fail("%v", err)
		}
		fmt.Printf("rendered %dx%d in %v -> %s\n", *width, h, took, *shot)
		return
	}

	app := shad.NewApp(s, shad.Options{
		Width: *width, Height: h, FPS: *fps, Threads: *threads, Reuse: *reuse, Fullscreen: *fullscreen,
		Title: "gigashad: " + *name, ShotDir: "shots",
	})
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
