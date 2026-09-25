// Package editor lets the user mark up a screenshot before it's uploaded:
// boxes, arrows, freehand lines, a highlighter, and blur for hiding things.
package editor

import (
	"bytes"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"math"
)

type Tool int

const (
	ToolBox Tool = iota
	ToolArrow
	ToolPen
	ToolHighlight
	ToolBlur
)

// Shape is one thing drawn on the screenshot. The pen uses all points; the
// other tools only the first and the last.
type Shape struct {
	Tool   Tool
	Color  color.NRGBA
	Width  int
	Points []image.Point
}

func (s *Shape) rect() image.Rectangle {
	first, last := s.Points[0], s.Points[len(s.Points)-1]
	return image.Rectangle{Min: first, Max: last}.Canon()
}

// Drawing is a screenshot with shapes on it. Finished shapes are drawn into
// a cached image, so previewing the shape being drawn stays quick.
type Drawing struct {
	base      *image.NRGBA
	shapes    []Shape
	committed *image.NRGBA
}

// New starts a drawing on a copy of the screenshot.
func New(screenshot image.Image) *Drawing {
	bounds := screenshot.Bounds()
	base := image.NewNRGBA(image.Rect(0, 0, bounds.Dx(), bounds.Dy()))
	draw.Draw(base, base.Bounds(), screenshot, bounds.Min, draw.Src)
	return &Drawing{base: base, committed: clone(base)}
}

// Decode starts a drawing from PNG or JPEG data.
func Decode(data []byte) (*Drawing, error) {
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	return New(img), nil
}

func (d *Drawing) Bounds() image.Rectangle { return d.base.Bounds() }

// LineWidth is a line thickness that suits the screenshot's size.
func (d *Drawing) LineWidth() int {
	size := min(d.base.Bounds().Dx(), d.base.Bounds().Dy())
	return max(3, size/250)
}

// Add finishes a shape. Shapes too small to see (a click) are left out.
func (d *Drawing) Add(shape Shape) bool {
	if len(shape.Points) == 0 {
		return false
	}
	if shape.Tool != ToolPen {
		r := shape.rect()
		if r.Dx() < 3 && r.Dy() < 3 {
			return false
		}
	}
	d.shapes = append(d.shapes, shape)
	drawShape(d.committed, shape)
	return true
}

// Undo removes the latest shape.
func (d *Drawing) Undo() bool {
	if len(d.shapes) == 0 {
		return false
	}
	d.shapes = d.shapes[:len(d.shapes)-1]
	d.committed = clone(d.base)
	for _, shape := range d.shapes {
		drawShape(d.committed, shape)
	}
	return true
}

// CanUndo reports whether there's anything to undo.
func (d *Drawing) CanUndo() bool { return len(d.shapes) > 0 }

// Preview returns the drawing with a shape that's still being drawn.
func (d *Drawing) Preview(shape *Shape) image.Image {
	if shape == nil || len(shape.Points) == 0 {
		return d.committed
	}
	preview := clone(d.committed)
	drawShape(preview, *shape)
	return preview
}

// Image returns the finished drawing.
func (d *Drawing) Image() *image.NRGBA { return d.committed }

// PNG encodes the finished drawing.
func (d *Drawing) PNG() ([]byte, error) {
	var buffer bytes.Buffer
	err := png.Encode(&buffer, d.committed)
	return buffer.Bytes(), err
}

func clone(img *image.NRGBA) *image.NRGBA {
	copied := image.NewNRGBA(img.Bounds())
	copy(copied.Pix, img.Pix)
	return copied
}

func drawShape(dst *image.NRGBA, shape Shape) {
	first, last := shape.Points[0], shape.Points[len(shape.Points)-1]
	switch shape.Tool {
	case ToolBox:
		r := shape.rect()
		corners := []image.Point{r.Min, {r.Max.X, r.Min.Y}, r.Max, {r.Min.X, r.Max.Y}, r.Min}
		for i := 0; i < 4; i++ {
			thickLine(dst, corners[i], corners[i+1], shape.Width, shape.Color)
		}
	case ToolArrow:
		drawArrow(dst, first, last, shape.Width, shape.Color)
	case ToolPen:
		if len(shape.Points) == 1 {
			fillCircle(dst, first.X, first.Y, float64(shape.Width)/2, shape.Color)
		}
		for i := 1; i < len(shape.Points); i++ {
			thickLine(dst, shape.Points[i-1], shape.Points[i], shape.Width, shape.Color)
		}
	case ToolHighlight:
		highlight := shape.Color
		highlight.A = 90
		fillRect(dst, shape.rect(), highlight)
	case ToolBlur:
		pixelate(dst, shape.rect(), max(10, shape.Width*3))
	}
}

// blend paints one pixel, mixing by the color's transparency.
func blend(dst *image.NRGBA, x, y int, c color.NRGBA, coverage float64) {
	if !(image.Point{x, y}.In(dst.Rect)) || coverage <= 0 {
		return
	}
	alpha := float64(c.A) / 255 * min(coverage, 1)
	i := dst.PixOffset(x, y)
	pix := dst.Pix[i : i+4 : i+4]
	pix[0] = uint8(float64(pix[0])*(1-alpha) + float64(c.R)*alpha)
	pix[1] = uint8(float64(pix[1])*(1-alpha) + float64(c.G)*alpha)
	pix[2] = uint8(float64(pix[2])*(1-alpha) + float64(c.B)*alpha)
	pix[3] = 255
}

func fillCircle(dst *image.NRGBA, cx, cy int, radius float64, c color.NRGBA) {
	r := int(math.Ceil(radius)) + 1
	for y := cy - r; y <= cy+r; y++ {
		for x := cx - r; x <= cx+r; x++ {
			distance := math.Hypot(float64(x-cx), float64(y-cy))
			// Soft edge for smooth lines
			blend(dst, x, y, c, radius+0.5-distance)
		}
	}
}

// thickLine draws a line with round ends by stamping circles along it.
func thickLine(dst *image.NRGBA, from, to image.Point, width int, c color.NRGBA) {
	radius := float64(width) / 2
	dx, dy := float64(to.X-from.X), float64(to.Y-from.Y)
	length := math.Hypot(dx, dy)
	step := max(1, radius/2)
	for t := 0.0; t <= length; t += step {
		x := float64(from.X) + dx*t/math.Max(length, 1)
		y := float64(from.Y) + dy*t/math.Max(length, 1)
		fillCircle(dst, int(math.Round(x)), int(math.Round(y)), radius, c)
	}
	fillCircle(dst, to.X, to.Y, radius, c)
}

func drawArrow(dst *image.NRGBA, from, to image.Point, width int, c color.NRGBA) {
	dx, dy := float64(to.X-from.X), float64(to.Y-from.Y)
	length := math.Hypot(dx, dy)
	if length < 1 {
		return
	}
	ux, uy := dx/length, dy/length
	head := math.Min(math.Max(float64(width)*4.5, 14), length)
	half := head * 0.6

	// The shaft stops inside the head, so its round end doesn't poke out
	baseX, baseY := float64(to.X)-ux*head, float64(to.Y)-uy*head
	shaftEnd := image.Point{int(math.Round(baseX + ux*head*0.3)), int(math.Round(baseY + uy*head*0.3))}
	thickLine(dst, from, shaftEnd, width, c)

	tip := [2]float64{float64(to.X), float64(to.Y)}
	left := [2]float64{baseX - uy*half, baseY + ux*half}
	right := [2]float64{baseX + uy*half, baseY - ux*half}
	fillTriangle(dst, tip, left, right, c)
}

func fillTriangle(dst *image.NRGBA, a, b, c [2]float64, col color.NRGBA) {
	minX := int(math.Floor(math.Min(a[0], math.Min(b[0], c[0]))))
	maxX := int(math.Ceil(math.Max(a[0], math.Max(b[0], c[0]))))
	minY := int(math.Floor(math.Min(a[1], math.Min(b[1], c[1]))))
	maxY := int(math.Ceil(math.Max(a[1], math.Max(b[1], c[1]))))
	edge := func(p, q [2]float64, x, y float64) float64 {
		return (q[0]-p[0])*(y-p[1]) - (q[1]-p[1])*(x-p[0])
	}
	area := edge(a, b, c[0], c[1])
	if area == 0 {
		return
	}
	for y := minY; y <= maxY; y++ {
		for x := minX; x <= maxX; x++ {
			// Four samples per pixel for smooth edges
			hits := 0
			for _, offset := range [][2]float64{{0.25, 0.25}, {0.75, 0.25}, {0.25, 0.75}, {0.75, 0.75}} {
				px, py := float64(x)+offset[0], float64(y)+offset[1]
				w0, w1, w2 := edge(b, c, px, py)/area, edge(c, a, px, py)/area, edge(a, b, px, py)/area
				if w0 >= 0 && w1 >= 0 && w2 >= 0 {
					hits++
				}
			}
			blend(dst, x, y, col, float64(hits)/4)
		}
	}
}

func fillRect(dst *image.NRGBA, r image.Rectangle, c color.NRGBA) {
	r = r.Intersect(dst.Rect)
	for y := r.Min.Y; y < r.Max.Y; y++ {
		for x := r.Min.X; x < r.Max.X; x++ {
			blend(dst, x, y, c, 1)
		}
	}
}

// pixelate turns an area into big blocks of its average colors, so text in
// it can't be read anymore.
func pixelate(dst *image.NRGBA, r image.Rectangle, block int) {
	r = r.Intersect(dst.Rect)
	for top := r.Min.Y; top < r.Max.Y; top += block {
		for left := r.Min.X; left < r.Max.X; left += block {
			cell := image.Rect(left, top, left+block, top+block).Intersect(r)
			var sum [3]int
			count := 0
			for y := cell.Min.Y; y < cell.Max.Y; y++ {
				for x := cell.Min.X; x < cell.Max.X; x++ {
					i := dst.PixOffset(x, y)
					sum[0] += int(dst.Pix[i])
					sum[1] += int(dst.Pix[i+1])
					sum[2] += int(dst.Pix[i+2])
					count++
				}
			}
			if count == 0 {
				continue
			}
			average := color.NRGBA{uint8(sum[0] / count), uint8(sum[1] / count), uint8(sum[2] / count), 255}
			fillRect(dst, cell, average)
		}
	}
}
