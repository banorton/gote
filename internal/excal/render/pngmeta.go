package render

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"hash/crc32"
	"io"
	"strings"
)

// Version invalidates every cached render when the renderer's output changes.
const Version = "1"

const textKey = "gote-render"

var pngSig = []byte("\x89PNG\r\n\x1a\n")

// Hash identifies the render of a drawing file: its bytes plus Version.
func Hash(src []byte) string {
	h := sha256.New()
	h.Write([]byte("gote-render-v" + Version + "\x00"))
	h.Write(src)
	return hex.EncodeToString(h.Sum(nil))[:32]
}

func chunk(typ string, data []byte) []byte {
	var b bytes.Buffer
	binary.Write(&b, binary.BigEndian, uint32(len(data)))
	b.WriteString(typ)
	b.Write(data)
	binary.Write(&b, binary.BigEndian, crc32.ChecksumIEEE(append([]byte(typ), data...)))
	return b.Bytes()
}

// stamp inserts a tEXt chunk holding hash right after IHDR.
func stamp(pngData []byte, hash string) []byte {
	const ihdrEnd = 8 + 4 + 4 + 13 + 4
	out := make([]byte, 0, len(pngData)+64)
	out = append(out, pngData[:ihdrEnd]...)
	out = append(out, chunk("tEXt", []byte(textKey+"\x00"+hash))...)
	return append(out, pngData[ihdrEnd:]...)
}

// Info reads the render hash and pixel size from the head of a PNG written by PNG.
func Info(r io.Reader) (hash string, w, h int, err error) {
	sig := make([]byte, 8)
	if _, err = io.ReadFull(r, sig); err != nil || !bytes.Equal(sig, pngSig) {
		return "", 0, 0, fmt.Errorf("not a PNG")
	}
	for i := 0; i < 2; i++ {
		var hdr [8]byte
		if _, err = io.ReadFull(r, hdr[:]); err != nil {
			return "", 0, 0, fmt.Errorf("truncated PNG: %w", err)
		}
		n := binary.BigEndian.Uint32(hdr[:4])
		if n > 1<<16 {
			return "", 0, 0, fmt.Errorf("unexpected PNG chunk size %d", n)
		}
		body := make([]byte, n+4)
		if _, err = io.ReadFull(r, body); err != nil {
			return "", 0, 0, fmt.Errorf("truncated PNG: %w", err)
		}
		switch string(hdr[4:]) {
		case "IHDR":
			w, h = int(binary.BigEndian.Uint32(body[0:4])), int(binary.BigEndian.Uint32(body[4:8]))
		case "tEXt":
			k, v, _ := strings.Cut(string(body[:n]), "\x00")
			if k != textKey {
				return "", 0, 0, fmt.Errorf("PNG has no %s stamp", textKey)
			}
			return v, w, h, nil
		default:
			return "", 0, 0, fmt.Errorf("PNG has no %s stamp", textKey)
		}
	}
	return "", 0, 0, fmt.Errorf("PNG has no %s stamp", textKey)
}
