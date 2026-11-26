// Package render draws Excalidraw scenes to PNG in pure Go.
//
// Output is clean geometry, not the hand-drawn look: no roughness, hachure
// fills are semi-opaque solids, curves on lines are plain polylines, and text
// metrics come from the Go fonts, not Excalidraw's.
package render

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	_ "image/jpeg"
	"image/png"
	"math"
	"strings"

	"github.com/fogleman/gg"
)

const (
	padding   = 32.0
	maxSide   = 2000.0
	minCanvas = 64.0
)

type element struct {
	ID              string       `json:"id"`
	Type            string       `json:"type"`
	X               float64      `json:"x"`
	Y               float64      `json:"y"`
	Width           float64      `json:"width"`
	Height          float64      `json:"height"`
	Angle           float64      `json:"angle"`
	StrokeColor     *string      `json:"strokeColor"`
	BackgroundColor string       `json:"backgroundColor"`
	FillStyle       string       `json:"fillStyle"`
	StrokeWidth     *float64     `json:"strokeWidth"`
	StrokeStyle     string       `json:"strokeStyle"`
	Opacity         *float64     `json:"opacity"`
	Roundness       *struct{}    `json:"roundness"`
	IsDeleted       bool         `json:"isDeleted"`
	Points          [][2]float64 `json:"points"`
	StartArrowhead  *string      `json:"startArrowhead"`
	EndArrowhead    *string      `json:"endArrowhead"`
	Text            string       `json:"text"`
	FontSize        float64      `json:"fontSize"`
	FontFamily      int          `json:"fontFamily"`
	TextAlign       string       `json:"textAlign"`
	VerticalAlign   string       `json:"verticalAlign"`
	ContainerID     *string      `json:"containerId"`
	FileID          *string      `json:"fileId"`
	Name            *string      `json:"name"`
}

type scene struct {
	Type     string `json:"type"`
	Elements []element
	AppState struct {
		ViewBackgroundColor string `json:"viewBackgroundColor"`
	} `json:"appState"`
	Files map[string]struct {
		MimeType string `json:"mimeType"`
		DataURL  string `json:"dataURL"`
	} `json:"files"`
}

type renderer struct {
	dc    *gg.Context
	s     float64 // device pixels per scene unit
	ox    float64 // scene coordinate at device origin
	oy    float64
	byID  map[string]*element
	files map[string]image.Image
	bg    color.Color
}

// PNG renders src (an .excalidraw file) and returns the encoded PNG, stamped
// with Hash(src), and its pixel size.
func PNG(src []byte) (data []byte, w, h int, err error) {
	img, err := Render(src)
	if err != nil {
		return nil, 0, 0, err
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, 0, 0, fmt.Errorf("encode PNG: %w", err)
	}
	b := img.Bounds()
	return stamp(buf.Bytes(), Hash(src)), b.Dx(), b.Dy(), nil
}

// Render draws the live elements of an Excalidraw scene in array (z) order.
func Render(src []byte) (*image.RGBA, error) {
	var sc scene
	if err := json.Unmarshal(src, &sc); err != nil {
		return nil, fmt.Errorf("invalid excalidraw JSON: %w", err)
	}
	if sc.Type != "excalidraw" {
		return nil, fmt.Errorf("not an excalidraw scene: type is %q", sc.Type)
	}
	bg := color.Color(color.White)
	if v := sc.AppState.ViewBackgroundColor; v != "" {
		c, err := parseColor(v)
		if err != nil {
			return nil, fmt.Errorf("appState.viewBackgroundColor: %w", err)
		}
		bg = c
	}
	var els []*element
	r := &renderer{byID: map[string]*element{}, files: map[string]image.Image{}}
	for i := range sc.Elements {
		e := &sc.Elements[i]
		if e.IsDeleted {
			continue
		}
		els = append(els, e)
		r.byID[e.ID] = e
	}
	for id, f := range sc.Files {
		if img := decodeDataURL(f.DataURL); img != nil {
			r.files[id] = img
		}
	}

	minX, minY, maxX, maxY := math.Inf(1), math.Inf(1), math.Inf(-1), math.Inf(-1)
	for _, e := range els {
		x0, y0, x1, y1 := r.bounds(e)
		for _, v := range []float64{x0, y0, x1, y1} {
			if math.IsNaN(v) || math.IsInf(v, 0) {
				return nil, fmt.Errorf("element %q: non-finite coordinates", e.ID)
			}
		}
		minX, minY, maxX, maxY = math.Min(minX, x0), math.Min(minY, y0), math.Max(maxX, x1), math.Max(maxY, y1)
	}
	var cw, ch float64
	if len(els) == 0 {
		minX, minY = 0, 0
		cw, ch = minCanvas, minCanvas
		r.ox, r.oy = 0, 0
	} else {
		cw, ch = maxX-minX+2*padding, maxY-minY+2*padding
		r.ox, r.oy = minX-padding, minY-padding
	}
	r.s = math.Min(1, maxSide/math.Max(cw, ch))
	W, H := int(math.Ceil(cw*r.s)), int(math.Ceil(ch*r.s))
	r.bg = bg
	r.dc = gg.NewContext(W, H)
	r.dc.SetColor(bg)
	r.dc.Clear()
	r.dc.Scale(r.s, r.s)
	r.dc.Translate(-r.ox, -r.oy)

	for _, e := range els {
		if err := r.draw(e); err != nil {
			return nil, fmt.Errorf("element %q: %w", e.ID, err)
		}
	}
	return r.dc.Image().(*image.RGBA), nil
}

func decodeDataURL(u string) image.Image {
	head, body, ok := strings.Cut(u, ",")
	if !ok || !strings.HasSuffix(head, ";base64") || !(strings.HasPrefix(head, "data:image/png") || strings.HasPrefix(head, "data:image/jpeg")) {
		return nil
	}
	raw, err := base64.StdEncoding.DecodeString(body)
	if err != nil {
		return nil
	}
	img, _, err := image.Decode(bytes.NewReader(raw))
	if err != nil {
		return nil
	}
	return img
}

func isLinear(t string) bool { return t == "line" || t == "arrow" || t == "freedraw" }

func (e *element) box() (x, y, w, h float64) {
	x, y, w, h = e.X, e.Y, e.Width, e.Height
	if isLinear(e.Type) && len(e.Points) > 0 {
		x0, y0, x1, y1 := pointsBounds(e.Points)
		return e.X + x0, e.Y + y0, x1 - x0, y1 - y0
	}
	if w < 0 {
		x, w = x+w, -w
	}
	if h < 0 {
		y, h = y+h, -h
	}
	return
}

func pointsBounds(p [][2]float64) (x0, y0, x1, y1 float64) {
	x0, y0, x1, y1 = p[0][0], p[0][1], p[0][0], p[0][1]
	for _, q := range p {
		x0, y0, x1, y1 = math.Min(x0, q[0]), math.Min(y0, q[1]), math.Max(x1, q[0]), math.Max(y1, q[1])
	}
	return
}

// bounds is the scene-space bounding box including rotation and stroke.
func (r *renderer) bounds(e *element) (x0, y0, x1, y1 float64) {
	x, y, w, h := e.box()
	if e.Type == "text" && e.ContainerID != nil {
		if c := r.byID[*e.ContainerID]; c != nil {
			x, y, w, h = r.boundTextBox(e, c)
		}
	}
	pad := e.strokeWidth() / 2
	if e.Type == "frame" {
		y -= 24
		h += 24
	}
	cx, cy := x+w/2, y+h/2
	x0, y0, x1, y1 = math.Inf(1), math.Inf(1), math.Inf(-1), math.Inf(-1)
	for _, c := range [][2]float64{{x, y}, {x + w, y}, {x, y + h}, {x + w, y + h}} {
		px, py := rotate(c[0], c[1], cx, cy, e.Angle)
		x0, y0, x1, y1 = math.Min(x0, px), math.Min(y0, py), math.Max(x1, px), math.Max(y1, py)
	}
	return x0 - pad, y0 - pad, x1 + pad, y1 + pad
}

func rotate(x, y, cx, cy, a float64) (float64, float64) {
	if a == 0 {
		return x, y
	}
	s, c := math.Sincos(a)
	dx, dy := x-cx, y-cy
	return cx + dx*c - dy*s, cy + dx*s + dy*c
}

func (e *element) strokeWidth() float64 {
	if e.StrokeWidth == nil {
		return 2
	}
	return *e.StrokeWidth
}

func (e *element) alpha() float64 {
	if e.Opacity == nil {
		return 1
	}
	return math.Max(0, math.Min(100, *e.Opacity)) / 100
}

func (e *element) stroke() (color.Color, error) {
	s := "#1e1e1e"
	if e.StrokeColor != nil {
		s = *e.StrokeColor
	}
	c, err := parseColor(s)
	if err != nil {
		return nil, fmt.Errorf("strokeColor: %w", err)
	}
	return withAlpha(c, e.alpha()), nil
}

// fill returns nil for no fill. Hachure and cross-hatch fills are drawn as a
// semi-opaque solid.
func (e *element) fill() (color.Color, error) {
	if e.BackgroundColor == "" || e.BackgroundColor == "transparent" {
		return nil, nil
	}
	c, err := parseColor(e.BackgroundColor)
	if err != nil {
		return nil, fmt.Errorf("backgroundColor: %w", err)
	}
	a := e.alpha()
	if e.FillStyle == "hachure" || e.FillStyle == "cross-hatch" || e.FillStyle == "zigzag" {
		a *= 0.5
	}
	return withAlpha(c, a), nil
}

var namedColors = map[string]color.NRGBA{
	"black": {0, 0, 0, 255}, "white": {255, 255, 255, 255}, "red": {255, 0, 0, 255},
	"green": {0, 128, 0, 255}, "blue": {0, 0, 255, 255}, "yellow": {255, 255, 0, 255},
	"orange": {255, 165, 0, 255}, "purple": {128, 0, 128, 255}, "gray": {128, 128, 128, 255}, "grey": {128, 128, 128, 255},
}

func parseColor(s string) (color.NRGBA, error) {
	if s == "transparent" {
		return color.NRGBA{}, nil
	}
	if c, ok := namedColors[strings.ToLower(s)]; ok {
		return c, nil
	}
	if strings.HasPrefix(s, "#") {
		var v [4]uint8
		v[3] = 255
		h := s[1:]
		ok := true
		hex := func(c byte) uint8 {
			switch {
			case c >= '0' && c <= '9':
				return c - '0'
			case c >= 'a' && c <= 'f':
				return c - 'a' + 10
			case c >= 'A' && c <= 'F':
				return c - 'A' + 10
			}
			ok = false
			return 0
		}
		switch len(h) {
		case 3, 4:
			for i := range h {
				v[i] = hex(h[i]) * 17
			}
		case 6, 8:
			for i := 0; i < len(h); i += 2 {
				v[i/2] = hex(h[i])<<4 | hex(h[i+1])
			}
		default:
			ok = false
		}
		if ok {
			return color.NRGBA{v[0], v[1], v[2], v[3]}, nil
		}
	}
	return color.NRGBA{}, fmt.Errorf("unsupported color %q (use #rrggbb, #rgb, transparent or a basic color name)", s)
}

func withAlpha(c color.NRGBA, a float64) color.NRGBA {
	c.A = uint8(math.Round(float64(c.A) * a))
	return c
}

func (r *renderer) dev(x, y float64) (float64, float64) { return (x - r.ox) * r.s, (y - r.oy) * r.s }

func (r *renderer) draw(e *element) error {
	dc := r.dc
	dc.Push()
	defer dc.Pop()
	x, y, w, h := e.box()
	if e.Type != "text" {
		dc.RotateAbout(e.Angle, x+w/2, y+h/2)
	}
	switch e.Type {
	case "rectangle", "ellipse", "diamond":
		return r.drawShape(e, x, y, w, h)
	case "line", "arrow":
		return r.drawLine(e)
	case "freedraw":
		return r.drawFreedraw(e)
	case "text":
		return r.drawText(e)
	case "image":
		return r.drawImage(e, x, y, w, h)
	case "frame":
		return r.drawFrame(e, x, y, w, h)
	default:
		return r.drawUnsupported(e, x, y, w, h)
	}
}

func (r *renderer) setStroke(e *element) error {
	c, err := e.stroke()
	if err != nil {
		return err
	}
	dc := r.dc
	dc.SetColor(c)
	sw := e.strokeWidth()
	dc.SetLineWidth(sw)
	dc.SetLineJoin(gg.LineJoinRound)
	dc.SetLineCap(gg.LineCapRound)
	dc.SetDash()
	switch e.StrokeStyle {
	case "dashed":
		dc.SetLineCap(gg.LineCapButt)
		dc.SetDash(8, 8+sw)
	case "dotted":
		dc.SetDash(1, 3+sw*2)
	}
	return nil
}

func (r *renderer) drawShape(e *element, x, y, w, h float64) error {
	dc := r.dc
	path := func() {
		dc.NewSubPath()
		switch e.Type {
		case "rectangle":
			if e.Roundness != nil {
				dc.DrawRoundedRectangle(x, y, w, h, math.Min(32, math.Min(w, h)*0.25))
			} else {
				dc.DrawRectangle(x, y, w, h)
			}
			dc.ClosePath()
		case "ellipse":
			dc.DrawEllipse(x+w/2, y+h/2, w/2, h/2)
		case "diamond":
			dc.MoveTo(x+w/2, y)
			dc.LineTo(x+w, y+h/2)
			dc.LineTo(x+w/2, y+h)
			dc.LineTo(x, y+h/2)
			dc.ClosePath()
		}
	}
	fc, err := e.fill()
	if err != nil {
		return err
	}
	if fc != nil {
		path()
		dc.SetColor(fc)
		dc.Fill()
	}
	if err := r.setStroke(e); err != nil {
		return err
	}
	path()
	dc.Stroke()
	return nil
}

func (r *renderer) linePath(e *element) {
	dc := r.dc
	for i, p := range e.Points {
		if i == 0 {
			dc.MoveTo(e.X+p[0], e.Y+p[1])
		} else {
			dc.LineTo(e.X+p[0], e.Y+p[1])
		}
	}
}

func (r *renderer) drawLine(e *element) error {
	if len(e.Points) < 2 {
		return r.drawUnsupported(e, e.X, e.Y, e.Width, e.Height)
	}
	if err := r.setStroke(e); err != nil {
		return err
	}
	r.linePath(e)
	r.dc.Stroke()
	if e.Type != "arrow" {
		return nil
	}
	n := len(e.Points)
	if e.EndArrowhead != nil {
		r.arrowhead(e, *e.EndArrowhead, e.Points[n-1], e.Points[n-2])
	}
	if e.StartArrowhead != nil {
		r.arrowhead(e, *e.StartArrowhead, e.Points[0], e.Points[1])
	}
	return nil
}

// arrowhead draws a head at tip, pointing away from prev.
func (r *renderer) arrowhead(e *element, kind string, tip, prev [2]float64) {
	dc := r.dc
	sw := e.strokeWidth()
	tx, ty := e.X+tip[0], e.Y+tip[1]
	dx, dy := tip[0]-prev[0], tip[1]-prev[1]
	d := math.Hypot(dx, dy)
	if d == 0 {
		return
	}
	dx, dy = dx/d, dy/d
	size := math.Min(14+sw*2, d)
	if size < 6 {
		size = math.Min(6, d)
	}
	bx, by := tx-dx*size, ty-dy*size // base point of head
	px, py := -dy, dx
	dc.SetDash()
	dc.SetLineCap(gg.LineCapRound)
	half := size * 0.4
	switch kind {
	case "bar":
		dc.MoveTo(tx+px*half*1.2, ty+py*half*1.2)
		dc.LineTo(tx-px*half*1.2, ty-py*half*1.2)
		dc.Stroke()
	case "dot", "circle", "circle_outline":
		rad := size * 0.4
		dc.DrawCircle(tx-dx*rad, ty-dy*rad, rad)
		if kind == "circle_outline" {
			dc.Stroke()
		} else {
			dc.Fill()
		}
	case "triangle", "triangle_outline":
		dc.MoveTo(tx, ty)
		dc.LineTo(bx+px*half, by+py*half)
		dc.LineTo(bx-px*half, by-py*half)
		dc.ClosePath()
		if kind == "triangle_outline" {
			dc.Stroke()
		} else {
			dc.Fill()
		}
	case "diamond", "diamond_outline":
		mx, my := tx-dx*size/2, ty-dy*size/2
		dc.MoveTo(tx, ty)
		dc.LineTo(mx+px*half, my+py*half)
		dc.LineTo(bx, by)
		dc.LineTo(mx-px*half, my-py*half)
		dc.ClosePath()
		if kind == "diamond_outline" {
			dc.Stroke()
		} else {
			dc.Fill()
		}
	default: // "arrow" and any head type not drawn specially
		ang := 25 * math.Pi / 180
		for _, s := range []float64{ang, -ang} {
			sn, cs := math.Sincos(s)
			hx, hy := dx*cs-dy*sn, dx*sn+dy*cs
			dc.MoveTo(tx, ty)
			dc.LineTo(tx-hx*size, ty-hy*size)
			dc.Stroke()
		}
	}
}

func (r *renderer) drawFreedraw(e *element) error {
	if err := r.setStroke(e); err != nil {
		return err
	}
	dc := r.dc
	p := e.Points
	switch {
	case len(p) == 0:
		return nil
	case len(p) == 1:
		c, _ := e.stroke()
		dc.SetColor(c)
		dc.DrawCircle(e.X+p[0][0], e.Y+p[0][1], e.strokeWidth()/2)
		dc.Fill()
		return nil
	}
	pt := func(i int) (float64, float64) { return e.X + p[i][0], e.Y + p[i][1] }
	x, y := pt(0)
	dc.MoveTo(x, y)
	for i := 1; i < len(p)-1; i++ {
		cx, cy := pt(i)
		nx, ny := pt(i + 1)
		dc.QuadraticTo(cx, cy, (cx+nx)/2, (cy+ny)/2)
	}
	lx, ly := pt(len(p) - 1)
	dc.LineTo(lx, ly)
	dc.Stroke()
	return nil
}

func (r *renderer) drawImage(e *element, x, y, w, h float64) error {
	img := r.files[""]
	if e.FileID != nil {
		img = r.files[*e.FileID]
	}
	if img == nil || w <= 0 || h <= 0 {
		return r.placeholder(e, x, y, w, h, "image")
	}
	dc := r.dc
	b := img.Bounds()
	dc.Push()
	dc.Translate(x, y)
	dc.Scale(w/float64(b.Dx()), h/float64(b.Dy()))
	dc.DrawImage(img, 0, 0)
	dc.Pop()
	return nil
}

func (r *renderer) drawFrame(e *element, x, y, w, h float64) error {
	dc := r.dc
	dc.SetColor(color.NRGBA{0xbb, 0xbb, 0xbb, 255})
	dc.SetLineWidth(1)
	dc.SetDash()
	dc.DrawRectangle(x, y, w, h)
	dc.Stroke()
	name := "Frame"
	if e.Name != nil && *e.Name != "" {
		name = *e.Name
	}
	r.label(x, y-6, name, 14, color.NRGBA{0x88, 0x88, 0x88, 255})
	return nil
}

func (r *renderer) drawUnsupported(e *element, x, y, w, h float64) error {
	return r.placeholder(e, x, y, w, h, e.Type)
}

func (r *renderer) placeholder(e *element, x, y, w, h float64, label string) error {
	dc := r.dc
	col := color.NRGBA{0x99, 0x99, 0x99, 255}
	dc.SetColor(color.NRGBA{0xee, 0xee, 0xee, 255})
	dc.DrawRectangle(x, y, w, h)
	dc.Fill()
	dc.SetColor(col)
	dc.SetLineWidth(1.5)
	dc.SetDash(6, 4)
	dc.DrawRectangle(x, y, w, h)
	dc.Stroke()
	dc.SetDash()
	r.label(x+6, y+16, label, 14, color.NRGBA{0x55, 0x55, 0x55, 255})
	return nil
}

// label draws one line of text with its baseline-left at scene point (x, y),
// ignoring the current rotation.
func (r *renderer) label(x, y float64, s string, size float64, c color.Color) {
	dc := r.dc
	dx, dy := dc.TransformPoint(x, y)
	dc.Push()
	dc.Identity()
	dc.SetFontFace(face(1, size*r.s))
	dc.SetColor(c)
	dc.DrawString(s, dx, dy)
	dc.Pop()
}
