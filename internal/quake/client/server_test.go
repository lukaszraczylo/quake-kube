package client

import (
	"net"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

func startServer(t *testing.T, udpAddr string) string {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = l.Close() })
	s := &Server{
		Addr:       l.Addr().String(),
		ServerAddr: udpAddr,
		Handler:    http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("page")) }),
	}
	go func() { _ = s.Serve(l) }()
	return l.Addr().String()
}

func echoUDP(t *testing.T) string {
	t.Helper()
	pc, err := net.ListenPacket("udp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = pc.Close() })
	go func() {
		buf := make([]byte, 2048)
		for {
			n, addr, err := pc.ReadFrom(buf)
			if err != nil {
				return
			}
			_, _ = pc.WriteTo(buf[:n], addr)
		}
	}()
	return pc.LocalAddr().String()
}

func TestServeSplitsHTTPAndWebsocket(t *testing.T) {
	addr := startServer(t, echoUDP(t))

	resp, err := http.Get("http://" + addr + "/")
	if err != nil {
		t.Fatal(err)
	}
	defer closeBody(resp)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("http status %d", resp.StatusCode)
	}

	ws, _, err := websocket.DefaultDialer.Dial("ws://"+addr+"/", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = ws.Close() }()
	if werr := ws.WriteMessage(websocket.BinaryMessage, []byte("ping")); werr != nil {
		t.Fatal(werr)
	}
	_ = ws.SetReadDeadline(time.Now().Add(5 * time.Second))
	_, msg, err := ws.ReadMessage()
	if err != nil {
		t.Fatal(err)
	}
	if string(msg) != "ping" {
		t.Fatalf("echo %q", msg)
	}
}

func TestServeKeepsWebsocketPastHTTPTimeouts(t *testing.T) {
	// A hijacked connection must not inherit the HTTP write deadline.
	addr := startServer(t, echoUDP(t))
	ws, _, err := websocket.DefaultDialer.Dial("ws://"+addr+"/", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = ws.Close() }()
	time.Sleep(200 * time.Millisecond)
	if err := ws.WriteMessage(websocket.BinaryMessage, []byte("late")); err != nil {
		t.Fatal(err)
	}
	_ = ws.SetReadDeadline(time.Now().Add(5 * time.Second))
	if _, _, err := ws.ReadMessage(); err != nil {
		t.Fatal(err)
	}
}

func TestServeRejectsBadServerAddr(t *testing.T) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = l.Close() }()
	s := &Server{Addr: l.Addr().String(), ServerAddr: "no-port", Handler: http.NotFoundHandler()}
	if err := s.Serve(l); err == nil || !strings.Contains(err.Error(), "port") {
		t.Fatalf("err = %v", err)
	}
}

func closeBody(r *http.Response) { _ = r.Body.Close() }
