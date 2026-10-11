package tun

import (
	"errors"
	"net"
	"os"
	"testing"
	"time"

	"smartdns/pkg/config"
	"smartdns/pkg/fakeip"

	"golang.zx2c4.com/wireguard/tun"
)

type mockTun struct {
	name   string
	mtu    int
	events chan tun.Event
	closed chan struct{}
}

func newMockTun(name string, mtu int) *mockTun {
	return &mockTun{
		name:   name,
		mtu:    mtu,
		events: make(chan tun.Event),
		closed: make(chan struct{}),
	}
}

func (m *mockTun) File() *os.File { return nil }
func (m *mockTun) Read(bufs [][]byte, sizes []int, offset int) (int, error) {
	<-m.closed
	return 0, errors.New("device closed")
}
func (m *mockTun) Write(bufs [][]byte, offset int) (int, error) {
	return len(bufs), nil
}
func (m *mockTun) MTU() (int, error)    { return m.mtu, nil }
func (m *mockTun) Name() (string, error) { return m.name, nil }
func (m *mockTun) Events() <-chan tun.Event { return m.events }
func (m *mockTun) Close() error {
	select {
	case <-m.closed:
	default:
		close(m.closed)
	}
	return nil
}
func (m *mockTun) BatchSize() int { return 1 }

func TestStackLifecycle(t *testing.T) {
	cfg := config.DefaultConfig()
	mt := newMockTun("mocktun0", 1500)
	dev := &Device{
		tunDev: mt,
		name:   mt.name,
		mtu:    mt.mtu,
	}

	st, err := NewStackWithDevice(cfg, dev)
	if err != nil {
		t.Fatalf("NewStackWithDevice failed: %v", err)
	}

	if st.DeviceName() != "mocktun0" {
		t.Errorf("Expected DeviceName mocktun0, got %s", st.DeviceName())
	}

	if err := st.Start(); err != nil {
		t.Fatalf("Failed to start stack: %v", err)
	}

	time.Sleep(50 * time.Millisecond)

	st.Stop()
	// Calling Stop again should be safe (idempotent)
	st.Stop()
}

func TestFakeIPTranslationInDial(t *testing.T) {
	// Setup Fake-IP mapping
	domain := "tun-test-domain.org"
	fakeIP := fakeip.DefaultPool.Allocate(domain)
	if fakeIP == nil {
		t.Fatalf("Failed to allocate fake IP")
	}

	if !fakeip.DefaultPool.IsFakeIP(fakeIP) {
		t.Errorf("Expected %v to be recognized as Fake-IP", fakeIP)
	}

	resolved, found := fakeip.DefaultPool.LookupIP(fakeIP)
	if !found || resolved != domain {
		t.Errorf("Expected lookup to yield %s, got %s (found: %v)", domain, resolved, found)
	}

	// Start a local test server
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to start test listener: %v", err)
	}
	defer ln.Close()

	serverReceived := make(chan string, 1)
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		buf := make([]byte, 16)
		n, _ := conn.Read(buf)
		serverReceived <- string(buf[:n])
	}()

	// Connect directly to listener
	conn, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatalf("Dial failed: %v", err)
	}
	_, _ = conn.Write([]byte("ping"))
	_ = conn.Close()

	select {
	case msg := <-serverReceived:
		if msg != "ping" {
			t.Errorf("Expected ping, got %s", msg)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("Timed out waiting for test server")
	}
}

func TestStackForwardTCP(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen failed: %v", err)
	}
	defer ln.Close()

	_, portStr, _ := net.SplitHostPort(ln.Addr().String())
	var portNum int
	for _, ch := range portStr {
		portNum = portNum*10 + int(ch-'0')
	}

	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		buf := make([]byte, 32)
		n, _ := conn.Read(buf)
		_, _ = conn.Write(append([]byte("echo:"), buf[:n]...))
	}()

	cfg := config.DefaultConfig()
	mt := newMockTun("mocktun1", 1500)
	dev := &Device{tunDev: mt, name: mt.name, mtu: mt.mtu}
	st, err := NewStackWithDevice(cfg, dev)
	if err != nil {
		t.Fatalf("NewStackWithDevice: %v", err)
	}
	defer st.Stop()

	clientPipe, serverPipe := net.Pipe()
	go st.forwardTCP(serverPipe, net.ParseIP("127.0.0.1"), uint16(portNum))

	_, _ = clientPipe.Write([]byte("hello"))
	buf := make([]byte, 32)
	n, err := clientPipe.Read(buf)
	if err != nil {
		t.Fatalf("Read from pipe: %v", err)
	}
	if string(buf[:n]) != "echo:hello" {
		t.Errorf("Expected echo:hello, got %s", string(buf[:n]))
	}
	_ = clientPipe.Close()
}

