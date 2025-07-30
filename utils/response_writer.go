package utils

import (
	"bytes"

	"github.com/gin-gonic/gin"
)

// BufferResponseWriter can be used when we need to buffer the HTTP response
// that would be sent to the client. This can be used if a middleware needs to
// perform check **after** the handler has executed and modify the response
// accordingly.
type BufferResponseWriter struct {
	gin.ResponseWriter

	Body           *bytes.Buffer
	originalWriter gin.ResponseWriter
}

func NewBufferResponseWriter(c *gin.Context) *BufferResponseWriter {
	buf := &BufferResponseWriter{
		ResponseWriter: c.Writer,
		Body:           &bytes.Buffer{},
		originalWriter: c.Writer,
	}

	c.Writer = buf

	return buf
}

func (wr *BufferResponseWriter) Write(b []byte) (int, error) {
	return wr.Body.Write(b)
}

func (wr *BufferResponseWriter) Restore(c *gin.Context) {
	c.Writer = wr.originalWriter

	_, _ = c.Writer.Write(wr.Body.Bytes())
}
