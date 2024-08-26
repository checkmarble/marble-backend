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
	buf := bufio.NewReader(r)
	b, err := buf.Peek(3)
	if err != nil {
		// not enough bytes
		return buf
	}
	if b[0] == bom0 && b[1] == bom1 && b[2] == bom2 {
		_, _ = buf.Discard(3)
	}
	return buf
}
