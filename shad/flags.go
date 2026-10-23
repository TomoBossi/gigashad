package shad

import (
	"flag"
	"time"
)

// Flags lets a shader declare its own command-line flags. Names are
// namespaced by the shader, so -breath declared by "oolacile" is
// -oolacile.breath; they are parsed with the engine's own flags and listed in
// -h. Declare them when registering the shader and read them when building
// its Def.
type Flags struct {
	set    *flag.FlagSet
	prefix string
}

// NewFlags declares shader's flags on set.
func NewFlags(set *flag.FlagSet, shader string) Flags {
	return Flags{set: set, prefix: shader + "."}
}

func (f Flags) Float64(name string, value float64, usage string) *float64 {
	return f.set.Float64(f.prefix+name, value, usage)
}

func (f Flags) Int(name string, value int, usage string) *int {
	return f.set.Int(f.prefix+name, value, usage)
}

func (f Flags) Bool(name string, value bool, usage string) *bool {
	return f.set.Bool(f.prefix+name, value, usage)
}

func (f Flags) String(name string, value string, usage string) *string {
	return f.set.String(f.prefix+name, value, usage)
}

func (f Flags) Duration(name string, value time.Duration, usage string) *time.Duration {
	return f.set.Duration(f.prefix+name, value, usage)
}
