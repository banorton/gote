package render

import (
	"image"
	"math"
	"strings"
	"sync"

	"github.com/fogleman/gg"
	"github.com/golang/freetype/truetype"
	"golang.org/x/image/font"
	"golang.org/x/image/font/gofont/gomono"
	"golang.org/x/image/font/gofont/goregular"
)

const lineHeight = 1.25

var (
	fontOnce  sync.Once
	regular   *truetype.Font
	mono      *truetype.Font
	faceMu    sync.Mutex
	faceCache = map[faceKey]font.Face{}
)

type faceKey struct {
	mono bool
	size float64
}

// face returns the Go Regular face (Go Mono for Excalidraw's code family 3).
func face(family int, size float64) font.Face {
	fontOnce.Do(func() {
		var err error
		if regular, err = truetype.Parse(goregular.TTF); err != nil {
			panic(err)
		}
		if mono, err = truetype.Parse(gomono.TTF); err != nil {
			panic(err)
		}
	})
	k := faceKey{family == 3, math.Round(size*64) / 64}
	faceMu.Lock()
	defer faceMu.Unlock()
	if f, ok := faceCache[k]; ok {
		return f
	}
	src := regular
	if k.mono {
		src = mono
	}
	f := truetype.NewFace(src, &truetype.Options{Size: k.size})
	faceCache[k] = f
	return f
}

func textWidth(f font.Face, s string) float64 {
	return float64(font.MeasureString(f, s)) / 64
}

type textBlock struct {
	lines []string
	size  float64
	w, h  float64 // scene units
}

func (e *element) block() textBlock {
	size := e.FontSize
	if size <= 0 {
		size = 20
	}
	lines := strings.Split(e.Text, "\n")
	f := face(e.FontFamily, size)
	w := 0.0
	for _, l := range lines {
		w = math.Max(w, textWidth(f, l))
	}
	return textBlock{lines, size, w, float64(len(lines)) * size * lineHeight}
}

// boundTextBox returns the scene-space box of text bound to container c.
func (r *renderer) boundTextBox(e, c *element) (x, y, w, h float64) {
	b := e.block()
	cx, cy, cw, ch := c.box()
	if isLinear(c.Type) {
		mx, my := polylineMid(c)
		return mx - b.w/2, my - b.h/2, b.w, b.h
	}
	ix, iw := cx+5, cw-10
	if c.Type != "rectangle" {
		iw = cw * 0.7
		ix = cx + (cw-iw)/2
	}
	switch align(e, true) {
	case "left":
		x = ix
	case "right":
		x = ix + iw - b.w
	default:
		x = ix + (iw-b.w)/2
	}
	switch e.VerticalAlign {
	case "top":
		y = cy + 5
	case "bottom":
		y = cy + ch - 5 - b.h
	default:
		y = cy + (ch-b.h)/2
	}
	return x, y, b.w, b.h
}

func align(e *element, bound bool) string {
	switch e.TextAlign {
	case "left", "center", "right":
		return e.TextAlign
	}
	if bound {
		return "center"
	}
	return "left"
}

func polylineMid(e *element) (float64, float64) {
	if len(e.Points) == 0 {
		return e.X, e.Y
	}
	total := 0.0
	for i := 1; i < len(e.Points); i++ {
		total += math.Hypot(e.Points[i][0]-e.Points[i-1][0], e.Points[i][1]-e.Points[i-1][1])
	}
	half := total / 2
	for i := 1; i < len(e.Points); i++ {
		a, b := e.Points[i-1], e.Points[i]
		seg := math.Hypot(b[0]-a[0], b[1]-a[1])
		if half <= seg && seg > 0 {
			t := half / seg
			return e.X + a[0] + (b[0]-a[0])*t, e.Y + a[1] + (b[1]-a[1])*t
		}
		half -= seg
	}
	last := e.Points[len(e.Points)-1]
	return e.X + last[0], e.Y + last[1]
}

func (r *renderer) drawText(e *element) error {
	if strings.TrimSpace(e.Text) == "" {
		return nil
	}
	col, err := e.stroke()
	if err != nil {
		return err
	}
	b := e.block()
	var c *element
	if e.ContainerID != nil {
		c = r.byID[*e.ContainerID]
	}
	var x, y float64
	var al string
	angle, rcx, rcy := e.Angle, 0.0, 0.0
	if c != nil {
		x, y, _, _ = r.boundTextBox(e, c)
		al = align(e, true)
		angle = c.Angle
		cx, cy, cw, ch := c.box()
		rcx, rcy = cx+cw/2, cy+ch/2
	} else {
		bx, by, bw, bh := e.box()
		x, y = bx, by
		al = align(e, false)
		switch al {
		case "center":
			x = bx + (bw-b.w)/2
		case "right":
			x = bx + bw - b.w
		}
		rcx, rcy = bx+bw/2, by+bh/2
	}

	if c != nil && isLinear(c.Type) {
		pad := 3.0
		r.dc.SetColor(r.bg)
		r.dc.DrawRectangle(x-pad, y, b.w+2*pad, b.h)
		r.dc.Fill()
	}
	f := face(e.FontFamily, b.size*r.s)
	m := f.Metrics()
	asc, desc := float64(m.Ascent)/64, float64(m.Descent)/64
	lh := b.size * lineHeight * r.s
	drawLines := func(dc *gg.Context, left, top float64) {
		dc.SetFontFace(f)
		dc.SetColor(col)
		for i, l := range b.lines {
			lw := textWidth(f, l)
			lx := left
			switch al {
			case "center":
				lx += (b.w*r.s - lw) / 2
			case "right":
				lx += b.w*r.s - lw
			}
			base := top + float64(i)*lh + (lh-(asc+desc))/2 + asc
			dc.DrawString(l, lx, base)
		}
	}
	dc := r.dc
	dx, dy := r.dev(x, y)
	if angle == 0 {
		dc.Push()
		dc.Identity()
		drawLines(dc, dx, dy)
		dc.Pop()
		return nil
	}
	lw, lhh := int(math.Ceil(b.w*r.s))+2, int(math.Ceil(b.h*r.s))+2
	layer := gg.NewContextForRGBA(image.NewRGBA(image.Rect(0, 0, lw, lhh)))
	drawLines(layer, 0, 0)
	ccx, ccy := r.dev(rcx, rcy)
	px, py := dx+float64(lw)/2, dy+float64(lhh)/2
	dc.Push()
	dc.Identity()
	dc.RotateAbout(angle, ccx, ccy)
	dc.DrawImageAnchored(layer.Image(), int(math.Round(px)), int(math.Round(py)), 0.5, 0.5)
	dc.Pop()
	return nil
}
