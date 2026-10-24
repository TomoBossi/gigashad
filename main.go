package main

import (
	"flag"
	"fmt"
	"os"
	"slices"
	"strconv"
	"strings"

	"gigashad/shad"
	"gigashad/shaders/explorer"
	"gigashad/shaders/gopher"
	"gigashad/shaders/oolacile"
	"gigashad/shaders/plasma"
)

// shaders maps names to registration functions, which declare the shader's
// own flags (namespaced by its name) and return its constructor.
var shaders = map[string]func(shad.Flags) func() shad.Def{
	"explorer": explorer.Register,
	"gopher":   gopher.Register,
	"oolacile": oolacile.Register,
	"plasma":   plasma.Register,
}

func main() {
	name := flag.String("shader", "", "shader to run (required): "+strings.Join(names(), ", "))
	backend := flag.String("backend", "auto", "where to render: auto (the shader decides), cpu or gpu")
	width := flag.Int("width", 0, "fixed render width in pixels; 0 adjusts it every frame to hold -fps")
	ar := flag.String("ar", "auto", "aspect ratio as w:h, or auto to follow the window (and the monitor when fullscreen)")
	fps := flag.Int("fps", 0, "target and cap frame rate; 0 uses the shader's own, or 30")
	threads := flag.Int("threads", 3, "CPU cores to render with; 0 uses all of them")
	reuse := flag.Int("reuse", 4, "on scenes that don't change over time, shade only 1 pixel in this many per frame (1, 2 or 4) and reuse the previous frame for the rest")
	fullscreen := flag.Bool("fullscreen", false, "start fullscreen")
	shot := flag.String("shot", "", "render one frame to this PNG and exit (GPU shots briefly open a window)")
	shotTime := flag.Float64("time", 0, "time in seconds for -shot")
	bench := flag.Int("bench", 0, "render this many frames from the spawn point as fast as possible, print the time per frame, and exit")
	constructors := map[string]func() shad.Def{}
	for n, register := range shaders {
		constructors[n] = register(shad.NewFlags(flag.CommandLine, n))
	}
	flag.Parse()

	if *name == "" {
		fail("pick a shader with -shader: %s", strings.Join(names(), ", "))
	}
	mk, ok := constructors[*name]
	if !ok {
		fail("unknown shader %q; have: %s", *name, strings.Join(names(), ", "))
	}
	ratio := 0.0 // follow the window
	if *ar != "auto" {
		r, err := parseRatio(*ar)
		if err != nil {
			fail("bad -ar: %v", err)
		}
		ratio = r
	}
	if *width < 0 || *fps < 0 {
		fail("-width and -fps must be 0 or positive")
	}
	be, ok := map[string]shad.Backend{"auto": shad.Auto, "cpu": shad.OnCPU, "gpu": shad.OnGPU}[*backend]
	if !ok {
		fail("-backend must be auto, cpu or gpu")
	}
	if *shot != "" && !shad.StartsOnGPU(new(mk()), be) {
		w := *width
		if w == 0 {
			w = shad.DefaultCPUWidth
		}
		if ratio == 0 {
			ratio = 16.0 / 10 // no window to follow
		}
		h := max(1, int(float64(w)/ratio+0.5))
		took, err := shad.Shot(new(mk()), w, h, *shotTime, *shot)
		if err != nil {
			fail("%v", err)
		}
		fmt.Printf("rendered %dx%d in %v -> %s\n", w, h, took, *shot)
		return
	}

	def := new(mk())
	if *fps == 0 {
		*fps = def.FPS
	}
	if *fps == 0 {
		*fps = 30
	}
	app, err := shad.NewApp(def, shad.Options{
		Width: *width, Aspect: ratio, Backend: be,
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
