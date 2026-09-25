package desktop

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
)

var _ fyne.Theme = (*classicTheme)(nil)

// classicTheme is a custom Fyne theme designed to mimic the classic Windows UI
type classicTheme struct{}

func NewWindowsTheme() fyne.Theme {
	return &classicTheme{}
}

func (w *classicTheme) Font(style fyne.TextStyle) fyne.Resource {
	style.Bold = false
	return theme.DefaultTheme().Font(style)
}

func (w *classicTheme) Icon(name fyne.ThemeIconName) fyne.Resource {
	return theme.DefaultTheme().Icon(name)
}

func (w *classicTheme) Color(name fyne.ThemeColorName, variant fyne.ThemeVariant) color.Color {
	switch name {
	case theme.ColorNameBackground:
		return color.NRGBA{R: 240, G: 240, B: 240, A: 255}
	case theme.ColorNameButton:
		return color.NRGBA{R: 255, G: 255, B: 255, A: 255}
	case theme.ColorNameDisabledButton:
		return color.NRGBA{R: 255, G: 255, B: 255, A: 255}
	case theme.ColorNameDisabled:
		return color.NRGBA{R: 160, G: 160, B: 160, A: 255}
	case theme.ColorNameHover:
		return color.NRGBA{R: 216, G: 230, B: 242, A: 255}
	case theme.ColorNameInputBackground, theme.ColorNameMenuBackground:
		return color.NRGBA{R: 255, G: 255, B: 255, A: 255}
	case theme.ColorNamePrimary, theme.ColorNameHyperlink:
		return color.NRGBA{R: 0, G: 102, B: 204, A: 255}
	case theme.ColorNameForeground:
		return color.NRGBA{R: 0, G: 0, B: 0, A: 255}
	case theme.ColorNamePlaceHolder:
		return color.NRGBA{R: 128, G: 128, B: 128, A: 255}
	case theme.ColorNameInputBorder:
		return color.NRGBA{R: 171, G: 173, B: 179, A: 255}
	case theme.ColorNameSeparator:
		return color.NRGBA{R: 213, G: 223, B: 229, A: 255}
	case theme.ColorNameShadow:
		return color.Transparent
	}
	return theme.DefaultTheme().Color(name, variant)
}

func (w *classicTheme) Size(name fyne.ThemeSizeName) float32 {
	switch name {
	case theme.SizeNameInputRadius, theme.SizeNameSelectionRadius, theme.SizeNameScrollBarRadius, theme.SizeNameWindowButtonRadius:
		return 0
	case theme.SizeNamePadding:
		return 4
	case theme.SizeNameText:
		return 12
	case theme.SizeNameInnerPadding:
		return 4
	case theme.SizeNameInputBorder:
		return 1
	}
	return theme.DefaultTheme().Size(name)
}
