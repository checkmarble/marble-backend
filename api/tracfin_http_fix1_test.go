package api

import (
	"bufio"
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTracfinCORSConfiguration(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg, enabled := buildCorsOptions(context.Background(), Configuration{MarbleBackofficeUrl: "https://backoffice.example/path"})
	require.True(t, enabled)
	router := gin.New()
	router.Use(cors.New(cfg))
	router.GET("/download", func(c *gin.Context) { c.Status(http.StatusOK) })
	request := httptest.NewRequest(http.MethodOptions, "/download", nil)
	request.Header.Set("Origin", "https://backoffice.example")
	request.Header.Set("Access-Control-Request-Method", http.MethodPut)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	require.Equal(t, "https://backoffice.example", response.Header().Get("Access-Control-Allow-Origin"))
	methods := response.Header().Get("Access-Control-Allow-Methods")
	for _, method := range []string{"OPTIONS", "HEAD", "GET", "POST", "DELETE", "PATCH", "PUT"} {
		assert.Contains(t, methods, method)
	}
	require.Empty(t, response.Header().Get("Access-Control-Allow-Credentials"))
	get := httptest.NewRequest(http.MethodGet, "/download", nil)
	get.Header.Set("Origin", "https://backoffice.example")
	response = httptest.NewRecorder()
	router.ServeHTTP(response, get)
	exposed := http.CanonicalHeaderKey(response.Header().Get("Access-Control-Expose-Headers"))
	assert.Contains(t, strings.ToLower(exposed), "x-tracfin-draft-revision")
	assert.Contains(t, strings.ToLower(exposed), "content-disposition")
	require.NotContains(t, exposed, "*")
	denied := httptest.NewRequest(http.MethodOptions, "/download", nil)
	denied.Header.Set("Origin", "https://evil.example")
	denied.Header.Set("Access-Control-Request-Method", http.MethodPut)
	response = httptest.NewRecorder()
	router.ServeHTTP(response, denied)
	require.Empty(t, response.Header().Get("Access-Control-Allow-Origin"))
	require.Empty(t, response.Header().Get("Access-Control-Allow-Credentials"))
}

type fix1Reader struct {
	*strings.Reader
	fail   bool
	closed bool
}

func (r *fix1Reader) Read(p []byte) (int, error) {
	n, err := r.Reader.Read(p)
	if err == io.EOF && r.fail {
		return 0, errors.New("source failed")
	}
	return n, err
}
func (r *fix1Reader) Close() error { r.closed = true; return nil }

type fix1Usecase struct {
	*tracfinHTTPFake
	reader *fix1Reader
	size   int64
}

func (f *fix1Usecase) DownloadExport(context.Context, string, string) (models.Blob, models.TracfinExport, error) {
	return models.Blob{ReadCloser: f.reader}, models.TracfinExport{Size: f.size}, nil
}

type fix1Listener struct {
	conn net.Conn
	done chan struct{}
	once sync.Once
}

func (l *fix1Listener) Accept() (net.Conn, error) {
	if l.conn != nil {
		c := l.conn
		l.conn = nil
		return c, nil
	}
	<-l.done
	return nil, net.ErrClosed
}
func (l *fix1Listener) Close() error   { l.once.Do(func() { close(l.done) }); return nil }
func (l *fix1Listener) Addr() net.Addr { return fix1Addr{} }

type fix1Addr struct{}

func (fix1Addr) Network() string { return "pipe" }
func (fix1Addr) String() string  { return "pipe" }
func TestTracfinHTTPDownloadClientFraming(t *testing.T) {
	for _, tc := range []struct {
		name      string
		fail      bool
		size      int64
		wantError bool
	}{{"truncated", true, 20, true}, {"complete", false, 11, false}} {
		t.Run(tc.name, func(t *testing.T) {
			gin.SetMode(gin.TestMode)
			client, server := net.Pipe()
			deadline := time.Now().Add(5 * time.Second)
			require.NoError(t, client.SetDeadline(deadline))
			require.NoError(t, server.SetDeadline(deadline))
			defer client.Close()
			defer server.Close()
			listener := &fix1Listener{conn: server, done: make(chan struct{})}
			reader := &fix1Reader{Reader: strings.NewReader("<ds>partial"), fail: tc.fail}
			fake := &fix1Usecase{tracfinHTTPFake: &tracfinHTTPFake{}, reader: reader, size: tc.size}
			router := gin.New()
			addTracfinRoutes(router, func(context.Context) tracfinHTTPUsecase { return fake })
			srv := &http.Server{Handler: router}
			finished := make(chan struct{})
			go func() { _ = srv.Serve(listener); close(finished) }()
			defer func() { _ = srv.Close(); _ = listener.Close(); <-finished }()
			req := httptest.NewRequest(http.MethodGet, "http://example/cases/"+testCaseID+"/tracfin/exports/"+testExportID, nil)
			req.Close = true
			require.NoError(t, req.Write(client))
			res, err := http.ReadResponse(bufio.NewReader(client), req)
			require.NoError(t, err)
			defer res.Body.Close()
			body, readErr := io.ReadAll(res.Body)
			assert.Equal(t, tc.size, res.ContentLength)
			require.Equal(t, "<ds>partial", string(body))
			if tc.wantError {
				assert.Error(t, readErr)
			} else {
				require.NoError(t, readErr)
			}
			require.True(t, reader.closed)
		})
	}
}
