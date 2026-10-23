package shad

import "github.com/hajimehoshi/ebiten/v2"

// Flyer is a ready-made camera controller: a Player steered by keys and
// mouse, colliding with the shader's Field if it has one. A shader calls its
// Update from Def.Update; keys that Flyer doesn't cover belong in
// Def.Bindings.
type Flyer struct {
	Player Player
	Keys   FlyKeys
}

// FlyKeys maps the controls. A zero key disables that control.
type FlyKeys struct {
	Forward, Back, Left, Right, Up, Down Key
	Fast                                 Key // hold to move 4x faster
	Grow, Shrink                         Key // change the player's size
	YawLeft, YawRight                    Key // keyboard turning, on top of the mouse
	PitchUp, PitchDown                   Key
	RollLeft, RollRight                  Key

	MouseSens float64 // radians per pixel of mouse movement; 0 disables the mouse
	TurnRate  float64 // radians per second for keyboard yaw and pitch
	RollRate  float64 // radians per second
	GrowRate  float64 // e-folds per second
}

// DefaultFlyKeys: WASD, Space/Ctrl up/down, Shift fast, mouse and arrows to
// look, Q/E roll, Z/C shrink/grow.
func DefaultFlyKeys() FlyKeys {
	return FlyKeys{
		Forward: ebiten.KeyW, Back: ebiten.KeyS, Left: ebiten.KeyA, Right: ebiten.KeyD,
		Up: ebiten.KeySpace, Down: ebiten.KeyControl, Fast: ebiten.KeyShift,
		Grow: ebiten.KeyC, Shrink: ebiten.KeyZ,
		YawLeft: ebiten.KeyArrowLeft, YawRight: ebiten.KeyArrowRight,
		PitchUp: ebiten.KeyArrowUp, PitchDown: ebiten.KeyArrowDown,
		RollLeft: ebiten.KeyQ, RollRight: ebiten.KeyE,
		MouseSens: 0.003, TurnRate: 1.5, RollRate: 1.0, GrowRate: 1.0,
	}
}

// Update reads the controls, steps the player and points the frame's camera
// through its eyes.
func (fl *Flyer) Update(c *Context) {
	in, k := c.Input, &fl.Keys
	axis := func(neg, pos Key) float64 {
		v := 0.0
		if pos != 0 && in.Pressed(pos) {
			v++
		}
		if neg != 0 && in.Pressed(neg) {
			v--
		}
		return v
	}
	var it Intent
	it.Move = V3(axis(k.Left, k.Right), axis(k.Down, k.Up), axis(k.Back, k.Forward))
	it.Jump = k.Up != 0 && in.Pressed(k.Up)
	it.Fast = k.Fast != 0 && in.Pressed(k.Fast)
	it.Grow = axis(k.Shrink, k.Grow) * k.GrowRate
	it.Yaw = axis(k.YawLeft, k.YawRight)*k.TurnRate*in.DT + in.MouseX*k.MouseSens
	it.Pitch = axis(k.PitchUp, k.PitchDown)*k.TurnRate*in.DT + in.MouseY*k.MouseSens
	it.Roll = axis(k.RollLeft, k.RollRight) * k.RollRate * in.DT
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
