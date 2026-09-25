package hotkeys

import (
	"strings"

	"github.com/sundeiii/yeet-client/internal/hotkey"
)

var modifierMapping = map[string]hotkey.Modifier{
	"ctrl":  ModCtrl,
	"shift": ModShift,
	"alt":   ModAlt,
	"cmd":   ModCmd,
	"win":   ModCmd,
	"super": ModCmd,
}

var keyMapping = map[string]hotkey.Key{
	"0":      Key0,
	"1":      Key1,
	"2":      Key2,
	"3":      Key3,
	"4":      Key4,
	"5":      Key5,
	"6":      Key6,
	"7":      Key7,
	"8":      Key8,
	"9":      Key9,
	"A":      hotkey.KeyA,
	"B":      hotkey.KeyB,
	"C":      hotkey.KeyC,
	"D":      hotkey.KeyD,
	"E":      hotkey.KeyE,
	"F":      hotkey.KeyF,
	"G":      hotkey.KeyG,
	"H":      hotkey.KeyH,
	"I":      hotkey.KeyI,
	"J":      hotkey.KeyJ,
	"K":      hotkey.KeyK,
	"L":      hotkey.KeyL,
	"M":      hotkey.KeyM,
	"N":      hotkey.KeyN,
	"O":      hotkey.KeyO,
	"P":      hotkey.KeyP,
	"Q":      hotkey.KeyQ,
	"R":      hotkey.KeyR,
	"S":      hotkey.KeyS,
	"T":      hotkey.KeyT,
	"U":      hotkey.KeyU,
	"V":      hotkey.KeyV,
	"W":      hotkey.KeyW,
	"X":      hotkey.KeyX,
	"Y":      hotkey.KeyY,
	"Z":      hotkey.KeyZ,
	"SPACE":  hotkey.KeySpace,
	"ENTER":  hotkey.KeyReturn,
	"RETURN": hotkey.KeyReturn,
	"ESCAPE": hotkey.KeyEscape,
	"ESC":    hotkey.KeyEscape,
	"UP":     hotkey.KeyUp,
	"DOWN":   hotkey.KeyDown,
	"LEFT":   hotkey.KeyLeft,
	"RIGHT":  hotkey.KeyRight,
}

func parseModifiers(parts []string) []hotkey.Modifier {
	var mods []hotkey.Modifier
	for _, part := range parts[:len(parts)-1] {
		modString := strings.ToLower(strings.TrimSpace(part))
		if mod, ok := modifierMapping[modString]; ok {
			mods = append(mods, mod)
		}
	}
	return mods
}

func parseKey(keyString string) hotkey.Key {
	keyString = strings.ToUpper(strings.TrimSpace(keyString))
	if keyValue, ok := keyMapping[keyString]; ok {
		return keyValue
	}
	return 0
}
