package shad

import (
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
)

// Input is one frame of keyboard and mouse state.
type Input struct {
	DT     float64 // seconds since the previous frame
	MouseX float64 // cursor movement since the previous frame, in pixels
	MouseY float64
	Wheel  float64      // scroll since the previous frame, positive up
	live   bool         // read the keyboard; otherwise only scripted keys are down
	keys   map[Key]bool // scripted keys, see ScriptedInput
}

// ScriptedInput is a frame of input with the given keys held (and counting as
// just pressed), for tests and scripted runs.
func ScriptedInput(dt float64, keys ...Key) *Input {
	in := &Input{DT: dt, keys: map[Key]bool{}}
	for _, k := range keys {
		in.keys[k] = true
	}
	return in
}

func (in *Input) Pressed(k Key) bool {
	if in.live {
		return ebiten.IsKeyPressed(k)
	}
	return in.keys[k]
}

func (in *Input) JustPressed(k Key) bool {
	if in.live {
		return inpututil.IsKeyJustPressed(k)
	}
	return in.keys[k]
}

// Axis is 1 while pos is held, -1 while neg is, 0 for both or neither.
func (in *Input) Axis(neg, pos Key) float64 {
	v := 0.0
	if in.Pressed(pos) {
		v++
	}
	if in.Pressed(neg) {
		v--
	}
	return v
}

// fired reports whether b's action runs this frame.
func (b *Binding) fired(in *Input) bool {
	if len(b.Keys) == 0 {
		return false
	}
	for _, k := range b.Keys[:len(b.Keys)-1] {
		if !in.Pressed(k) {
			return false
		}
	}
	last := b.Keys[len(b.Keys)-1]
	if b.Hold {
		return in.Pressed(last)
	}
	return in.JustPressed(last)
}
