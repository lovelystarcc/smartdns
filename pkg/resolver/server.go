package resolver

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"sync"
	"time"

	"smartdns/pkg/config"
	"smartdns/pkg/network"

	"github.com/miekg/dns"
)

type cacheItem struct {
	msg     *dns.Msg
	expires time.Time
}

type Server struct {
	cfg       *config.Config
	cache     map[string]cacheItem
	cacheMu   sync.RWMutex
	client    *http.Client
	transport *http.Transport
	udpServer *dns.Server
	tcpServer *dns.Server
	stopCh    chan struct{}
}

func New(cfg *config.Config) *Server {
	s := &Server{
		cfg:    cfg,
		cache:  make(map[string]cacheItem),
		stopCh: make(chan struct{}),
	}
	s.initHTTPClient()
	return s
}

func (s *Server) initHTTPClient() {
	active := s.cfg.GetActive()
	var bootstrapAddrs []string
	if active != nil && len(active.Bootstrap) > 0 {
		for _, b := range active.Bootstrap {
			if !stringsContainsPort(b) {
				bootstrapAddrs = append(bootstrapAddrs, b+":53")
			} else {
				bootstrapAddrs = append(bootstrapAddrs, b)
			}
		}
	} else {
		bootstrapAddrs = []string{"1.1.1.1:53", "8.8.8.8:53", "77.88.8.8:53"}
		if gw := network.GetPhysicalGateway(); gw != "" {
			bootstrapAddrs = append(bootstrapAddrs, gw+":53")
		}
	}

	dialer := &net.Dialer{
		Timeout:   3 * time.Second,
		KeepAlive: 30 * time.Second,
	}

	// Custom resolver for DoH server host to avoid recursion
	s.transport = &http.Transport{
		Proxy:                 nil,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   4 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: false,
		},
		DialContext: func(ctx context.Context, netw, addr string) (net.Conn, error) {
			host, port, err := net.SplitHostPort(addr)
			if err != nil {
				return dialer.DialContext(ctx, netw, addr)
			}

			// 1. Direct IP or parsed from sslip.io/nip.io (zero DNS queries needed)
			if ip := network.ParseIPFromHostname(host); ip != nil {
				return dialer.DialContext(ctx, netw, net.JoinHostPort(ip.String(), port))
			}

			// 2. Resolve host via bootstrap DNS (including local gateway fallback)
			resolvedIP, err := resolveViaBootstrap(ctx, host, bootstrapAddrs)
			if err == nil && resolvedIP != "" {
				return dialer.DialContext(ctx, netw, net.JoinHostPort(resolvedIP, port))
			}

			return dialer.DialContext(ctx, netw, addr)
		},
	}

	s.client = &http.Client{
		Transport: s.transport,
		Timeout:   5 * time.Second,
	}
}

func stringsContainsPort(s string) bool {
	_, _, err := net.SplitHostPort(s)
	return err == nil
}

func resolveViaBootstrap(ctx context.Context, host string, bootstraps []string) (string, error) {
	m := new(dns.Msg)
	m.SetQuestion(dns.Fqdn(host), dns.TypeA)
	c := new(dns.Client)
	c.Timeout = 2 * time.Second

	for _, b := range bootstraps {
		r, _, err := c.ExchangeContext(ctx, m, b)
		if err == nil && r != nil && len(r.Answer) > 0 {
			for _, ans := range r.Answer {
				if a, ok := ans.(*dns.A); ok {
					return a.A.String(), nil
				}
			}
		}
	}
	return "", fmt.Errorf("bootstrap resolution failed for %s", host)
}

func (s *Server) Start() error {
	addr := s.cfg.ListenAddr
	if addr == "" {
		addr = "127.0.0.1:53"
	}

	dns.HandleFunc(".", s.handleDNS)

	s.udpServer = &dns.Server{Addr: addr, Net: "udp"}
	s.tcpServer = &dns.Server{Addr: addr, Net: "tcp"}

	errCh := make(chan error, 2)

	go func() {
		log.Printf("[smartdns] Listening UDP on %s", addr)
		if err := s.udpServer.ListenAndServe(); err != nil {
			errCh <- err
		}
	}()

	go func() {
		log.Printf("[smartdns] Listening TCP on %s", addr)
		if err := s.tcpServer.ListenAndServe(); err != nil {
			errCh <- err
		}
	}()

	// Background cache cleaner
	go s.cleanCacheLoop()

	select {
	case err := <-errCh:
		return err
	case <-time.After(100 * time.Millisecond):
		return nil
	}
}

func (s *Server) Stop() {
	close(s.stopCh)
	if s.udpServer != nil {
		_ = s.udpServer.Shutdown()
	}
	if s.tcpServer != nil {
		_ = s.tcpServer.Shutdown()
	}
	if s.transport != nil {
		s.transport.CloseIdleConnections()
	}
}

func (s *Server) cleanCacheLoop() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-s.stopCh:
			return
		case <-ticker.C:
			now := time.Now()
			s.cacheMu.Lock()
			for k, v := range s.cache {
				if now.After(v.expires) {
					delete(s.cache, k)
				}
			}
			s.cacheMu.Unlock()
		}
	}
}

func (s *Server) handleDNS(w dns.ResponseWriter, r *dns.Msg) {
	if len(r.Question) == 0 {
		dns.HandleFailed(w, r)
		return
	}

	q := r.Question[0]
	cacheKey := fmt.Sprintf("%s:%d:%d", q.Name, q.Qtype, q.Qclass)

	// 1. Check cache
	s.cacheMu.RLock()
	item, found := s.cache[cacheKey]
	s.cacheMu.RUnlock()

	now := time.Now()
	if found && now.Before(item.expires) {
		resp := item.msg.Copy()
		resp.Id = r.Id
		_ = w.WriteMsg(resp)
		return
	}

	// 2. Query Upstream DoH
	active := s.cfg.GetActive()
	if active == nil || active.Upstream == "" {
		dns.HandleFailed(w, r)
		return
	}

	resp, err := s.queryDoH(r, active.Upstream)
	if err != nil {
		log.Printf("[smartdns] DoH query error to %s: %v", active.Upstream, err)
		dns.HandleFailed(w, r)
		return
	}

	// 3. Cache response if TTL > 0
	ttl := uint32(60)
	if len(resp.Answer) > 0 {
		ttl = resp.Answer[0].Header().Ttl
		if ttl < 5 {
			ttl = 5
		}
		if ttl > 3600 {
			ttl = 3600
		}
		s.cacheMu.Lock()
		s.cache[cacheKey] = cacheItem{
			msg:     resp.Copy(),
			expires: now.Add(time.Duration(ttl) * time.Second),
		}
		s.cacheMu.Unlock()
	}

	resp.Id = r.Id
	_ = w.WriteMsg(resp)
}

func (s *Server) queryDoH(req *dns.Msg, dohURL string) (*dns.Msg, error) {
	wire, err := req.Pack()
	if err != nil {
		return nil, fmt.Errorf("failed to pack DNS message: %w", err)
	}

	u, err := url.Parse(dohURL)
	if err != nil {
		return nil, fmt.Errorf("invalid upstream URL: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		u.String(),
		bytes.NewReader(wire),
	)
	if err != nil {
		return nil, err
	}

	httpReq.Header.Set("Content-Type", "application/dns-message")
	httpReq.Header.Set("Accept", "application/dns-message")

	httpResp, err := s.client.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(httpResp.Body, 512))
		return nil, fmt.Errorf("DoH status %d: %s", httpResp.StatusCode, string(body))
	}

	respBytes, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return nil, err
	}

	resp := new(dns.Msg)
	if err := resp.Unpack(respBytes); err != nil {
		return nil, fmt.Errorf("failed to unpack DNS response: %w", err)
	}

	return resp, nil
}
