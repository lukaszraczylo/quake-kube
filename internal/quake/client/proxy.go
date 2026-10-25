package client

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"sync"
	"syscall"
	"time"

	"github.com/gorilla/websocket"
)

// Optimized buffer sizes for gaming traffic patterns
const (
	// readTimeout bounds silence from a websocket peer; pings go out every 15s.
	readTimeout = 45 * time.Second

	// Quake 3 max packet size is ~1400 bytes, use 2KB for safety
	bufferSize = 2048

	// Number of buffers to preallocate at startup
	preallocBufferCount = 128
)

// buffer wraps a byte slice for pool usage (pointer type to avoid allocations)
type buffer struct {
	data []byte
}

// Pre-warm the buffer pool with buffers to eliminate allocation spikes during gameplay
var bufferPool = sync.Pool{
	New: func() interface{} {
		return &buffer{data: make([]byte, bufferSize)}
	},
}

// Initialize buffer pool with pre-allocated buffers
func init() {
	// Pre-allocate buffers to reduce GC pressure during high-traffic periods
	for i := 0; i < preallocBufferCount; i++ {
		bufferPool.Put(&buffer{data: make([]byte, bufferSize)})
	}
}

// Optimized WebSocket upgrader with performance-focused configuration
var DefaultUpgrader = &websocket.Upgrader{
	ReadBufferSize:  4096, // 4KB - sufficient for Quake packets
	WriteBufferSize: 4096, // 4KB - sufficient for Quake packets
	CheckOrigin: func(r *http.Request) bool {
		return true // Required for cross-origin support
	},
	// Disable compression for lower CPU usage and better latency
	EnableCompression: false,
}

type WebsocketUDPProxy struct {
	Upgrader *websocket.Upgrader

	addr net.Addr
}

func NewProxy(addr string) (*WebsocketUDPProxy, error) {
	raddr, err := net.ResolveUDPAddr("udp", addr)
	if err != nil {
		return nil, err
	}
	return &WebsocketUDPProxy{addr: raddr}, nil
}

// ServeHTTP handles websocket proxying optimized for maximum performance and minimum latency
func (w *WebsocketUDPProxy) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	// Context with cancellation for proper cleanup
	ctx, cancel := context.WithCancel(req.Context())
	defer cancel()

	// Get appropriate upgrader with fallback
	upgrader := w.Upgrader
	if w.Upgrader == nil {
		upgrader = DefaultUpgrader
	}

	// Preserve protocol headers for protocol compliance
	upgradeHeader := http.Header{}
	if hdr := req.Header.Get("Sec-Websocket-Protocol"); hdr != "" {
		upgradeHeader.Set("Sec-Websocket-Protocol", hdr)
	}

	// Add performance-oriented headers
	upgradeHeader.Set("X-Content-Type-Options", "nosniff") // Prevent MIME sniffing

	// Upgrade with error handling
	ws, err := upgrader.Upgrade(rw, req, upgradeHeader)
	if err != nil {
		log.Printf("wsproxy: couldn't upgrade %v", err)
		return
	}
	defer ws.Close()

	// Set optimal websocket options for gaming traffic
	ws.SetReadLimit(65536) // 64KB max message size for security

	// Dead peers must not pin a goroutine pair and a UDP socket forever: any
	// message or pong pushes the deadline out, silence for readTimeout ends it.
	ws.SetPongHandler(func(string) error { return ws.SetReadDeadline(time.Now().Add(readTimeout)) })

	// Configure WS to not use compression for binary game data
	ws.EnableWriteCompression(false)

	// Create UDP backend with optimized buffer sizes
	udpConfig := net.ListenConfig{
		// Set control function for socket options
		Control: func(network, address string, c syscall.RawConn) error {
			return c.Control(func(fd uintptr) {
				// Increase UDP receive buffer for performance
				// Note: Values might need OS-specific tuning
				syscall.SetsockoptInt(int(fd), syscall.SOL_SOCKET, syscall.SO_RCVBUF, 4*1024*1024) // 4MB buffer
				syscall.SetsockoptInt(int(fd), syscall.SOL_SOCKET, syscall.SO_SNDBUF, 4*1024*1024) // 4MB buffer
			})
		},
	}

	// Create high-performance UDP listener
	network, laddr := w.localSocket()
	backend, err := udpConfig.ListenPacket(ctx, network, laddr)
	if err != nil {
		log.Printf("wsproxy: couldn't create UDP listener: %v", err)
		return
	}
	defer backend.Close()

	// Buffered error channel to prevent blocking
	errc := make(chan error, 2)

	// Signal for graceful shutdown
	done := make(chan struct{})
	defer close(done)

	// Cached values for better performance
	portPrefix := []byte("\xff\xff\xff\xffport")

	if err := ws.SetReadDeadline(time.Now().Add(readTimeout)); err != nil {
		return
	}

	// Enable keepalive for web socket connection
	go func() {
		ticker := time.NewTicker(15 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				// Send ping to keep connection alive
				if err := ws.WriteControl(websocket.PingMessage, []byte{}, time.Now().Add(time.Second)); err != nil {
					return
				}
			case <-done:
				return
			}
		}
	}()

	// WS to UDP direction - optimized for gaming traffic
	go func() {
		// Defer recovery from panics
		defer func() {
			if r := recover(); r != nil {
				errc <- fmt.Errorf("panic in ws reader: %v", r)
			}
		}()

		for {
			msg, buf, err := readPacket(ws)
			if err != nil {
				errc <- err
				return
			}
			if derr := ws.SetReadDeadline(time.Now().Add(readTimeout)); derr != nil {
				bufferPool.Put(buf)
				errc <- derr
				return
			}

			// Fast prefix check for special messages using direct comparison
			if len(msg) >= 8 && bytes.Equal(msg[:8], portPrefix) {
				bufferPool.Put(buf)
				continue
			}

			// Short write deadline for better throughput
			if err := backend.SetWriteDeadline(time.Now().Add(2 * time.Second)); err != nil {
				bufferPool.Put(buf)
				errc <- err
				return
			}

			_, err = backend.WriteTo(msg, w.addr)
			bufferPool.Put(buf)
			if err != nil {
				errc <- err
				return
			}
		}
	}()

	// UDP to WS direction - optimized for maximum throughput
	go func() {
		// Defer recovery from panics
		defer func() {
			if r := recover(); r != nil {
				errc <- fmt.Errorf("panic in udp reader: %v", r)
			}
		}()

		for {
			// Get buffer from pool for zero-allocation reading
			buf := bufferPool.Get().(*buffer)

			n, _, err := backend.ReadFrom(buf.data)
			if err != nil {
				bufferPool.Put(buf)
				errc <- err
				return
			}

			// Write directly to websocket from the buffer slice
			if err := ws.WriteMessage(websocket.BinaryMessage, buf.data[:n]); err != nil {
				bufferPool.Put(buf)
				errc <- err
				return
			}

			// Return buffer to pool
			bufferPool.Put(buf)
		}
	}()

	// Wait for any goroutine to signal completion
	select {
	case err := <-errc:
		closeMsg := websocket.FormatCloseMessage(websocket.CloseNormalClosure, "")
		var ce *websocket.CloseError
		if errors.As(err, &ce) {
			if ce.Code != websocket.CloseNoStatusReceived {
				closeMsg = websocket.FormatCloseMessage(ce.Code, ce.Text)
			}
			if ce.Code == websocket.CloseAbnormalClosure {
				log.Printf("wsproxy: %v", err)
			}
		} else {
			log.Printf("wsproxy: %v", err)
		}
		// WriteControl is the only writer call safe beside the UDP->WS writer.
		_ = ws.WriteControl(websocket.CloseMessage, closeMsg, time.Now().Add(time.Second))
	case <-ctx.Done():
		// Context canceled (e.g., client disconnected)
		return
	}
}

// readPacket reads one websocket message into a pooled buffer. The caller
// must return buf to bufferPool once msg is no longer used.
func readPacket(ws *websocket.Conn) (msg []byte, buf *buffer, err error) {
	_, r, err := ws.NextReader()
	if err != nil {
		return nil, nil, err
	}
	buf = bufferPool.Get().(*buffer)
	n, err := io.ReadFull(r, buf.data)
	switch err {
	case io.EOF, io.ErrUnexpectedEOF:
		return buf.data[:n], buf, nil
	case nil:
		// Message at least as large as the pooled buffer: take the rest.
		rest, rerr := io.ReadAll(r)
		if rerr != nil {
			bufferPool.Put(buf)
			return nil, nil, rerr
		}
		return append(buf.data[:n:n], rest...), buf, nil
	default:
		bufferPool.Put(buf)
		return nil, nil, err
	}
}

// localSocket returns a UDP socket family matching the backend. A dual-stack
// "[::]" socket sending to an IPv4 target goes through v4-mapped addressing,
// which dropped packets under load in tests.
func (w *WebsocketUDPProxy) localSocket() (network, laddr string) {
	if ua, ok := w.addr.(*net.UDPAddr); ok && ua.IP.To4() == nil && ua.IP != nil {
		return "udp6", "[::]:0"
	}
	return "udp4", "0.0.0.0:0"
}
