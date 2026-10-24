package shad

import (
	"strconv"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
)

// ControlsHelp lists a shader's controls, one "keys: what they do" line
// each: Def.Controls first, then the Bindings. Bindings with the same Help
// in a row share a line.
func ControlsHelp(d *Def) []string {
	lines := append([]string(nil), d.Controls...)
	for i := 0; i < len(d.Bindings); {
		b := d.Bindings[i]
		var alts []string
		for ; i < len(d.Bindings) && d.Bindings[i].Help == b.Help; i++ {
			names := make([]string, len(d.Bindings[i].Keys))
			for j, k := range d.Bindings[i].Keys {
				names[j] = KeyName(k)
			}
			if len(names) > 0 {
				alts = append(alts, strings.Join(names, "+"))
			}
			for _, btn := range d.Bindings[i].Buttons {
				alts = append(alts, ButtonName(btn))
			}
		}
		if b.Help != "" {
			lines = append(lines, strings.Join(alts, " or ")+": "+b.Help)
		}
	}
	return lines
}

// Help lists the controls these keys map, for Def.Controls.
func (k *FlyKeys) Help() []string {
	var lines []string
	add := func(what string, controls ...[]Key) {
		names := make([]string, len(controls))
		for i, keys := range controls {
			if len(keys) == 0 {
				return
			}
			alts := make([]string, len(keys))
			for j, key := range keys {
				alts[j] = KeyName(key)
			}
			names[i] = strings.Join(alts, " or ")
		}
		lines = append(lines, strings.Join(names, "/")+": "+what)
	}
	add("move", k.Forward, k.Left, k.Back, k.Right)
	add("up/down", k.Up, k.Down)
	add("move faster (hold)", k.Fast)
	if k.MouseSens > 0 {
		lines = append(lines, "mouse: look around")
	}
	add("turn", k.YawLeft, k.YawRight, k.PitchUp, k.PitchDown)
	add("roll", k.RollLeft, k.RollRight)
	add("shrink/grow", k.Shrink, k.Grow)

	pad := func(what string, controls ...[]Button) {
		names := make([]string, len(controls))
		for i, buttons := range controls {
			if len(buttons) == 0 {
				return
			}
			alts := make([]string, len(buttons))
			for j, b := range buttons {
				alts[j] = ButtonName(b)
			}
			names[i] = strings.Join(alts, " or ")
		}
		lines = append(lines, strings.Join(names, "/")+": "+what)
	}
	if k.MoveStick {
		lines = append(lines, "left stick: move forward/back, glide left/right")
	}
	if k.LookStick {
		lines = append(lines, "right stick: look around, faster the further it's tilted")
	}
	pad("up/down", k.PadUp, k.PadDown)
	pad("move faster (hold)", k.PadFast)
	pad("roll", k.PadRollLeft, k.PadRollRight)
	pad("shrink/grow", k.PadShrink, k.PadGrow)
	return lines
}

// buttonNames are the standard layout's buttons as labeled on an Xbox pad.
var buttonNames = map[Button]string{
	ebiten.StandardGamepadButtonRightBottom: "A", ebiten.StandardGamepadButtonRightRight: "B",
	ebiten.StandardGamepadButtonRightLeft: "X", ebiten.StandardGamepadButtonRightTop: "Y",
	ebiten.StandardGamepadButtonFrontTopLeft: "LB", ebiten.StandardGamepadButtonFrontTopRight: "RB",
	ebiten.StandardGamepadButtonFrontBottomLeft: "LT", ebiten.StandardGamepadButtonFrontBottomRight: "RT",
	ebiten.StandardGamepadButtonCenterLeft: "Back", ebiten.StandardGamepadButtonCenterRight: "Start",
	ebiten.StandardGamepadButtonLeftStick: "left stick click", ebiten.StandardGamepadButtonRightStick: "right stick click",
	ebiten.StandardGamepadButtonLeftTop: "D-pad up", ebiten.StandardGamepadButtonLeftBottom: "D-pad down",
	ebiten.StandardGamepadButtonLeftLeft: "D-pad left", ebiten.StandardGamepadButtonLeftRight: "D-pad right",
	ebiten.StandardGamepadButtonCenterCenter: "Guide",
}

// ButtonName is a gamepad button's label on an Xbox pad.
func ButtonName(b Button) string {
	if n, ok := buttonNames[b]; ok {
		return n
	}
	return "button " + strconv.Itoa(int(b))
}

// keyNames spells out keys whose ebiten names aren't what's on the keyboard.
var keyNames = map[Key]string{
	ebiten.KeyEscape: "Esc", ebiten.KeyControl: "Ctrl", ebiten.KeyMeta: "Cmd", ebiten.KeyAlt: "Option",
	ebiten.KeyArrowLeft: "Left", ebiten.KeyArrowRight: "Right", ebiten.KeyArrowUp: "Up", ebiten.KeyArrowDown: "Down",
	ebiten.KeyEqual: "=", ebiten.KeyMinus: "-", ebiten.KeyNumpadAdd: "keypad +", ebiten.KeyNumpadSubtract: "keypad -",
}

// KeyName is a key's name as printed on the keyboard.
func KeyName(k Key) string {
	if n, ok := keyNames[k]; ok {
		return n
	}
	return k.String()
}
