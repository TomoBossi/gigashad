package main

import (
	"flag"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
)

type flags struct {
	frag     string
	width    int
	ar       float64
	windowed bool
}

func NewFlags() (*flags, error) {
	frag := flag.String("frag", "", "Path to the fragment shader source file. This argument is REQUIRED.")
	width := flag.Int("width", 320, "Render width in pixels (default 320)")
	ar := flag.String("ar", "16:9", "Render aspect ratio in width:height format (default \"16:9\")")
	windowed := flag.Bool("windowed", false, "If provided, the render will be displayed in windowed mode using the render width and height as the window size")

	flag.Parse()

	if *frag == "" {
		return nil, fmt.Errorf("error: Fragment shader source file not provided")
	}

	if fragExists, err := exists(*frag); *frag != "" && !fragExists {
		return nil, fmt.Errorf("error: Fragment shader source file not found:\n\t%s", err.Error())
	}

	if *frag != "" && filepath.Ext(*frag) != ".frag" {
		return nil, fmt.Errorf("error: Fragment shader source file must have a .frag extension")
	}

	if *width <= 0 {
		return nil, fmt.Errorf("error: Render width must be greater than 0")
	}

	parsedAspectRatio, err := parseAspectRatio(*ar)
	if err != nil {
		return nil, fmt.Errorf("error: Aspect Ratio could not be parsed:\n\t%s", err.Error())
	}

	return &flags{
		frag:     *frag,
		width:    *width,
		ar:       parsedAspectRatio,
		windowed: *windowed,
	}, nil
}

func parseAspectRatio(ar string) (float64, error) {
	operands := strings.Split(ar, ":")
	if len(operands) != 2 {
		return 0, fmt.Errorf("error: Invalid format, expected \"width:height\"")
	}
	width, err := strconv.ParseFloat(operands[0], 64)
	if err != nil {
		return 0, fmt.Errorf("error: invalid width value")
	}

	height, err := strconv.ParseFloat(operands[1], 64)
	if err != nil {
		return 0, fmt.Errorf("error: Invalid height value")
	}

	if height == 0 {
		return 0, fmt.Errorf("error: Height cannot be zero")
	}

	return width / height, nil
}

func (f flags) Frag() string {
	return f.frag
}

func (f flags) Width() int {
	return f.width
}

func (f flags) Ar() float64 {
	return f.ar
}

func (f flags) Windowed() bool {
	return f.windowed
}
