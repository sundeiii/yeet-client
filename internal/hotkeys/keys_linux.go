package hotkeys

import "github.com/sundeiii/yeet-client/internal/hotkey"

// On Linux, the hotkey library has Key1 mapped to 0x0030
// However, it seems like 0x0030 should actually be Key0...?

const (
	Key0 = hotkey.Key(0x0030)
	Key1 = hotkey.Key(0x0031)
	Key2 = hotkey.Key(0x0032)
	Key3 = hotkey.Key(0x0033)
	Key4 = hotkey.Key(0x0034)
	Key5 = hotkey.Key(0x0035)
	Key6 = hotkey.Key(0x0036)
	Key7 = hotkey.Key(0x0037)
	Key8 = hotkey.Key(0x0038)
	Key9 = hotkey.Key(0x0039)
)

const SuperKeyIdentifier = "Super"
