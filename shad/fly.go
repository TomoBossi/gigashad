package shad

import (
	"math"

	"github.com/hajimehoshi/ebiten/v2"
)

// Flyer is a ready-made camera controller: a Player steered by keys and
// mouse, colliding with the shader's Field if it has one. A shader calls its
// Update from Def.Update; keys that Flyer doesn't cover belong in
// Def.Bindings.
type Flyer struct {
	Player Player
	Keys   FlyKeys
}

// FlyKeys maps the controls. Each control is a list of keys, any of which
// works; an empty list disables it. (Lists rather than single keys also
// because ebiten.KeyA is 0, so a zero Key can't mean "none".)
type FlyKeys struct {
	Forward, Back, Left, Right, Up, Down []Key
	Fast                                 []Key // hold to move 4x faster
	Grow, Shrink                         []Key // change the player's size
	YawLeft, YawRight                    []Key // keyboard turning, on top of the mouse
	PitchUp, PitchDown                   []Key
	RollLeft, RollRight                  []Key

	// Gamepad, in the standard layout. MoveStick moves forward and back and
	// glides sideways; LookStick turns, faster the further it's tilted.
	// Triggers given as PadShrink and PadGrow work by how far they're pulled.
	MoveStick, LookStick      bool
	PadUp, PadDown            []Button
	PadFast                   []Button // hold to move 4x faster
	PadRollLeft, PadRollRight []Button
	PadShrink, PadGrow        []Button

	MouseSens   float64 // radians per pixel of mouse movement; 0 disables the mouse
	TurnRate    float64 // radians per second for keyboard yaw and pitch
	PadTurnRate float64 // radians per second with LookStick fully tilted
	RollRate    float64 // radians per second
	GrowRate    float64 // e-folds per second
}

// Keys is shorthand for a FlyKeys control.
func Keys(k ...Key) []Key { return k }

// Buttons is shorthand for a FlyKeys gamepad control.
func Buttons(b ...Button) []Button { return b }

// DefaultFlyKeys: WASD, Space/Ctrl up/down, Shift fast, mouse and arrows to
// look, Q/E roll, Z/C shrink/grow. On a gamepad: left stick to move, right
// stick to look, A/B up/down, X fast, LB/RB roll, LT/RT shrink/grow.
func DefaultFlyKeys() FlyKeys {
	return FlyKeys{
		Forward: Keys(ebiten.KeyW), Back: Keys(ebiten.KeyS),
		Left: Keys(ebiten.KeyA), Right: Keys(ebiten.KeyD),
		Up: Keys(ebiten.KeySpace), Down: Keys(ebiten.KeyControl),
		Fast: Keys(ebiten.KeyShift),
		Grow: Keys(ebiten.KeyC), Shrink: Keys(ebiten.KeyZ),
		YawLeft: Keys(ebiten.KeyArrowLeft), YawRight: Keys(ebiten.KeyArrowRight),
		PitchUp: Keys(ebiten.KeyArrowUp), PitchDown: Keys(ebiten.KeyArrowDown),
		RollLeft: Keys(ebiten.KeyQ), RollRight: Keys(ebiten.KeyE),
		MoveStick: true, LookStick: true,
		PadUp:       Buttons(ebiten.StandardGamepadButtonRightBottom),
		PadDown:     Buttons(ebiten.StandardGamepadButtonRightRight),
		PadFast:     Buttons(ebiten.StandardGamepadButtonRightLeft),
		PadRollLeft: Buttons(ebiten.StandardGamepadButtonFrontTopLeft), PadRollRight: Buttons(ebiten.StandardGamepadButtonFrontTopRight),
		PadShrink: Buttons(ebiten.StandardGamepadButtonFrontBottomLeft), PadGrow: Buttons(ebiten.StandardGamepadButtonFrontBottomRight),
		MouseSens: 0.003, TurnRate: 1.5, PadTurnRate: 2.5, RollRate: 1.0, GrowRate: 1.0,
	}
}

// Update reads the controls, steps the player and points the frame's camera
// through its eyes.
func (fl *Flyer) Update(c *Context) {
	in, k := c.Input, &fl.Keys
	held := func(keys []Key) bool {
		for _, key := range keys {
			if in.Pressed(key) {
				return true
			}
		}
		return false
	}
	axis := func(neg, pos []Key) float64 {
		v := 0.0
		if held(pos) {
			v++
		}
		if held(neg) {
			v--
		}
		return v
	}
	// Gamepad buttons, by how far they're pressed.
	pressed := func(buttons []Button) float64 {
		v := 0.0
		for _, b := range buttons {
			v = max(v, in.ButtonValue(b))
		}
		return v
	}
	padAxis := func(neg, pos []Button) float64 { return pressed(pos) - pressed(neg) }
	var move, look Vec2
	if k.MoveStick {
		move = in.Stick(ebiten.StandardGamepadAxisLeftStickHorizontal, ebiten.StandardGamepadAxisLeftStickVertical)
	}
	if k.LookStick {
		// Squared, so small tilts aim finely and full tilt turns fast.
		look = in.Stick(ebiten.StandardGamepadAxisRightStickHorizontal, ebiten.StandardGamepadAxisRightStickVertical)
		look = look.Mul(look.Len() * k.PadTurnRate * in.DT)
	}
	unit := func(v float64) float64 { return Clamp(v, -1, 1) }

	var it Intent
	it.Move = V3(
		unit(axis(k.Left, k.Right)+move.X),
		unit(axis(k.Down, k.Up)+padAxis(k.PadDown, k.PadUp)),
		unit(axis(k.Back, k.Forward)-move.Y))
	it.Jump = held(k.Up) || pressed(k.PadUp) > 0.5
	it.Fast = held(k.Fast) || pressed(k.PadFast) > 0.5
	it.Grow = unit(axis(k.Shrink, k.Grow)+padAxis(k.PadShrink, k.PadGrow)) * k.GrowRate
	it.Yaw = axis(k.YawLeft, k.YawRight)*k.TurnRate*in.DT + in.MouseX*k.MouseSens + look.X
	it.Pitch = axis(k.PitchUp, k.PitchDown)*k.TurnRate*in.DT + in.MouseY*k.MouseSens + look.Y
	it.Roll = unit(axis(k.RollLeft, k.RollRight)+padAxis(k.PadRollLeft, k.PadRollRight)) * k.RollRate * in.DT
	fl.Player.Step(it, c.Field(), in.DT)
	c.Frame.Cam = fl.Player.Camera()
}

// Data reports the player's state, for Def.Data.
func (fl *Flyer) Data(c *Context) string {
	p := &fl.Player
	mode := "fly"
	if p.Walk {
		mode = "walk"
	}
	if !p.Collide {
		mode += ", no collisions"
	}
	return "Player\n  " + mode + "\n"
}

// ZoomBindings change the field of view while their keys are held: = (or
// keypad +) narrows it and - (or keypad -) widens it, moving the focal
// length (1 / tan(FovY/2)) by rate per second.
func (fl *Flyer) ZoomBindings(rate float64) []Binding {
	zoom := func(key Key, dir float64, help string) Binding {
		return Binding{
			Keys: []Key{key}, Hold: true, Help: help,
			Action: func(c *Context) {
				p := &fl.Player
				focal := 1 / math.Tan(p.FovY/2)
				focal = Clamp(focal+dir*rate*c.Input.DT, 0.2, 10)
				p.FovY = 2 * math.Atan(1/focal)
			},
		}
	}
	return []Binding{
		zoom(ebiten.KeyEqual, 1, "narrow the view"), zoom(ebiten.KeyNumpadAdd, 1, "narrow the view"),
		zoom(ebiten.KeyMinus, -1, "widen the view"), zoom(ebiten.KeyNumpadSubtract, -1, "widen the view"),
	}
}
