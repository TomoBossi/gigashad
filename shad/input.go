package shad

import (
	"math"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
)

// Button is a gamepad button in the standard layout, named by position
// (ebiten.StandardGamepadButtonRightBottom is A on an Xbox pad).
type Button = ebiten.StandardGamepadButton

// Stick is a gamepad stick axis in the standard layout.
type Stick = ebiten.StandardGamepadAxis

// Input is one frame of keyboard, mouse and gamepad state. Every connected
// gamepad with a standard layout counts, as if they were one.
type Input struct {
	DT      float64 // seconds since the previous frame
	MouseX  float64 // cursor movement since the previous frame, in pixels
	MouseY  float64
	Wheel   float64 // scroll since the previous frame, positive up
	live    bool    // read the devices; otherwise only scripted input counts
	pads    []ebiten.GamepadID
	keys    map[Key]bool       // scripted keys, see ScriptedInput
	buttons map[Button]float64 // scripted buttons and sticks
	sticks  map[Stick]float64
}

// ScriptedInput is a frame of input with the given keys held (and counting as
// just pressed), for tests and scripted runs.
func ScriptedInput(dt float64, keys ...Key) *Input {
	in := &Input{DT: dt, keys: map[Key]bool{}, buttons: map[Button]float64{}, sticks: map[Stick]float64{}}
	for _, k := range keys {
		in.keys[k] = true
	}
	return in
}

// WithButton holds b at v (0 to 1) in a scripted input.
func (in *Input) WithButton(b Button, v float64) *Input {
	in.buttons[b] = v
	return in
}

// WithStick tilts axis a to v (-1 to 1) in a scripted input.
func (in *Input) WithStick(a Stick, v float64) *Input {
	in.sticks[a] = v
	return in
}

// readPads finds the connected gamepads with a standard layout.
func (in *Input) readPads() {
	for _, id := range ebiten.AppendGamepadIDs(nil) {
		if ebiten.IsStandardGamepadLayoutAvailable(id) {
			in.pads = append(in.pads, id)
		}
	}
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

// ButtonValue is how far b is pressed, 0 to 1: analog for triggers, 0 or 1
// for the rest.
func (in *Input) ButtonValue(b Button) float64 {
	if !in.live {
		return in.buttons[b]
	}
	v := 0.0
	for _, id := range in.pads {
		v = max(v, ebiten.StandardGamepadButtonValue(id, b))
	}
	return v
}

func (in *Input) ButtonPressed(b Button) bool {
	if !in.live {
		return in.buttons[b] > 0.5
	}
	for _, id := range in.pads {
		if ebiten.IsStandardGamepadButtonPressed(id, b) {
			return true
		}
	}
	return false
}

func (in *Input) ButtonJustPressed(b Button) bool {
	if !in.live {
		return in.buttons[b] > 0.5
	}
	for _, id := range in.pads {
		if inpututil.IsStandardGamepadButtonJustPressed(id, b) {
			return true
		}
	}
	return false
}

// stickDeadZone is the tilt sticks ignore, as a fraction of full tilt:
// resting sticks rarely read exactly 0.
const stickDeadZone = 0.15

// Stick is a stick's position from its two axes, each -1 to 1, with x right
// and y down. A small tilt reads as 0, and the rest of the range is
// stretched to reach 1 again.
func (in *Input) Stick(x, y Stick) Vec2 {
	v := Vec2{in.stickAxis(x), in.stickAxis(y)}
	r := v.Len()
	if r < stickDeadZone {
		return Vec2{}
	}
	return v.Mul(min(1, (r-stickDeadZone)/(1-stickDeadZone)) / r)
}

// stickAxis is the axis tilted furthest across the gamepads.
func (in *Input) stickAxis(a Stick) float64 {
	if !in.live {
		return in.sticks[a]
	}
	v := 0.0
	for _, id := range in.pads {
		if w := ebiten.StandardGamepadAxisValue(id, a); math.Abs(w) > math.Abs(v) {
			v = w
		}
	}
	return v
}

// fired reports whether b's action runs this frame.
func (b *Binding) fired(in *Input) bool {
	for _, btn := range b.Buttons {
		if b.Hold && in.ButtonPressed(btn) || !b.Hold && in.ButtonJustPressed(btn) {
			return true
		}
	}
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
