package render

import (
	"bytes"
	"flag"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

var update = flag.Bool("update", false, "rewrite golden PNGs")

func fixture(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", name+".excalidraw"))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// diffRatio returns the fraction of pixels differing by more than 24 in any channel.
func diffRatio(a, b image.Image) float64 {
	n, bad := 0, 0
	r := a.Bounds()
	for y := r.Min.Y; y < r.Max.Y; y++ {
		for x := r.Min.X; x < r.Max.X; x++ {
			n++
			ar, ag, ab, aa := a.At(x, y).RGBA()
			br, bg, bb, ba := b.At(x, y).RGBA()
			for _, d := range []int{int(ar>>8) - int(br>>8), int(ag>>8) - int(bg>>8), int(ab>>8) - int(bb>>8), int(aa>>8) - int(ba>>8)} {
				if d > 24 || d < -24 {
					bad++
					break
				}
			}
		}
	}
	return float64(bad) / float64(n)
}

func TestGolden(t *testing.T) {
	for _, name := range []string{"shapes", "text", "lines", "image", "misc", "empty"} {
		t.Run(name, func(t *testing.T) {
			data, _, _, err := PNG(fixture(t, name))
			if err != nil {
				t.Fatal(err)
			}
			golden := filepath.Join("testdata", name+".golden.png")
			if *update {
				if err := os.WriteFile(golden, data, 0o644); err != nil {
					t.Fatal(err)
				}
				return
			}
			gb, err := os.ReadFile(golden)
			if err != nil {
				t.Fatalf("%v (run with -update)", err)
			}
			got, err := png.Decode(bytes.NewReader(data))
			if err != nil {
				t.Fatal(err)
			}
			want, err := png.Decode(bytes.NewReader(gb))
			if err != nil {
				t.Fatal(err)
			}
			if got.Bounds() != want.Bounds() {
				t.Fatalf("size %v, golden %v", got.Bounds(), want.Bounds())
			}
			if r := diffRatio(got, want); r > 0.002 {
				t.Fatalf("%.2f%% of pixels differ from %s", r*100, golden)
			}
		})
	}
}

func TestDeterministic(t *testing.T) {
	for _, name := range []string{"shapes", "text", "lines", "image"} {
		src := fixture(t, name)
		a, _, _, err := PNG(src)
		if err != nil {
			t.Fatal(err)
		}
		b, _, _, _ := PNG(src)
		if !bytes.Equal(a, b) {
			t.Errorf("%s: output differs between runs", name)
		}
	}
}

func TestEmptyDrawingIsSmallBlankCanvas(t *testing.T) {
	data, w, h, err := PNG(fixture(t, "empty"))
	if err != nil {
		t.Fatal(err)
	}
	if w < 16 || w > 256 || h < 16 || h > 256 {
		t.Fatalf("size %dx%d", w, h)
	}
	img, _ := png.Decode(bytes.NewReader(data))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			if c := color.NRGBAModel.Convert(img.At(x, y)).(color.NRGBA); c != (color.NRGBA{255, 255, 255, 255}) {
				t.Fatalf("pixel %d,%d = %v", x, y, c)
			}
		}
	}
}

func TestDeletedElementsAreSkipped(t *testing.T) {
	with := `{"type":"excalidraw","elements":[
	 {"id":"a","type":"rectangle","x":0,"y":0,"width":50,"height":50,"strokeColor":"#000000"},
	 {"id":"b","type":"rectangle","x":500,"y":500,"width":50,"height":50,"isDeleted":true}]}`
	without := `{"type":"excalidraw","elements":[{"id":"a","type":"rectangle","x":0,"y":0,"width":50,"height":50,"strokeColor":"#000000"}]}`
	a, _, _, err := PNG([]byte(with))
	if err != nil {
		t.Fatal(err)
	}
	b, _, _, _ := PNG([]byte(without))
	ia, _ := png.Decode(bytes.NewReader(a))
	ib, _ := png.Decode(bytes.NewReader(b))
	if ia.Bounds() != ib.Bounds() || diffRatio(ia, ib) != 0 {
		t.Fatal("deleted element changed the render")
	}
}

func TestHugeDrawingIsCapped(t *testing.T) {
	src := `{"type":"excalidraw","elements":[
	 {"id":"a","type":"rectangle","x":0,"y":0,"width":50,"height":50},
	 {"id":"b","type":"rectangle","x":40000,"y":3000,"width":50,"height":50}]}`
	_, w, h, err := PNG([]byte(src))
	if err != nil {
		t.Fatal(err)
	}
	if max(w, h) > 2000 || max(w, h) < 1990 {
		t.Fatalf("size %dx%d", w, h)
	}
}

func TestBackgroundColorFromAppState(t *testing.T) {
	data, _, _, err := PNG(fixture(t, "misc"))
	if err != nil {
		t.Fatal(err)
	}
	img, _ := png.Decode(bytes.NewReader(data))
	if c := color.NRGBAModel.Convert(img.At(1, 1)).(color.NRGBA); c != (color.NRGBA{0xf8, 0xf9, 0xfa, 255}) {
		t.Fatalf("corner %v", c)
	}
}

func TestBoundTextLandsInsideContainer(t *testing.T) {
	src := `{"type":"excalidraw","elements":[
	 {"id":"r","type":"rectangle","x":0,"y":0,"width":300,"height":200,"strokeColor":"#ffffff","strokeWidth":1},
	 {"id":"t","type":"text","x":900,"y":900,"width":10,"height":10,"text":"hello","fontSize":20,"fontFamily":2,"strokeColor":"#000000","containerId":"r","textAlign":"center","verticalAlign":"middle"}]}`
	data, w, h, err := PNG([]byte(src))
	if err != nil {
		t.Fatal(err)
	}
	img, _ := png.Decode(bytes.NewReader(data))
	if w > 400 || h > 300 {
		t.Fatalf("canvas %dx%d: text was not moved into its container", w, h)
	}
	minX, minY, maxX, maxY := w, h, 0, 0
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			if r, _, _, _ := img.At(x, y).RGBA(); r < 0x4000 {
				minX, minY, maxX, maxY = min(minX, x), min(minY, y), max(maxX, x), max(maxY, y)
			}
		}
	}
	cx, cy := (minX+maxX)/2-32, (minY+maxY)/2-32
	if maxX < minX || abs(cx-150) > 8 || abs(cy-100) > 8 {
		t.Fatalf("text center (%d,%d) bbox %d,%d-%d,%d, want about (150,100)", cx, cy, minX, minY, maxX, maxY)
	}
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

func TestInvalidInput(t *testing.T) {
	for src, want := range map[string]string{
		`not json`:                       "invalid excalidraw",
		`{"type":"other","elements":[]}`: "not an excalidraw",
		`{"type":"excalidraw","elements":[{"id":"x","type":"rectangle","width":5,"height":5,"strokeColor":"nope"}]}`: `"x"`,
	} {
		_, _, _, err := PNG([]byte(src))
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%q: err %v, want %q", src, err, want)
		}
	}
}

func TestHashAndInfoRoundTrip(t *testing.T) {
	src := fixture(t, "shapes")
	data, w, h, err := PNG(src)
	if err != nil {
		t.Fatal(err)
	}
	hash, iw, ih, err := Info(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	if hash != Hash(src) || iw != w || ih != h {
		t.Fatalf("info %s %dx%d, want %s %dx%d", hash, iw, ih, Hash(src), w, h)
	}
	if Hash(src) == Hash(append(append([]byte{}, src...), ' ')) {
		t.Fatal("hash ignores content")
	}
	if _, err := png.Decode(bytes.NewReader(data)); err != nil {
		t.Fatalf("PNG with hash chunk must stay decodable: %v", err)
	}
	if _, _, _, err := Info(strings.NewReader("garbage")); err == nil {
		t.Fatal("want error on non-PNG")
	}
}
