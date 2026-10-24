package shad

import (
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
			alts = append(alts, strings.Join(names, "+"))
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
	return lines
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
