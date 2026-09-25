package editor

import (
	"image"
	"image/color"
	"image/png"
	"os"
	"testing"
)

// A screenshot with fine stripes, like text, to see what blur does to it
func testScreenshot() *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, 400, 300))
	for y := 0; y < 300; y++ {
		for x := 0; x < 400; x++ {
			c := color.NRGBA{240, 240, 240, 255}
			if x%4 < 2 {
				c = color.NRGBA{20, 20, 20, 255}
			}
			img.SetNRGBA(x, y, c)
		}
	}
	return img
}

var red = color.NRGBA{229, 57, 53, 255}

func TestShapes(t *testing.T) {
	d := New(testScreenshot())
	width := d.LineWidth()

	if !d.Add(Shape{Tool: ToolBox, Color: red, Width: width, Points: []image.Point{{20, 20}, {120, 80}}}) {
		t.Fatal("box not added")
	}
	if got := d.Image().NRGBAAt(70, 20); got != red {
		t.Errorf("box edge should be red, got %v", got)
	}
	if got := d.Image().NRGBAAt(70, 50); got == red {
		t.Error("inside of the box should stay as it was")
	}

	d.Add(Shape{Tool: ToolArrow, Color: red, Width: width, Points: []image.Point{{200, 250}, {350, 150}}})
	if got := d.Image().NRGBAAt(340, 158); got != red {
		t.Errorf("arrow head should be filled, got %v", got)
	}

	// Blur makes stripes unreadable: neighbours become the same color
	d.Add(Shape{Tool: ToolBlur, Width: width, Points: []image.Point{{200, 20}, {380, 120}}})
	a, b := d.Image().NRGBAAt(201, 21), d.Image().NRGBAAt(203, 21)
	if a != b {
		t.Errorf("blurred pixels should match, got %v and %v", a, b)
	}
	if d.Image().NRGBAAt(199, 21) != testScreenshot().NRGBAAt(199, 21) {
		t.Error("blur must stay inside its area")
	}

	// A click without dragging draws nothing
	if d.Add(Shape{Tool: ToolBox, Color: red, Width: width, Points: []image.Point{{5, 5}, {6, 6}}}) {
		t.Error("tiny boxes should be left out")
	}

	if path := os.Getenv("EDITOR_PREVIEW"); path != "" {
		file, _ := os.Create(path)
		png.Encode(file, d.Image())
		file.Close()
	}

	// Undo takes the blur back off
	d.Undo()
	if d.Image().NRGBAAt(201, 21) == d.Image().NRGBAAt(203, 21) {
		t.Error("undo should bring the stripes back")
	}
	d.Undo()
	d.Undo()
	if d.CanUndo() || d.Undo() {
		t.Error("nothing should be left to undo")
	}
	if d.Image().NRGBAAt(70, 20) == red {
		t.Error("everything should be undone")
	}
}

func TestPreviewLeavesDrawingAlone(t *testing.T) {
	d := New(testScreenshot())
	shape := &Shape{Tool: ToolHighlight, Color: color.NRGBA{253, 216, 53, 255}, Width: 3, Points: []image.Point{{0, 0}, {100, 100}}}
	preview := d.Preview(shape).(*image.NRGBA)
	if preview.NRGBAAt(50, 50) == d.Image().NRGBAAt(50, 50) {
		t.Error("the preview should show the highlight")
	}
	if d.CanUndo() {
		t.Error("previewing must not add the shape")
	}

	data, err := d.PNG()
	if err != nil {
		t.Fatal(err)
	}
	again, err := Decode(data)
	if err != nil || again.Bounds().Dx() != 400 {
		t.Errorf("PNG round trip: %v", err)
	}
}
