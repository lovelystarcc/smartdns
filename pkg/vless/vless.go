package vless

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdh"
	"crypto/sha256"
	"crypto/tls"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"io"
	"net"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	utls "github.com/refraction-networking/utls"
	"golang.org/x/crypto/hkdf"

	"smartnet/pkg/config"
)

// ParseURL parses a vless:// URI into a VLESSConfig.
// Format: vless://uuid@server:port?encryption=none&security=reality&sni=sni.domain&pbk=public_key&sid=short_id&flow=xtls-rprx-vision#name
func ParseURL(rawURL string) (*config.VLESSConfig, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("invalid VLESS URL: %w", err)
	}

	if u.Scheme != "vless" {
		return nil, fmt.Errorf("unsupported scheme %q (expected vless://)", u.Scheme)
	}

	uuidStr := u.User.Username()
	if uuidStr == "" {
		return nil, fmt.Errorf("missing UUID in VLESS URL")
	}

	server := u.Hostname()
	if server == "" {
		return nil, fmt.Errorf("missing server host in VLESS URL")
	}

	portStr := u.Port()
	if portStr == "" {
		portStr = "443"
	}
	port, err := strconv.Atoi(portStr)
	if err != nil || port <= 0 || port > 65535 {
		return nil, fmt.Errorf("invalid port %q in VLESS URL", portStr)
	}

	q := u.Query()
	cfg := &config.VLESSConfig{
		Server:     server,
		Port:       port,
		UUID:       uuidStr,
		Flow:       q.Get("flow"),
		ServerName: q.Get("sni"),
		PublicKey:  q.Get("pbk"),
		ShortID:    q.Get("sid"),
		SpiderX:    q.Get("spx"),
	}

	if cfg.ServerName == "" {
		cfg.ServerName = server
	}

	return cfg, nil
}

// Dialer establishes outbound connections over VLESS-Reality.
type Dialer struct {
	cfg *config.VLESSConfig
}

// NewDialer creates a new VLESS dialer.
func NewDialer(cfg *config.VLESSConfig) *Dialer {
	return &Dialer{cfg: cfg}
}

// Dial connects to target (host:port) through the VLESS server.
func (d *Dialer) Dial(network, target string) (net.Conn, error) {
	return d.DialContext(context.Background(), network, target)
}

// DialContext connects to target through the VLESS server with context.
func (d *Dialer) DialContext(ctx context.Context, network, target string) (net.Conn, error) {
	serverAddr := net.JoinHostPort(d.cfg.Server, strconv.Itoa(d.cfg.Port))

	var dialer net.Dialer
	rawConn, err := dialer.DialContext(ctx, "tcp", serverAddr)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to VLESS server %s: %w", serverAddr, err)
	}

	var tlsConn net.Conn
	if d.cfg.PublicKey != "" {
		tlsConn, err = realityHandshake(ctx, rawConn, d.cfg)
		if err != nil {
			_ = rawConn.Close()
			return nil, err
		}
	} else {
		tlsConfig := &tls.Config{
			ServerName:         d.cfg.ServerName,
			InsecureSkipVerify: true,
		}
		stdConn := tls.Client(rawConn, tlsConfig)
		if err := stdConn.HandshakeContext(ctx); err != nil {
			_ = rawConn.Close()
			return nil, fmt.Errorf("VLESS TLS handshake failed with %s: %w", d.cfg.ServerName, err)
		}
		tlsConn = stdConn
	}

	// Build and send VLESS request header
	cmd := byte(0x01) // TCP
	if strings.HasPrefix(network, "udp") {
		cmd = byte(0x02) // UDP
	}

	hdr, err := buildRequestHeader(d.cfg.UUID, cmd, target)
	if err != nil {
		_ = tlsConn.Close()
		return nil, fmt.Errorf("failed to build VLESS header: %w", err)
	}

	if _, err := tlsConn.Write(hdr); err != nil {
		_ = tlsConn.Close()
		return nil, fmt.Errorf("failed to send VLESS header: %w", err)
	}

	return &vlessConn{Conn: tlsConn}, nil
}

type vlessConn struct {
	net.Conn
	mu         sync.Mutex
	headerRead bool
}

func (c *vlessConn) Read(b []byte) (int, error) {
	c.mu.Lock()
	if !c.headerRead {
		respHdr := make([]byte, 2)
		if _, err := io.ReadFull(c.Conn, respHdr); err != nil {
			c.mu.Unlock()
			return 0, fmt.Errorf("failed to read VLESS response header: %w", err)
		}
		if respHdr[0] != 0x00 {
			c.mu.Unlock()
			return 0, fmt.Errorf("unsupported VLESS response version: %d", respHdr[0])
		}
		addonLen := int(respHdr[1])
		if addonLen > 0 {
			addonBuf := make([]byte, addonLen)
			if _, err := io.ReadFull(c.Conn, addonBuf); err != nil {
				c.mu.Unlock()
				return 0, fmt.Errorf("failed to read VLESS addon bytes: %w", err)
			}
		}
		c.headerRead = true
	}
	c.mu.Unlock()
	return c.Conn.Read(b)
}

func parseUUID(uuidStr string) ([]byte, error) {
	clean := strings.ReplaceAll(uuidStr, "-", "")
	if len(clean) != 32 {
		return nil, fmt.Errorf("UUID must be 32 hex chars, got %d", len(clean))
	}
	return hex.DecodeString(clean)
}

func buildRequestHeader(uuidStr string, cmd byte, target string) ([]byte, error) {
	uuidBytes, err := parseUUID(uuidStr)
	if err != nil {
		return nil, err
	}

	host, portStr, err := net.SplitHostPort(target)
	if err != nil {
		return nil, fmt.Errorf("invalid target %q: %w", target, err)
	}

	portNum, err := strconv.Atoi(portStr)
	if err != nil || portNum <= 0 || portNum > 65535 {
		return nil, fmt.Errorf("invalid port %q: %w", portStr, err)
	}

	var buf bytes.Buffer
	buf.WriteByte(0x00) // VLESS version 0
	buf.Write(uuidBytes) // 16 bytes UUID
	buf.WriteByte(0x00) // Addon length 0
	buf.WriteByte(cmd)  // 0x01 = TCP, 0x02 = UDP

	// Port (2 bytes BigEndian)
	var portBytes [2]byte
	binary.BigEndian.PutUint16(portBytes[:], uint16(portNum))
	buf.Write(portBytes[:])

	// Address Type & Address
	if ip := net.ParseIP(host); ip != nil {
		if v4 := ip.To4(); v4 != nil {
			buf.WriteByte(0x01) // IPv4
			buf.Write(v4)
		} else {
			buf.WriteByte(0x03) // IPv6
			buf.Write(ip.To16())
		}
	} else {
		// Domain
		buf.WriteByte(0x02) // Domain
		buf.WriteByte(byte(len(host)))
		buf.WriteString(host)
	}

	return buf.Bytes(), nil
}

func realityHandshake(ctx context.Context, rawConn net.Conn, cfg *config.VLESSConfig) (net.Conn, error) {
	pubKeyBytes, err := base64.RawURLEncoding.DecodeString(cfg.PublicKey)
	if err != nil {
		pubKeyBytes, err = base64.URLEncoding.DecodeString(cfg.PublicKey)
		if err != nil {
			return nil, fmt.Errorf("invalid reality public key base64: %w", err)
		}
	}
	var shortIDBytes []byte
	if cfg.ShortID != "" {
		shortIDBytes, err = hex.DecodeString(cfg.ShortID)
		if err != nil {
			return nil, fmt.Errorf("invalid reality short_id hex: %w", err)
		}
	}

	uConn := utls.UClient(rawConn, &utls.Config{
		ServerName:             cfg.ServerName,
		InsecureSkipVerify:     true,
		SessionTicketsDisabled: true,
	}, utls.HelloChrome_Auto)

	if err := uConn.BuildHandshakeState(); err != nil {
		return nil, fmt.Errorf("failed to build uTLS handshake state: %w", err)
	}

	hello := uConn.HandshakeState.Hello
	hello.SessionId = make([]byte, 32)
	copy(hello.Raw[39:], hello.SessionId) // КРИТИЧНО: зануляем AAD в сыром ClientHello

	hello.SessionId[0] = 1
	hello.SessionId[1] = 8
	hello.SessionId[2] = 0
	hello.SessionId[3] = 0
	binary.BigEndian.PutUint32(hello.SessionId[4:], uint32(time.Now().Unix()))
	if len(shortIDBytes) > 0 {
		copy(hello.SessionId[8:], shortIDBytes)
	}

	publicKey, err := ecdh.X25519().NewPublicKey(pubKeyBytes)
	if err != nil {
		return nil, fmt.Errorf("invalid x25519 public key: %w", err)
	}

	ecdhe := uConn.HandshakeState.State13.KeyShareKeys.Ecdhe
	if ecdhe == nil {
		ecdhe = uConn.HandshakeState.State13.KeyShareKeys.MlkemEcdhe
	}
	if ecdhe == nil {
		return nil, fmt.Errorf("ecdhe keys not found in TLS 1.3 ClientHello")
	}

	sharedKey, err := ecdhe.ECDH(publicKey)
	if err != nil {
		return nil, fmt.Errorf("reality ECDH exchange failed: %w", err)
	}

	authKey := make([]byte, 32)
	if _, err := hkdf.New(sha256.New, sharedKey, hello.Random[:20], []byte("REALITY")).Read(authKey); err != nil {
		return nil, fmt.Errorf("reality HKDF key derivation failed: %w", err)
	}

	block, err := aes.NewCipher(authKey)
	if err != nil {
		return nil, fmt.Errorf("reality AES cipher init failed: %w", err)
	}

	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("reality GCM init failed: %w", err)
	}

	aead.Seal(hello.SessionId[:0], hello.Random[20:], hello.SessionId[:16], hello.Raw)
	copy(hello.Raw[39:], hello.SessionId)

	if err := uConn.HandshakeContext(ctx); err != nil {
		return nil, fmt.Errorf("reality TLS handshake failed with %s: %w", cfg.ServerName, err)
	}
	return uConn, nil
}
