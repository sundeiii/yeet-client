package desktop

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

// BorderedButton is a custom widget that creates regular buttons with a 1px border.
type BorderedButton struct {
	widget.BaseWidget
	Instance *widget.Button
}

// NewBorderedButton creates a new bordered button with the given text and callback.
func NewBorderedButton(text string, callback func()) *BorderedButton {
	b := &BorderedButton{Instance: widget.NewButton(text, callback)}
	b.ExtendBaseWidget(b)
	return b
}

// CreateRenderer implements the fyne.Widget interface.
func (b *BorderedButton) CreateRenderer() fyne.WidgetRenderer {
	// Create a transparent rectangle with a solid border
	border := canvas.NewRectangle(color.Transparent)
	border.StrokeColor = color.NRGBA{R: 171, G: 173, B: 179, A: 255}
	border.StrokeWidth = 1

	// Place button instance on top of the rect
	c := container.NewStack(b.Instance, border)
	return widget.NewSimpleRenderer(c)
}
