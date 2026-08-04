package pure_utils

import (
	"bufio"
	"io"
)

const (
	bom0 = 0xef
	bom1 = 0xbb
	bom2 = 0xbf
)

func NewReaderWithoutBom(r io.Reader) io.Reader {
	reader, _ := TrimBom(r)
	return reader
}

// TrimBom behaves like NewReaderWithoutBom and additionally reports how many bytes it discarded
// (0 or 3). Callers that track absolute byte offsets into the file need that count: the returned
// reader starts after the BOM, so offsets measured on it are short by exactly this many bytes.
func TrimBom(r io.Reader) (io.Reader, int64) {
	buf := bufio.NewReader(r)
	b, err := buf.Peek(3)
	if err != nil {
		// not enough bytes
		return buf, 0
	}
	if b[0] == bom0 && b[1] == bom1 && b[2] == bom2 {
		_, _ = buf.Discard(3)
		return buf, 3
	}
	return buf, 0
}
