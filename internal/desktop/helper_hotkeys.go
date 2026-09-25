package desktop

import (
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/widget"
	"github.com/sundeiii/yeet-client/internal/hotkeys"
)

type HotkeyButton struct {
	widget.Button
	value       string
	isCapturing bool
	focused     bool
	modifiers   fyne.KeyModifier

	OnStart     func()
	OnChanged   func(string)
	OnCancelled func()
}

// Ensure HotkeyButton implements these interfaces
var _ fyne.Focusable = (*HotkeyButton)(nil)
var _ desktop.Keyable = (*HotkeyButton)(nil)

func NewHotkeyButton(initialValue string) *HotkeyButton {
	b := &HotkeyButton{value: initialValue}
	b.Text = initialValue
	b.ExtendBaseWidget(b)
	b.OnTapped = func() {
		if b.isCapturing {
			return
		}
		b.isCapturing = true
		b.Text = "Press some keys..."
		b.Refresh()

		if b.OnStart != nil {
			b.OnStart()
		}
	}
	return b
}

func (b *HotkeyButton) FocusGained() {
	b.focused = true
	b.Refresh()
}

func (b *HotkeyButton) FocusLost() {
	b.focused = false
	b.Refresh()

	if b.isCapturing {
		b.isCapturing = false
		b.Text = b.value
		b.Refresh()

		if b.OnCancelled != nil {
			b.OnCancelled()
		}
	}
}

func (b *HotkeyButton) TypedRune(r rune)          {}
func (b *HotkeyButton) TypedKey(e *fyne.KeyEvent) {}

func (b *HotkeyButton) KeyDown(e *fyne.KeyEvent) {
	if !b.isCapturing {
		return
	}
	isModifier := false

	switch e.Name {
	case desktop.KeyShiftLeft, desktop.KeyShiftRight:
		b.modifiers |= fyne.KeyModifierShift
		isModifier = true
	case desktop.KeyControlLeft, desktop.KeyControlRight:
		b.modifiers |= fyne.KeyModifierControl
		isModifier = true
	case desktop.KeyAltLeft, desktop.KeyAltRight:
		b.modifiers |= fyne.KeyModifierAlt
		isModifier = true
	case desktop.KeySuperLeft, desktop.KeySuperRight:
		b.modifiers |= fyne.KeyModifierSuper
		isModifier = true
	}

	if isModifier {
		return
	}
	if e.Name == fyne.KeyEscape {
		b.CancelCapture()
		return
	}

	var parts []string
	if b.modifiers&fyne.KeyModifierControl != 0 {
		parts = append(parts, "Ctrl")
	}
	if b.modifiers&fyne.KeyModifierShift != 0 {
		parts = append(parts, "Shift")
	}
	if b.modifiers&fyne.KeyModifierAlt != 0 {
		parts = append(parts, "Alt")
	}
	if b.modifiers&fyne.KeyModifierSuper != 0 {
		parts = append(parts, hotkeys.SuperKeyIdentifier)
	}

	keyName := string(e.Name)
	if len(keyName) == 1 {
		// Convert single character keys to uppercase, e.g. "a" -> "A"
		keyName = strings.ToUpper(keyName)
	}

	parts = append(parts, keyName)
	shortcut := strings.Join(parts, "+")

	b.value = shortcut
	b.Text = shortcut
	b.isCapturing = false
	b.Refresh()

	if b.OnChanged != nil {
		b.OnChanged(shortcut)
	}
}

func (b *HotkeyButton) KeyUp(e *fyne.KeyEvent) {
	if !b.isCapturing {
		return
	}
	switch e.Name {
	case desktop.KeyShiftLeft, desktop.KeyShiftRight:
		b.modifiers &^= fyne.KeyModifierShift
	case desktop.KeyControlLeft, desktop.KeyControlRight:
		b.modifiers &^= fyne.KeyModifierControl
	case desktop.KeyAltLeft, desktop.KeyAltRight:
		b.modifiers &^= fyne.KeyModifierAlt
	case desktop.KeySuperLeft, desktop.KeySuperRight:
		b.modifiers &^= fyne.KeyModifierSuper
	}
}

func (b *HotkeyButton) CancelCapture() {
	if !b.isCapturing {
		return
	}
	b.isCapturing = false
	b.Text = b.value
	b.Refresh()

	if b.OnCancelled != nil {
		b.OnCancelled()
	}
}
