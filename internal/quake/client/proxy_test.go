package client

import (
	"net"
	"net/http/httptest"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

func startUDPEcho(tb testing.TB) string {
	tb.Helper()
	pc, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		tb.Fatal(err)
	}
	tb.Cleanup(func() { _ = pc.Close() })
	_ = pc.(*net.UDPConn).SetReadBuffer(4 << 20)
	go func() {
		buf := make([]byte, 2048)
		for {
			n, addr, err := pc.ReadFrom(buf)
			if err != nil {
				return
			}
			_, _ = pc.WriteTo(buf[:n], addr) // loss is tolerated, like real UDP
		}
	}()
	return pc.LocalAddr().String()
}

func dialProxy(tb testing.TB, udpAddr string) *websocket.Conn {
	tb.Helper()
	p, err := NewProxy(udpAddr)
	if err != nil {
		tb.Fatal(err)
	}
	ts := httptest.NewServer(p)
	tb.Cleanup(ts.Close)
	ws, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(ts.URL, "http"), nil)
	if err != nil {
		tb.Fatal(err)
	}
	tb.Cleanup(func() { _ = ws.Close() })
	return ws
}

func TestProxyRoundTrip(t *testing.T) {
	ws := dialProxy(t, startUDPEcho(t))
	msg := []byte("\xff\xff\xff\xffgetinfo")
	if err := ws.WriteMessage(websocket.BinaryMessage, msg); err != nil {
		t.Fatal(err)
	}
	_ = ws.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, got, err := ws.ReadMessage()
	if err != nil || string(got) != string(msg) {
		t.Fatalf("got %q, err %v", got, err)
	}
}

func BenchmarkProxyRoundTrip(b *testing.B) {
	ws := dialProxy(b, startUDPEcho(b))
	msg := make([]byte, 200)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := ws.WriteMessage(websocket.BinaryMessage, msg); err != nil {
			b.Fatal(err)
		}
		if _, _, err := ws.ReadMessage(); err != nil {
			b.Fatal(err)
		}
	}
}

// TestProxyManyClients drives concurrent clients through one proxy and checks
// every packet echoes back and no goroutines leak after they disconnect.
func TestProxyManyClients(t *testing.T) {
	const clients, packets = 200, 200
	udp := startUDPEcho(t)
	p, err := NewProxy(udp)
	if err != nil {
		t.Fatal(err)
	}
	ts := httptest.NewServer(p)
	defer ts.Close()
	before := runtime.NumGoroutine()

	var wg sync.WaitGroup
	var failed atomic.Int64
	// Limit simultaneous dials: a connect burst overflows the loopback listen
	// backlog (ECONNRESET) regardless of the proxy.
	dialing := make(chan struct{}, 20)
	start := time.Now()
	for i := 0; i < clients; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			dialing <- struct{}{}
			ws, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(ts.URL, "http"), nil)
			<-dialing
			if err != nil {
				t.Logf("dial: %v", err)
				failed.Add(1)
				return
			}
			defer ws.Close()
			msg := make([]byte, 300)
			for j := 0; j < packets; j++ {
				_ = ws.SetReadDeadline(time.Now().Add(5 * time.Second))
				if err := ws.WriteMessage(websocket.BinaryMessage, msg); err != nil {
					t.Logf("write: %v", err)
					failed.Add(1)
					return
				}
				if _, got, err := ws.ReadMessage(); err != nil || len(got) != len(msg) {
					t.Logf("read: %d bytes, %v", len(got), err)
					failed.Add(1)
					return
				}
			}
		}()
	}
	wg.Wait()
	t.Logf("%d clients x %d packets in %v (%.0f pkt/s)", clients, packets, time.Since(start),
		float64(clients*packets)/time.Since(start).Seconds())
	if n := failed.Load(); n != 0 {
		t.Fatalf("%d clients failed", n)
	}
	deadline := time.Now().Add(3 * time.Second)
	for runtime.NumGoroutine() > before+5 && time.Now().Before(deadline) {
		time.Sleep(50 * time.Millisecond)
	}
	if n := runtime.NumGoroutine(); n > before+5 {
		t.Fatalf("goroutine leak: before %d, after %d", before, n)
	}
}
