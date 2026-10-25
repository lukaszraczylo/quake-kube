package client

import (
	"context"
	"net"
	"net/http"
	"syscall"
	"time"

	"github.com/gorilla/websocket"
)

type Server struct {
	Addr       string
	Handler    http.Handler
	ServerAddr string
}

// Serve answers WebSocket upgrades with the UDP proxy and everything else with Handler.
func (s *Server) Serve(l net.Listener) error {
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

	srv := &http.Server{
		Addr: s.Addr,
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if websocket.IsWebSocketUpgrade(r) {
				wsproxy.ServeHTTP(w, r)
				return
			}
			s.Handler.ServeHTTP(w, r)
		}),
		// net/http clears these deadlines when a connection is hijacked, so they
		// bound plain HTTP only; the proxy sets its own websocket deadlines.
		ReadTimeout:       2 * time.Minute,
		WriteTimeout:      2 * time.Minute,
		IdleTimeout:       3 * time.Minute,
		MaxHeaderBytes:    1 << 16,
		ReadHeaderTimeout: 5 * time.Second,
	}
	return srv.Serve(l)
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
