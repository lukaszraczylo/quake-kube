package client

import (
	"context"
	"net"
	"net/http"
	"syscall"
	"time"

	"github.com/cockroachdb/cmux"
)

type Server struct {
	Addr       string
	Handler    http.Handler
	ServerAddr string
}

func (s *Server) Serve(l net.Listener) error {
	// Configure multiplexer with optimized buffer size
	m := cmux.New(l)

	// Optimize for websocket connections with higher priority
	websocketL := m.Match(cmux.HTTP1HeaderField("Upgrade", "websocket"))
	httpL := m.Match(cmux.Any()) // HTTP fallback

	// Serve regular HTTP traffic
	go func() {
		httpServer := &http.Server{
			Addr:    s.Addr,
			Handler: s.Handler,
			// Optimized timeout settings for better connection handling
			ReadTimeout:    2 * time.Minute, // Shorter for better resource usage
			WriteTimeout:   2 * time.Minute, // Shorter for better resource usage
			IdleTimeout:    3 * time.Minute, // Added idle timeout for connection reuse
			MaxHeaderBytes: 1 << 16,         // 64KB is sufficient and more efficient
			// Additional performance tuning
			ReadHeaderTimeout: 5 * time.Second, // Protect against slow clients
		}
		if err := httpServer.Serve(httpL); err != cmux.ErrListenerClosed {
			panic(err)
		}
	}()

	host, port, err := net.SplitHostPort(s.ServerAddr)
	if err != nil {
		return err
	}
	proxyTarget := s.ServerAddr
	if net.ParseIP(host).IsUnspecified() {
		// handle case where host is 0.0.0.0
		proxyTarget = net.JoinHostPort("127.0.0.1", port)
	}
	wsproxy, err := NewProxy(proxyTarget)
	if err != nil {
		return err
	}

	go func() {
		// Create optimized server for WebSocket traffic
		wsServer := &http.Server{
			Handler: wsproxy,
			// WebSocket servers need longer timeouts
			ReadTimeout:       5 * time.Minute,
			WriteTimeout:      5 * time.Minute,
			IdleTimeout:       10 * time.Minute, // WebSockets use long-lived connections
			ReadHeaderTimeout: 10 * time.Second, // Faster header processing for WS upgrade
		}

		if err := wsServer.Serve(websocketL); err != cmux.ErrListenerClosed {
			panic(err)
		}
	}()

	return m.Serve()
}

func (s *Server) ListenAndServe() error {
	// Create TCP listener with optimized configuration
	config := net.ListenConfig{
		// Set keep alive and other socket options
		Control: func(network, address string, c syscall.RawConn) error {
			var operr error
			if err := c.Control(func(fd uintptr) {
				// Set TCP options for better performance
				operr = syscall.SetsockoptInt(int(fd), syscall.SOL_SOCKET, syscall.SO_REUSEADDR, 1)
				if operr != nil {
					return
				}

				// Enable TCP keep alive
				operr = syscall.SetsockoptInt(int(fd), syscall.SOL_SOCKET, syscall.SO_KEEPALIVE, 1)
				if operr != nil {
					return
				}

				// Increase socket buffer sizes for better throughput
				operr = syscall.SetsockoptInt(int(fd), syscall.SOL_SOCKET, syscall.SO_RCVBUF, 4*1024*1024) // 4MB
				if operr != nil {
					return
				}

				operr = syscall.SetsockoptInt(int(fd), syscall.SOL_SOCKET, syscall.SO_SNDBUF, 4*1024*1024) // 4MB
				if operr != nil {
					return
				}

				// Disable Nagle's algorithm for lower latency
				operr = syscall.SetsockoptInt(int(fd), syscall.IPPROTO_TCP, syscall.TCP_NODELAY, 1)
			}); err != nil {
				return err
			}
			return operr
		},
	}

	// Create optimized TCP listener
	l, err := config.Listen(context.Background(), "tcp", s.Addr)
	if err != nil {
		return err
	}

	return s.Serve(l)
}
