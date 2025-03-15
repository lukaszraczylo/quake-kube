package client

import (
	"bytes"
	"context"
	"fmt"
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
	// Increased buffer size for better throughput with game packets
	bufferSize = 16384 // 16KB for better performance with larger packets
	
	// Number of buffers to preallocate at startup
	preallocBufferCount = 64
)

// Pre-warm the buffer pool with buffers to eliminate allocation spikes during gameplay
var bufferPool = sync.Pool{
	New: func() interface{} {
		return make([]byte, bufferSize)
	},
}

// Initialize buffer pool with pre-allocated buffers
func init() {
	// Pre-allocate buffers to reduce GC pressure during high-traffic periods
	for i := 0; i < preallocBufferCount; i++ {
		bufferPool.Put(make([]byte, bufferSize))
	}
}

// Optimized WebSocket upgrader with performance-focused configuration
var DefaultUpgrader = &websocket.Upgrader{
	ReadBufferSize:  32768,  // 32KB for better read performance
	WriteBufferSize: 32768,  // 32KB for better write performance
	CheckOrigin: func(r *http.Request) bool {
		return true // Required for cross-origin support
	},
	// Disable compression for lower CPU usage and better latency
	// Game packets are often already compressed or binary data
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
	ws.SetPongHandler(func(string) error { return nil }) // Fast no-op pong handler
	
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
	backend, err := udpConfig.ListenPacket(ctx, "udp", "0.0.0.0:0")
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
	
	// Disable read deadline for long-running connections
	if err := ws.SetReadDeadline(time.Time{}); err != nil {
		errc <- err
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
			// Most efficient message reading method
			_, msg, err := ws.ReadMessage()
			if err != nil {
				// Handle close with proper error code propagation
				m := websocket.FormatCloseMessage(websocket.CloseNormalClosure, fmt.Sprintf("%v", err))
				if e, ok := err.(*websocket.CloseError); ok {
					if e.Code != websocket.CloseNoStatusReceived {
						m = websocket.FormatCloseMessage(e.Code, e.Text)
					}
				}
				errc <- err
				// Best effort close message
				ws.WriteMessage(websocket.CloseMessage, m)
				return
			}
			
			// Fast prefix check for special messages using direct comparison
			if len(msg) >= 8 && bytes.Equal(msg[:8], portPrefix) {
				continue
			}
			
			// Short write deadline for better throughput
			if err := backend.SetWriteDeadline(time.Now().Add(2 * time.Second)); err != nil {
				errc <- err
				return
			}
			
			// Zero-copy write directly to UDP
			_, err = backend.WriteTo(msg, w.addr)
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
			bufInterface := bufferPool.Get()
			buffer := bufInterface.([]byte)
			
			// Read from UDP socket with deadline
			if err := backend.SetReadDeadline(time.Now().Add(30 * time.Second)); err != nil {
				bufferPool.Put(bufInterface)
				errc <- err
				return
			}
			
			n, _, err := backend.ReadFrom(buffer)
			if err != nil {
				bufferPool.Put(bufInterface)
				errc <- err
				return
			}
			
			// Write directly to websocket from the buffer slice
			if err := ws.WriteMessage(websocket.BinaryMessage, buffer[:n]); err != nil {
				bufferPool.Put(bufInterface)
				errc <- err
				return
			}
			
			// Return buffer to pool
			bufferPool.Put(bufInterface)
		}
	}()
	
	// Wait for any goroutine to signal completion
	select {
	case err := <-errc:
		if e, ok := err.(*websocket.CloseError); !ok || e.Code == websocket.CloseAbnormalClosure {
			log.Printf("wsproxy: %v", err)
		}
	case <-ctx.Done():
		// Context canceled (e.g., client disconnected)
		return
	}
}
