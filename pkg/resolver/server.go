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
	"strings"
	"sync"
	"time"

	"smartdns/pkg/config"
	"smartdns/pkg/network"

	"github.com/miekg/dns"
)

type UpstreamState int

const (
	StateHealthy UpstreamState = iota
	StateDegraded
)

const MaxCacheEntries = 10000

func (s UpstreamState) String() string {
	switch s {
	case StateHealthy:
		return "Healthy"
	case StateDegraded:
		return "Degraded (Fallback Active)"
	default:
		return "Unknown"
	}
}

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

	// Health and Fallback state
	stateMu          sync.RWMutex
	state            UpstreamState
	consecutiveFails int
}

func New(cfg *config.Config) *Server {
	s := &Server{
		cfg:    cfg,
		cache:  make(map[string]cacheItem),
		stopCh: make(chan struct{}),
		state:  StateHealthy,
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
		Timeout:   4 * time.Second,
	}
}

func stringsContainsPort(s string) bool {
	_, _, err := net.SplitHostPort(s)
	return err == nil
}

func resolveViaBootstrap(ctx context.Context, host string, bootstraps []string) (string, error) {
	if len(bootstraps) == 0 {
		return "", fmt.Errorf("no bootstrap servers provided")
	}

	raceCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	m := new(dns.Msg)
	m.SetQuestion(dns.Fqdn(host), dns.TypeA)

	type result struct {
		ip  string
		err error
	}
	ch := make(chan result, len(bootstraps))

	for _, b := range bootstraps {
		go func(srv string) {
			c := &dns.Client{
				Net:     "udp",
				Timeout: 1500 * time.Millisecond,
			}
			req := m.Copy()
			resp, _, err := c.ExchangeContext(raceCtx, req, srv)
			if err == nil && resp != nil && len(resp.Answer) > 0 {
				for _, ans := range resp.Answer {
					if a, ok := ans.(*dns.A); ok {
						ch <- result{ip: a.A.String(), err: nil}
						return
					}
				}
			}
			if err == nil {
				err = fmt.Errorf("no A record from %s", srv)
			}
			ch <- result{ip: "", err: err}
		}(b)
	}

	var lastErr error
	for i := 0; i < len(bootstraps); i++ {
		select {
		case res := <-ch:
			if res.err == nil && res.ip != "" {
				cancel()
				return res.ip, nil
			}
			lastErr = res.err
		case <-raceCtx.Done():
			if lastErr != nil {
				return "", lastErr
			}
			return "", fmt.Errorf("bootstrap resolution timed out for %s", host)
		}
	}

	if lastErr != nil {
		return "", lastErr
	}
	return "", fmt.Errorf("bootstrap resolution failed for %s", host)
}

func (s *Server) getFallbackServers() []string {
	servers := []string{"1.1.1.1:53", "8.8.8.8:53", "77.88.8.8:53"}
	if gw := network.GetPhysicalGateway(); gw != "" {
		servers = append([]string{gw + ":53"}, servers...)
	}
	return servers
}

func isLocalOrCaptive(domain string) bool {
	d := strings.ToLower(strings.TrimSuffix(domain, "."))
	if strings.HasSuffix(d, ".local") || strings.HasSuffix(d, ".lan") ||
		strings.HasSuffix(d, ".home") || strings.HasSuffix(d, ".internal") ||
		d == "localhost" {
		return true
	}
	captiveDomains := []string{
		"captive.apple.com",
		"connectivitycheck.gstatic.com",
		"connectivitycheck.android.com",
		"detectportal.firefox.com",
		"msftconnecttest.com",
		"ipv6.msftconnecttest.com",
		"www.msftconnecttest.com",
		"networkcheck.kde.org",
	}
	for _, cd := range captiveDomains {
		if d == cd || strings.HasSuffix(d, "."+cd) {
			return true
		}
	}
	return false
}

func (s *Server) Start() error {
	addr := s.cfg.ListenAddr
	if addr == "" {
		addr = "127.0.0.1:53"
	}

	handler := dns.HandlerFunc(s.handleDNS)
	s.udpServer = &dns.Server{Addr: addr, Net: "udp", Handler: handler}
	s.tcpServer = &dns.Server{Addr: addr, Net: "tcp", Handler: handler}

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

	// Background health checker and auto-recovery
	go s.healthCheckLoop()

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

func (s *Server) recordFailure(upstream string) {
	s.stateMu.Lock()
	defer s.stateMu.Unlock()

	s.consecutiveFails++
	if s.consecutiveFails >= 3 && s.state == StateHealthy {
		s.state = StateDegraded
		log.Printf("[smartdns] ⚠️ Upstream DoH (%s) failed 3 consecutive times. Switched to Automatic Fallback mode (routing via standard DNS without delay).", upstream)
	}
}

func (s *Server) recordSuccess(upstream string) {
	s.stateMu.Lock()
	defer s.stateMu.Unlock()

	if s.state == StateDegraded {
		s.state = StateHealthy
		log.Printf("[smartdns] ✅ Primary Upstream DoH (%s) recovered! Restored SmartDNS routing.", upstream)
	}
	s.consecutiveFails = 0
}

func (s *Server) healthCheckLoop() {
	ticker := time.NewTicker(4 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-s.stopCh:
			return
		case <-ticker.C:
			s.stateMu.RLock()
			isDegraded := (s.state == StateDegraded)
			s.stateMu.RUnlock()

			if isDegraded {
				active := s.cfg.GetActive()
				if active == nil || active.Upstream == "" {
					continue
				}
				probe := new(dns.Msg)
				probe.SetQuestion("cloudflare.com.", dns.TypeA)
				resp, err := s.queryDoHWithTimeout(probe, active.Upstream, 2*time.Second)
				if err == nil && resp != nil && resp.Rcode == dns.RcodeSuccess {
					s.recordSuccess(active.Upstream)
				}
			}
		}
	}
}

func (s *Server) queryFallback(r *dns.Msg) (*dns.Msg, error) {
	fallbacks := s.getFallbackServers()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	type result struct {
		msg *dns.Msg
		err error
	}
	ch := make(chan result, len(fallbacks))

	for _, target := range fallbacks {
		go func(srv string) {
			c := &dns.Client{
				Net:     "udp",
				Timeout: 1200 * time.Millisecond,
			}
			req := r.Copy()
			resp, _, err := c.ExchangeContext(ctx, req, srv)
			if err == nil && resp != nil {
				ch <- result{msg: resp, err: nil}
			} else {
				ch <- result{msg: nil, err: err}
			}
		}(target)
	}

	var lastErr error
	for i := 0; i < len(fallbacks); i++ {
		select {
		case res := <-ch:
			if res.err == nil && res.msg != nil {
				cancel()
				return res.msg, nil
			}
			lastErr = res.err
		case <-ctx.Done():
			if lastErr != nil {
				return nil, lastErr
			}
			return nil, fmt.Errorf("fallback query timed out")
		}
	}

	if lastErr != nil {
		return nil, lastErr
	}
	return nil, fmt.Errorf("all fallback resolvers failed")
}

func (s *Server) cacheResponse(cacheKey string, resp *dns.Msg, isFallback bool) {
	if len(resp.Answer) == 0 {
		return
	}
	ttl := uint32(5)
	if !isFallback {
		ttl = resp.Answer[0].Header().Ttl
		if ttl < 5 {
			ttl = 5
		}
		if ttl > 3600 {
			ttl = 3600
		}
	}
	s.cacheMu.Lock()
	defer s.cacheMu.Unlock()

	// Enforce maximum cache capacity to avoid memory leaks (e.g. DNS water torture attacks)
	if len(s.cache) >= MaxCacheEntries {
		now := time.Now()
		for k, v := range s.cache {
			if now.After(v.expires) {
				delete(s.cache, k)
			}
		}
		// If still at or over capacity, evict 10% of entries
		if len(s.cache) >= MaxCacheEntries {
			evictCount := MaxCacheEntries / 10
			for k := range s.cache {
				delete(s.cache, k)
				evictCount--
				if evictCount <= 0 {
					break
				}
			}
		}
	}

	s.cache[cacheKey] = cacheItem{
		msg:     resp.Copy(),
		expires: time.Now().Add(time.Duration(ttl) * time.Second),
	}
}

func (s *Server) handleDNS(w dns.ResponseWriter, r *dns.Msg) {
	if len(r.Question) == 0 {
		dns.HandleFailed(w, r)
		return
	}

	q := r.Question[0]
	cacheKey := fmt.Sprintf("%s:%d:%d", q.Name, q.Qtype, q.Qclass)

	// 1. Direct local / captive portal bypass
	if isLocalOrCaptive(q.Name) {
		resp, err := s.queryFallback(r)
		if err == nil && resp != nil {
			resp.Id = r.Id
			_ = w.WriteMsg(resp)
			return
		}
	}

	// 2. Check cache
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

	// 3. Check circuit breaker state
	s.stateMu.RLock()
	currentState := s.state
	s.stateMu.RUnlock()

	active := s.cfg.GetActive()
	var resp *dns.Msg
	var err error
	usedFallback := false

	if currentState == StateHealthy && active != nil && active.Upstream != "" {
		// Normal path: Query Primary DoH with a 2.5s timeout
		resp, err = s.queryDoHWithTimeout(r, active.Upstream, 2500*time.Millisecond)
		if err != nil {
			log.Printf("[smartdns] ⚠️ Upstream DoH error for %s (%v).", q.Name, err)
			s.recordFailure(active.Upstream)

			// Try secondary DoH if configured
			if active.BackupUpstream != "" {
				backupResp, backupErr := s.queryDoHWithTimeout(r, active.BackupUpstream, 2000*time.Millisecond)
				if backupErr == nil && backupResp != nil {
					resp = backupResp
				}
			}

			// If secondary DoH also failed or wasn't configured, failover to parallel fallback
			if resp == nil {
				resp, err = s.queryFallback(r)
				usedFallback = true
				if err != nil {
					log.Printf("[smartdns] ❌ Fallback also failed for %s: %v", q.Name, err)
					dns.HandleFailed(w, r)
					return
				}
			}
		} else {
			s.recordSuccess(active.Upstream)
		}
	} else {
		// Circuit breaker tripped or no primary profile:
		// Try backup DoH if available before falling back to unencrypted DNS
		if active != nil && active.BackupUpstream != "" {
			resp, _ = s.queryDoHWithTimeout(r, active.BackupUpstream, 1500*time.Millisecond)
		}
		if resp == nil {
			// Go straight to fallback to avoid timeout latency
			resp, err = s.queryFallback(r)
			usedFallback = true
			if err != nil {
				log.Printf("[smartdns] ❌ Fallback query failed for %s: %v", q.Name, err)
				dns.HandleFailed(w, r)
				return
			}
		}
	}

	// 4. Cache response (short 5s TTL for fallback to prevent sticking to Direct IP)
	s.cacheResponse(cacheKey, resp, usedFallback)

	resp.Id = r.Id
	_ = w.WriteMsg(resp)
}

func (s *Server) queryDoH(req *dns.Msg, dohURL string) (*dns.Msg, error) {
	return s.queryDoHWithTimeout(req, dohURL, 3*time.Second)
}

func (s *Server) queryDoHWithTimeout(req *dns.Msg, dohURL string, timeout time.Duration) (*dns.Msg, error) {
	wire, err := req.Pack()
	if err != nil {
		return nil, fmt.Errorf("failed to pack DNS message: %w", err)
	}

	u, err := url.Parse(dohURL)
	if err != nil {
		return nil, fmt.Errorf("invalid upstream URL: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	httpReq, err := http.NewRequestWithContext(
		ctx,
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
