package infra

import (
	"context"
	"io"
	"net"
	"sync"

	"cloud.google.com/go/cloudsqlconn"
	"github.com/cockroachdb/errors"
)

type CloudSqlBridge struct {
	instance string
	dial     func(context.Context, string) (net.Conn, error)
	listener net.Listener
	addr     string

	cancel    context.CancelFunc
	wg        sync.WaitGroup
	closeOnce sync.Once
}

func StartCloudSqlBridge(ctx context.Context, dialer *cloudsqlconn.Dialer, instance string) (*CloudSqlBridge, error) {
	return startCloudSqlBridge(ctx, instance, func(ctx context.Context, instance string) (net.Conn, error) {
		return dialer.Dial(ctx, instance)
	})
}

func startCloudSqlBridge(
	ctx context.Context,
	instance string,
	dial func(context.Context, string) (net.Conn, error),
) (*CloudSqlBridge, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, errors.Wrap(err, "could not start Cloud SQL bridge")
	}

	ctx, cancel := context.WithCancel(ctx)

	bridge := &CloudSqlBridge{
		instance: instance,
		dial:     dial,
		listener: listener,
		addr:     listener.Addr().String(),
		cancel:   cancel,
	}

	context.AfterFunc(ctx, func() {
		_ = listener.Close()
	})

	bridge.wg.Add(1)

	go bridge.serve(ctx)

	return bridge, nil
}

func (br *CloudSqlBridge) Addr() string {
	if br == nil {
		return ""
	}
	return br.addr
}

func (br *CloudSqlBridge) serve(ctx context.Context) {
	defer br.wg.Done()

	for {
		src, err := br.listener.Accept()
		if err != nil {
			if errors.Is(err, net.ErrClosed) || ctx.Err() != nil {
				return
			}

			continue
		}

		br.wg.Add(1)

		go br.handleConnection(ctx, src)
	}
}

func (br *CloudSqlBridge) handleConnection(ctx context.Context, src net.Conn) {
	defer br.wg.Done()
	defer src.Close()

	dst, err := br.dial(ctx, br.instance)
	if err != nil {
		return
	}
	defer dst.Close()

	stopShutdown := context.AfterFunc(ctx, func() {
		_ = src.Close()
		_ = dst.Close()
	})
	defer stopShutdown()

	var wg sync.WaitGroup

	wg.Add(2)

	go func() {
		defer wg.Done()

		_, _ = io.Copy(dst, src)

		if conn, ok := dst.(*net.TCPConn); ok {
			_ = conn.CloseWrite()
		}
	}()

	go func() {
		defer wg.Done()
		_, _ = io.Copy(src, dst)

		if conn, ok := src.(*net.TCPConn); ok {
			_ = conn.CloseWrite()
		}
	}()

	wg.Wait()
}

func (br *CloudSqlBridge) Close() {
	if br == nil {
		return
	}

	br.closeOnce.Do(func() {
		br.cancel()
		br.listener.Close()
	})

	br.wg.Wait()
}
