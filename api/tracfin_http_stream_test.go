package api

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"context"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type tracfinFailingStream struct {
	recorder      *httptest.ResponseRecorder
	reads         int
	closed        bool
	sawFirstChunk bool
}

func (r *tracfinFailingStream) Read(p []byte) (int, error) {
	r.reads++
	if r.reads == 1 {
		return copy(p, "<ds>partial"), nil
	}
	r.sawFirstChunk = r.recorder.Body.String() == "<ds>partial"
	return 0, errors.New("stream failed")
}
func (r *tracfinFailingStream) Seek(int64, int) (int64, error) {
	return 0, errors.New("seek unsupported")
}
func (r *tracfinFailingStream) Close() error { r.closed = true; return nil }

var _ io.ReadSeekCloser = (*tracfinFailingStream)(nil)

func TestTracfinHTTPDownloadStreamingReadFailure(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	stream := &tracfinFailingStream{recorder: recorder}
	fake := &tracfinHTTPFake{reader: stream, size: int64(len("<ds>partial</ds>"))}
	router := gin.New()
	addTracfinRoutes(router, func(context.Context) tracfinHTTPUsecase { return fake })
	request := httptest.NewRequest(http.MethodGet, "/cases/"+testCaseID+"/tracfin/exports/"+testExportID, strings.NewReader(""))
	router.ServeHTTP(recorder, request)
	require.True(t, stream.closed)
	require.True(t, stream.sawFirstChunk, "first XML chunk should reach the client before the next read")
	require.Equal(t, "<ds>partial", recorder.Body.String(), "a stream error must not append a JSON error to XML already sent")
}
