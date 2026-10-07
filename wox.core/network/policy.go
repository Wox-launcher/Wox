// Package network owns Wox's outbound network policy without depending on settings or UI.
package network

import (
	"context"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
)

var ErrOffline error = offlineError{}
var errorTranslator atomic.Value

type offlineError struct{}

func (offlineError) Error() string {
	if translate := errorTranslator.Load(); translate != nil {
		return translate.(func() string)()
	}
	return "Offline mode is enabled. Turn it off in Privacy settings to use this feature."
}

// SetErrorTranslator supplies localized errors without coupling policy to application initialization.
func SetErrorTranslator(translate func() string) { errorTranslator.Store(translate) }

// Policy tracks operations until their response bodies close, including streaming requests.
type Policy struct {
	mu         sync.Mutex
	offline    bool
	next       uint64
	active     map[uint64]context.CancelFunc
	transports map[*http.Transport]struct{}
}

var Default = &Policy{}

func IsOffline() bool             { return Default.IsOffline() }
func (p *Policy) IsOffline() bool { p.mu.Lock(); defer p.mu.Unlock(); return p.offline }

// SetOffline closes the admission gate before cancelling existing external operations.
func (p *Policy) SetOffline(enabled bool) {
	p.mu.Lock()
	p.offline = enabled
	var cancels []context.CancelFunc
	var transports []*http.Transport
	if enabled {
		for transport := range p.transports {
			transports = append(transports, transport)
		}
		p.transports = nil
		for _, cancel := range p.active {
			cancels = append(cancels, cancel)
		}
	}
	p.mu.Unlock()
	for _, cancel := range cancels {
		cancel()
	}
	for _, transport := range transports {
		transport.CloseIdleConnections()
	}
}

// LocalURL allows only explicit loopback hosts; arbitrary DNS names never get resolved offline.
func LocalURL(u *url.URL) bool {
	if u == nil {
		return false
	}
	host := strings.ToLower(u.Hostname())
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func Check(rawURL string) error {
	u, err := url.Parse(rawURL)
	if err != nil {
		return err
	}
	if IsOffline() && !LocalURL(u) {
		return ErrOffline
	}
	return nil
}

// Begin atomically registers work that must stop when offline mode is enabled.
// Callers must release it after all I/O or subprocess work has finished.
func (p *Policy) Begin(ctx context.Context) (context.Context, func(), error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.offline {
		return ctx, func() {}, ErrOffline
	}
	ctx, cancel := context.WithCancelCause(ctx)
	if p.active == nil {
		p.active = make(map[uint64]context.CancelFunc)
	}
	p.next++
	id := p.next
	p.active[id] = func() { cancel(ErrOffline) }
	return ctx, func() { p.mu.Lock(); delete(p.active, id); p.mu.Unlock(); cancel(context.Canceled) }, nil
}

type transport struct {
	policy *Policy
	base   http.RoundTripper
	local  *http.Transport
}

// Wrap preserves a client's transport while enforcing the same policy on each redirect hop.
func Wrap(base http.RoundTripper) http.RoundTripper { return Default.Wrap(base) }
func (p *Policy) Wrap(base http.RoundTripper) http.RoundTripper {
	if base == nil {
		base = http.DefaultTransport
	}
	if t, ok := base.(*transport); ok && t.policy == p {
		return t
	}
	local := http.DefaultTransport.(*http.Transport).Clone()
	if configured, ok := base.(*http.Transport); ok {
		local = configured.Clone()
	}
	// Keep TLS validation against the original host while pinning socket addresses.
	local.DialTLSContext = nil
	local.DialTLS = nil
	local.DialContext = DialLocal
	local.Proxy = nil
	return &transport{policy: p, base: base, local: local}
}

func (t *transport) RoundTrip(req *http.Request) (*http.Response, error) {
	if LocalURL(req.URL) {
		return t.local.RoundTrip(req)
	}
	ctx, release, err := t.policy.Begin(req.Context())
	if err != nil {
		return nil, err
	}
	if base, ok := t.base.(*http.Transport); ok && !base.DisableKeepAlives {
		t.policy.mu.Lock()
		if !t.policy.offline {
			if t.policy.transports == nil {
				t.policy.transports = make(map[*http.Transport]struct{})
			}
			t.policy.transports[base] = struct{}{}
		}
		t.policy.mu.Unlock()
	}
	resp, err := t.base.RoundTrip(req.Clone(ctx))
	if err != nil {
		release()
		if context.Cause(ctx) == ErrOffline {
			return nil, ErrOffline
		}
		return nil, err
	}
	if resp.Body == nil {
		resp.Body = http.NoBody
	}
	resp.Body = &responseBody{ReadCloser: resp.Body, release: release, ctx: ctx}
	return resp, nil
}

type responseBody struct {
	ctx context.Context
	io.ReadCloser
	release func()
	once    sync.Once
}

// Read reports mode cancellation consistently, including a streaming body interrupted after headers.
func (b *responseBody) Read(data []byte) (int, error) {
	n, err := b.ReadCloser.Read(data)
	if err != nil {
		b.once.Do(b.release)
		if err != io.EOF && context.Cause(b.ctx) == ErrOffline {
			return n, ErrOffline
		}
	}
	return n, err
}

func (b *responseBody) Close() error { err := b.ReadCloser.Close(); b.once.Do(b.release); return err }
func (t *transport) CloseIdleConnections() {
	t.local.CloseIdleConnections()
	if c, ok := t.base.(interface{ CloseIdleConnections() }); ok {
		c.CloseIdleConnections()
	}
}

// BaseTransport exposes transport configuration to existing proxy and DNS fallback adapters.
func BaseTransport(base http.RoundTripper) http.RoundTripper {
	if t, ok := base.(*transport); ok {
		return t.base
	}
	return base
}

var listeners = struct {
	sync.Mutex
	next      uint64
	callbacks map[uint64]func(bool)
}{callbacks: make(map[uint64]func(bool))}

// Subscribe notifies adapters after policy changes; the returned function releases the subscription.
func Subscribe(callback func(bool)) func() {
	listeners.Lock()
	listeners.next++
	id := listeners.next
	listeners.callbacks[id] = callback
	listeners.Unlock()
	return func() { listeners.Lock(); delete(listeners.callbacks, id); listeners.Unlock() }
}

// Notify publishes a completed mode transition without holding policy locks.
func Notify() {
	listeners.Lock()
	callbacks := make([]func(bool), 0, len(listeners.callbacks))
	for _, cb := range listeners.callbacks {
		callbacks = append(callbacks, cb)
	}
	listeners.Unlock()
	enabled := IsOffline()
	for _, cb := range callbacks {
		cb(enabled)
	}
}

// DialLocal pins localhost to numeric loopback without consulting DNS or system proxy settings.
func DialLocal(ctx context.Context, protocol, address string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return nil, err
	}
	if strings.EqualFold(host, "localhost") {
		conn, err := (&net.Dialer{}).DialContext(ctx, protocol, net.JoinHostPort("127.0.0.1", port))
		if err == nil {
			return conn, nil
		}
		return (&net.Dialer{}).DialContext(ctx, protocol, net.JoinHostPort("::1", port))
	}
	ip := net.ParseIP(host)
	if ip == nil || !ip.IsLoopback() {
		return nil, ErrOffline
	}
	return (&net.Dialer{}).DialContext(ctx, protocol, address)
}
