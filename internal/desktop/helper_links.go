package desktop

import (
	"image/color"
	"net/url"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/widget"
)

// UnderlinedLink is a custom widget that mimics a standard HTML hyperlink.
type UnderlinedLink struct {
	widget.BaseWidget
	Text *canvas.Text
	URL  *url.URL
}

func NewUnderlinedLink(text string, url *url.URL) *UnderlinedLink {
	link := &UnderlinedLink{
		Text: canvas.NewText(text, color.NRGBA{R: 0, G: 0, B: 238, A: 255}),
		URL:  url,
	}
	link.Text.TextSize = 12
	link.Text.TextStyle = fyne.TextStyle{Bold: true}
	link.ExtendBaseWidget(link)
	return link
}

func (link *UnderlinedLink) CreateRenderer() fyne.WidgetRenderer {
	line := canvas.NewLine(color.NRGBA{R: 0, G: 0, B: 238, A: 255})
	line.StrokeWidth = 1
	c := container.NewWithoutLayout(link.Text, line)
	return &linkRenderer{link: link, line: line, container: c}
}

func (link *UnderlinedLink) Tapped(*fyne.PointEvent) {
	if link.URL != nil {
		OpenBrowser(link.URL.String())
	}
}

func (link *UnderlinedLink) Cursor() desktop.Cursor {
	return desktop.PointerCursor
}

// linkRenderer handles laying out the text and the underline correctly.
type linkRenderer struct {
	link      *UnderlinedLink
	line      *canvas.Line
	container *fyne.Container
}

func (r *linkRenderer) Layout(s fyne.Size) {
	r.link.Text.Resize(s)
	// Draw the line at the very bottom of the text bounds
	r.line.Position1 = fyne.NewPos(0, s.Height-1)
	r.line.Position2 = fyne.NewPos(s.Width, s.Height-1)
}

func (r *linkRenderer) MinSize() fyne.Size {
	return r.link.Text.MinSize()
}

func (r *linkRenderer) Refresh() {
	r.container.Refresh()
}

func (r *linkRenderer) Objects() []fyne.CanvasObject {
	return []fyne.CanvasObject{r.container}
}

func (r *linkRenderer) Destroy() {}
