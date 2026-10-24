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

// shaders maps names to registration functions, which declare the shader's
// own flags (namespaced by its name) and return its constructor.
var shaders = map[string]func(shad.Flags) func() shad.Def{
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
	shotDir := flag.String("shotdir", "shots", "directory where screenshots (P) and scene data (Tab) are saved, created if it doesn't exist; relative paths start at the current directory")
	showFPS := flag.Bool("showfps", false, "show a frame rate counter in the bottom-right corner")
	bench := flag.Int("bench", 0, "render this many frames from the spawn point as fast as possible, print the time per frame, and exit")
	// Only the chosen shader's flags are declared, so -h lists them only
	// when a shader is given: gigashad -shader oolacile -h.
	engine := flag.NewFlagSet("", flag.ContinueOnError)
	flag.VisitAll(func(f *flag.Flag) { engine.Var(f.Value, f.Name, f.Usage) })
	chosen := shaderArg(os.Args[1:])
	var own *flag.FlagSet
	var mk func() shad.Def
	if register, ok := shaders[chosen]; ok {
		own = flag.NewFlagSet("", flag.ContinueOnError)
		mk = register(shad.NewFlags(own, chosen))
		own.VisitAll(func(f *flag.Flag) { flag.Var(f.Value, f.Name, f.Usage) })
	}
	flag.Usage = func() {
		out := flag.CommandLine.Output()
		fmt.Fprintf(out, "Usage: gigashad -shader <name> [flags]\n\nFlags:\n")
		engine.SetOutput(out)
		engine.PrintDefaults()
		if own == nil {
			fmt.Fprintf(out, "\nFor a shader's own controls and flags: gigashad -shader <name> -h\n")
			return
		}
		fmt.Fprintf(out, "\n%s flags:\n", chosen)
		own.SetOutput(out)
		own.PrintDefaults()
		none := true
		own.VisitAll(func(*flag.Flag) { none = false })
		if none {
			fmt.Fprintf(out, "  (none)\n")
		}
		fmt.Fprintf(out, "\n%s controls:\n", chosen)
		for _, line := range shad.ControlsHelp(new(mk())) {
			fmt.Fprintf(out, "  - %s\n", line)
		}
	}
	flag.Parse()

	if *name == "" {
		fail("pick a shader with -shader: %s", strings.Join(names(), ", "))
	}
	if mk == nil {
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
	if *shotDir == "" {
		fail("-shotdir can't be empty")
	}
	if info, err := os.Stat(*shotDir); err == nil && !info.IsDir() {
		fail("-shotdir %s is a file, not a directory", *shotDir)
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
		Title: "gigashad: " + *name, ShotDir: *shotDir, ShotPath: *shot, ShotTime: *shotTime, Bench: *bench,
		ShowFPS: *showFPS,
	})
	if err != nil {
		fail("%v", err)
	}
	if err := app.Run(); err != nil {
		fail("%v", err)
	}
}

// shaderArg finds the value of -shader in args before they're parsed, so its
// flags can be declared first.
func shaderArg(args []string) string {
	for i, a := range args {
		if a == "--" {
			break
		}
		a = strings.TrimPrefix(strings.TrimPrefix(a, "-"), "-")
		if v, ok := strings.CutPrefix(a, "shader="); ok {
			return v
		}
		if a == "shader" && i+1 < len(args) {
			return args[i+1]
		}
	}
	return ""
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
