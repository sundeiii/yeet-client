package editor

import (
	"github.com/sundeiii/yeet-client/internal/i18n"
	"image"
	"image/color"
	"log"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

var palette = []color.NRGBA{
	{229, 57, 53, 255},   // red
	{253, 216, 53, 255},  // yellow
	{140, 198, 62, 255},  // green
	{30, 136, 229, 255},  // blue
	{255, 255, 255, 255}, // white
	{20, 20, 20, 255},    // black
}

var toolNames = []struct {
	tool Tool
	name string
}{
	{ToolBox, "Box"},
	{ToolArrow, "Arrow"},
	{ToolPen, "Pen"},
	{ToolHighlight, "Highlight"},
	{ToolBlur, "Blur"},
}

// Open shows the editor for a screenshot. done gets the edited picture as a
// PNG, or nil when the user cancels (or closes the window). It's called once.
// Must be called on the main thread.
func Open(app fyne.App, screenshot []byte, done func([]byte)) {
	drawing, err := Decode(screenshot)
	if err != nil {
		log.Printf("Editor could not read the screenshot: %v", err)
		done(screenshot)
		return
	}

	w := app.NewWindow(i18n.T("puush editor"))
	finished := false
	finish := func(result []byte) {
		if finished {
			return
		}
		finished = true
		done(result)
		w.Close()
	}
	w.SetOnClosed(func() {
		if !finished {
			finished = true
			done(nil)
		}
	})

	area := newDrawArea(drawing)

	// Tools
	var toolButtons []*widget.Button
	selectTool := func(tool Tool) {
		area.tool = tool
		for i, button := range toolButtons {
			if toolNames[i].tool == tool {
				button.Importance = widget.HighImportance
			} else {
				button.Importance = widget.MediumImportance
			}
			button.Refresh()
		}
	}
	tools := container.NewHBox()
	for _, entry := range toolNames {
		entry := entry
		button := widget.NewButton(i18n.T(entry.name), func() { selectTool(entry.tool) })
		toolButtons = append(toolButtons, button)
		tools.Add(button)
	}
	selectTool(ToolBox)

	// Colors
	var swatches []*swatch
	colors := container.NewHBox()
	for _, c := range palette {
		c := c
		s := newSwatch(c, func() {
			area.color = c
			for _, other := range swatches {
				other.setSelected(other.color == c)
			}
		})
		swatches = append(swatches, s)
		colors.Add(s)
	}
	swatches[0].setSelected(true)

	var undo *widget.Button
	undo = widget.NewButtonWithIcon(i18n.T("Undo"), theme.ContentUndoIcon(), func() {
		area.undo()
	})
	area.onChange = func() {
		if drawing.CanUndo() {
			undo.Enable()
		} else {
			undo.Disable()
		}
	}
	area.onChange()

	cancel := widget.NewButton(i18n.T("Cancel"), func() { finish(nil) })
	upload := widget.NewButtonWithIcon(i18n.T("Upload"), theme.UploadIcon(), func() {
		data, err := drawing.PNG()
		if err != nil {
			log.Printf("Editor could not save the picture: %v", err)
			data = screenshot
		}
		finish(data)
	})
	upload.Importance = widget.HighImportance

	toolbar := container.NewHBox(tools, widget.NewSeparator(), colors, layout.NewSpacer(), undo, cancel, upload)
	w.SetContent(container.NewBorder(container.NewPadded(toolbar), nil, nil, nil, area))

	// Keyboard: Ctrl+Z undoes, Enter uploads, Escape cancels
	w.Canvas().AddShortcut(&desktop.CustomShortcut{KeyName: fyne.KeyZ, Modifier: fyne.KeyModifierShortcutDefault}, func(fyne.Shortcut) {
		area.undo()
	})
	w.Canvas().SetOnTypedKey(func(event *fyne.KeyEvent) {
		switch event.Name {
		case fyne.KeyEscape:
			finish(nil)
		case fyne.KeyReturn, fyne.KeyEnter:
			upload.OnTapped()
		}
	})

	// As big as the screenshot, within reason
	bounds := drawing.Bounds()
	width, height := float32(bounds.Dx()), float32(bounds.Dy())
	scale := min(1, 1100/width, 700/height)
	w.Resize(fyne.NewSize(max(720, width*scale), height*scale+60))
	w.CenterOnScreen()
	w.Show()
	w.RequestFocus()
}

// drawArea shows the drawing and turns mouse drags into shapes.
type drawArea struct {
	widget.BaseWidget

	drawing  *Drawing
	image    *canvas.Image
	tool     Tool
	color    color.NRGBA
	current  *Shape
	onChange func()
}

var _ desktop.Mouseable = (*drawArea)(nil)
var _ fyne.Draggable = (*drawArea)(nil)

func newDrawArea(drawing *Drawing) *drawArea {
	area := &drawArea{drawing: drawing, color: palette[0]}
	area.image = canvas.NewImageFromImage(drawing.Image())
	area.image.FillMode = canvas.ImageFillContain
	area.ExtendBaseWidget(area)
	return area
}

func (a *drawArea) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(a.image)
}

func (a *drawArea) MinSize() fyne.Size {
	return fyne.NewSize(200, 150)
}

// toImage turns a position in the widget into a pixel of the screenshot,
// which is shown scaled down and centered.
func (a *drawArea) toImage(pos fyne.Position) image.Point {
	bounds := a.drawing.Bounds()
	size := a.Size()
	width, height := float32(bounds.Dx()), float32(bounds.Dy())
	scale := min(size.Width/width, size.Height/height)
	if scale <= 0 {
		return image.Point{}
	}
	offsetX := (size.Width - width*scale) / 2
	offsetY := (size.Height - height*scale) / 2
	x := int((pos.X - offsetX) / scale)
	y := int((pos.Y - offsetY) / scale)
	return image.Point{max(0, min(x, bounds.Dx()-1)), max(0, min(y, bounds.Dy()-1))}
}

func (a *drawArea) MouseDown(event *desktop.MouseEvent) {
	if event.Button != desktop.MouseButtonPrimary {
		return
	}
	a.current = &Shape{
		Tool:   a.tool,
		Color:  a.color,
		Width:  a.drawing.LineWidth(),
		Points: []image.Point{a.toImage(event.Position)},
	}
}

func (a *drawArea) Dragged(event *fyne.DragEvent) {
	if a.current == nil {
		return
	}
	point := a.toImage(event.Position)
	if a.current.Tool == ToolPen || len(a.current.Points) == 1 {
		a.current.Points = append(a.current.Points, point)
	} else {
		a.current.Points[len(a.current.Points)-1] = point
	}
	a.show(a.drawing.Preview(a.current))
}

func (a *drawArea) DragEnd() {
	a.commit()
}

// MouseUp finishes the shape. (Fyne sends it before DragEnd.) A pen click
// leaves a dot; other shapes that small are left out.
func (a *drawArea) MouseUp(event *desktop.MouseEvent) {
	a.commit()
}

func (a *drawArea) commit() {
	if a.current == nil {
		return
	}
	a.drawing.Add(*a.current)
	a.current = nil
	a.show(a.drawing.Image())
	if a.onChange != nil {
		a.onChange()
	}
}

func (a *drawArea) undo() {
	if a.drawing.Undo() {
		a.show(a.drawing.Image())
		if a.onChange != nil {
			a.onChange()
		}
	}
}

func (a *drawArea) show(img image.Image) {
	a.image.Image = img
	a.image.Refresh()
}

// swatch is a color button.
type swatch struct {
	widget.BaseWidget
	color    color.NRGBA
	fill     *canvas.Rectangle
	onTapped func()
}

func newSwatch(c color.NRGBA, onTapped func()) *swatch {
	s := &swatch{color: c, onTapped: onTapped}
	s.fill = canvas.NewRectangle(c)
	s.fill.StrokeColor = color.NRGBA{120, 120, 120, 255}
	s.fill.StrokeWidth = 1
	s.fill.CornerRadius = 4
	s.ExtendBaseWidget(s)
	return s
}

func (s *swatch) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(s.fill)
}

func (s *swatch) MinSize() fyne.Size {
	return fyne.NewSquareSize(26)
}

func (s *swatch) Tapped(*fyne.PointEvent) {
	s.onTapped()
}

func (s *swatch) Cursor() desktop.Cursor {
	return desktop.PointerCursor
}

func (s *swatch) setSelected(selected bool) {
	if selected {
		s.fill.StrokeColor = color.NRGBA{0, 102, 204, 255}
		s.fill.StrokeWidth = 3
	} else {
		s.fill.StrokeColor = color.NRGBA{120, 120, 120, 255}
		s.fill.StrokeWidth = 1
	}
	s.fill.Refresh()
}
