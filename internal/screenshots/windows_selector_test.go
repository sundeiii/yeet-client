//go:build windows

package screenshots

import (
	"image"
	"image/color"
	"image/png"
	"os"
	"testing"
)

// renderForTest draws the selector for a made-up screen and returns the result.
func renderForTest(t *testing.T, setup func(state *selectionState)) *image.NRGBA {
	t.Helper()

	// A colorful "screen" at a negative position, like a monitor left of the main one
	shot := image.NewNRGBA(image.Rect(-400, 0, 400, 500))
	for y := shot.Rect.Min.Y; y < shot.Rect.Max.Y; y++ {
		for x := shot.Rect.Min.X; x < shot.Rect.Max.X; x++ {
			shot.Set(x, y, color.NRGBA{uint8(x), uint8(y), uint8(x ^ y), 255})
		}
	}

	frozen, err := newFrozenScreen(shot)
	if err != nil {
		t.Fatal(err)
	}
	defer frozen.close()

	state := &selectionState{virtualX: -400, virtualY: 0, width: 800, height: 500, frozen: frozen}
	setup(state)
	renderSelector(state)

	// Read the back buffer; the bitmap has to be taken out of its DC first
	selectObject(frozen.backDC, frozen.oldBack)
	frozen.oldBack = 0
	screenDC, _ := getDC(0)
	defer releaseDC(0, screenDC)
	img, err := bitmapToNRGBA(screenDC, frozen.backBmp, 800, 500)
	if err != nil {
		t.Fatal(err)
	}

	if path := os.Getenv("SELECTOR_PREVIEW"); path != "" {
		file, _ := os.Create(path + "-" + t.Name() + ".png")
		png.Encode(file, img)
		file.Close()
	}
	return img
}

func TestSelectorRendersSelection(t *testing.T) {
	img := renderForTest(t, func(state *selectionState) {
		state.dragging = true
		state.start = point{-300, 100}
		state.current = point{0, 300}
		state.cursor = state.current
	})

	// Inside the selection the screen is at full brightness...
	inside := img.NRGBAAt(-150+400, 200)
	if inside.G != 200 {
		t.Errorf("selected pixels should be untouched, got %v", inside)
	}
	// ...and outside it's dimmed
	outside := img.NRGBAAt(350+400, 50)
	if outside.G >= 50 {
		t.Errorf("pixels outside the selection should be dimmed, got %v", outside)
	}
}

func TestSelectorIdle(t *testing.T) {
	renderForTest(t, func(state *selectionState) {
		// Near the bottom-right corner, so the magnifier flips around
		state.cursor = point{380, 480}
	})
}

func TestAreaSelectionImage(t *testing.T) {
	shot := image.NewNRGBA(image.Rect(-100, 0, 100, 100))
	shot.Set(-50, 10, color.NRGBA{255, 0, 0, 255})
	selection := areaSelection{area: image.Rect(-60, 5, -40, 15), shot: shot}

	img := selection.Image()
	if img.Bounds().Dx() != 20 || img.Bounds().Dy() != 10 {
		t.Fatalf("unexpected size %v", img.Bounds())
	}
	if r, _, _, _ := img.At(-50, 10).RGBA(); r != 0xffff {
		t.Error("the selected area should come from the right place")
	}
}
