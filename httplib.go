package httplib

import (
	"bytes"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/andywang2011/go-logging"
)

const (
	// DefaultMaxBodyLogBytes is the default body truncation limit when body logging is enabled.
	DefaultMaxBodyLogBytes = 2000
	defaultTimeout         = 3 * time.Second
)

// Doer is implemented by http.Client for dependency injection and test doubles.
type Doer interface {
	Do(*http.Request) (*http.Response, error)
}

// Client is an alias for http.Client that implements Doer.
type Client = http.Client

// Option configures NewClient.
type Option func(*clientSettings)

type clientSettings struct {
	timeout         time.Duration
	transport       http.RoundTripper
	maxBodyLogBytes int // 0: metadata-only logs (no body read)
}

type transport struct {
	base            http.RoundTripper
	maxBodyLogBytes int
}

func (t *transport) RoundTrip(req *http.Request) (*http.Response, error) {
	begin := time.Now()
	opt := logging.AttrOption{StartTime: &begin, EncryptStatus: logging.EncryptStatusDefault}

	if t.maxBodyLogBytes <= 0 {
		return t.roundTripMetadataOnly(req, opt)
	}
	return t.roundTripWithBodyLog(req, opt)
}

func (t *transport) roundTripMetadataOnly(req *http.Request, opt logging.AttrOption) (*http.Response, error) {
	input := requestLogMeta(req)
	resp, err := t.base.RoundTrip(req)
	if err != nil {
		logging.FromContext(req.Context()).ErrorL("api log: "+logLabel(req), input, err, opt)
		return resp, err
	}
	out := map[string]any{
		"status":         resp.StatusCode,
		"content_length": resp.ContentLength,
		"body_log_bytes": 0,
		"body_log_trunc": false,
	}
	logging.FromContext(req.Context()).Info("api log: "+logLabel(req), input, out, opt)
	return resp, nil
}

func (t *transport) roundTripWithBodyLog(req *http.Request, opt logging.AttrOption) (*http.Response, error) {
	cap := t.maxBodyLogBytes
	input := requestLogMeta(req)
	var rb []byte
	if req.Body != nil {
		var err error
		rb, err = io.ReadAll(io.LimitReader(req.Body, int64(cap)+1))
		if err != nil {
			logging.FromContext(req.Context()).ErrorL("api log: "+logLabel(req), input, err, opt)
			return nil, err
		}
		_ = req.Body.Close()
		trunc := len(rb) > cap
		if trunc {
			rb = rb[:cap]
		}
		req.Body = io.NopCloser(bytes.NewReader(rb))
		input["body"] = logging.Compress(rb, cap)
		input["body_truncated"] = trunc
	} else {
		input["body"] = logging.Compress(nil, cap)
	}

	resp, err := t.base.RoundTrip(req)
	if err != nil {
		logging.FromContext(req.Context()).ErrorL("api log: "+logLabel(req), input, err, opt)
		return resp, err
	}
	if resp != nil && resp.Body != nil {
		b, rerr := io.ReadAll(io.LimitReader(resp.Body, int64(cap)+1))
		if rerr != nil {
			_ = resp.Body.Close()
			logging.FromContext(req.Context()).ErrorL("api log: "+logLabel(req), input, rerr, opt)
			return resp, rerr
		}
		_ = resp.Body.Close()
		trunc := len(b) > cap
		if trunc {
			b = b[:cap]
		}
		resp.Body = io.NopCloser(bytes.NewReader(b))
		logging.FromContext(req.Context()).Info("api log: "+logLabel(req), input, map[string]any{
			"body":           logging.Compress(b, cap),
			"body_truncated": trunc,
			"status":         resp.StatusCode,
		}, opt)
		return resp, nil
	}
	logging.FromContext(req.Context()).Info("api log: "+logLabel(req), input, map[string]any{"status": resp.StatusCode}, opt)
	return resp, nil
}

func logLabel(req *http.Request) string {
	if req.URL == nil {
		return ""
	}
	return req.URL.Host + req.URL.Path
}

// requestLogMeta builds log metadata; sensitive query params and headers are always redacted.
func requestLogMeta(req *http.Request) map[string]any {
	if req.URL == nil {
		return map[string]any{"method": req.Method}
	}
	q := req.URL.Query()
	m := map[string]any{
		"method": req.Method,
		"host":   req.URL.Scheme + "://" + req.URL.Host,
		"path":   req.URL.Path,
		"query":  logging.SpreadMaps(redactQuery(q)),
		"header": logging.SpreadMaps(redactHeaders(req.Header)),
	}
	return m
}

var sensitiveQueryKeys = map[string]struct{}{
	"access_token": {}, "token": {}, "key": {}, "secret": {}, "password": {}, "refresh_token": {},
}

// redactQuery masks sensitive query values regardless of body logging mode.
func redactQuery(q url.Values) url.Values {
	if q == nil {
		return nil
	}
	out := make(url.Values, len(q))
	for k, vs := range q {
		lk := strings.ToLower(k)
		if _, bad := sensitiveQueryKeys[lk]; bad {
			out[k] = []string{"***"}
			continue
		}
		out[k] = vs
	}
	return out
}

var sensitiveHeaderKeys = map[string]struct{}{
	"authorization": {}, "cookie": {}, "x-api-key": {}, "proxy-authorization": {},
}

func redactHeaders(h http.Header) http.Header {
	if h == nil {
		return nil
	}
	out := make(http.Header, len(h))
	for k, vs := range h {
		lk := strings.ToLower(k)
		if _, bad := sensitiveHeaderKeys[lk]; bad {
			out[k] = []string{"***"}
			continue
		}
		out[k] = vs
	}
	return out
}

// NewClient returns an HTTP client with a logging RoundTripper; body logging is off by default.
func NewClient(options ...Option) *Client {
	s := &clientSettings{
		timeout:         defaultTimeout,
		transport:       http.DefaultTransport,
		maxBodyLogBytes: 0,
	}
	for _, opt := range options {
		opt(s)
	}
	return &Client{
		Timeout:   s.timeout,
		Transport: newLogTransport(s.transport, s.maxBodyLogBytes),
	}
}

// ClientConfig is a structured alternative to functional options for NewClientFromConfig.
type ClientConfig struct {
	// Timeout is the client request timeout (default 3s when zero).
	Timeout time.Duration
	// Transport is the base RoundTripper (http.DefaultTransport when nil).
	Transport http.RoundTripper
	// MaxBodyLogBytes is 0 for metadata-only logs; >0 enables truncated body logging.
	MaxBodyLogBytes int
}

// NewClientFromConfig builds a Client from cfg.
func NewClientFromConfig(cfg ClientConfig) *Client {
	rt := cfg.Transport
	if rt == nil {
		rt = http.DefaultTransport
	}
	timeout := defaultTimeout
	if cfg.Timeout > 0 {
		timeout = cfg.Timeout
	}
	return &Client{
		Timeout:   timeout,
		Transport: newLogTransport(rt, cfg.MaxBodyLogBytes),
	}
}

// NewTransport wraps tsp with metadata-only outbound logging (no body buffering).
func NewTransport(tsp http.RoundTripper) http.RoundTripper {
	if tsp == nil {
		tsp = http.DefaultTransport
	}
	return newLogTransport(tsp, 0)
}

// NewDebugTransport wraps tsp with truncated request/response body logging (maxBodyBytes, or DefaultMaxBodyLogBytes when <=0).
func NewDebugTransport(tsp http.RoundTripper, maxBodyBytes int) http.RoundTripper {
	if tsp == nil {
		tsp = http.DefaultTransport
	}
	if maxBodyBytes <= 0 {
		maxBodyBytes = DefaultMaxBodyLogBytes
	}
	return newLogTransport(tsp, maxBodyBytes)
}

func newLogTransport(tsp http.RoundTripper, maxBodyLogBytes int) http.RoundTripper {
	return &transport{base: tsp, maxBodyLogBytes: maxBodyLogBytes}
}

// WithTimeout sets the client request timeout.
func WithTimeout(timeout time.Duration) Option {
	return func(s *clientSettings) {
		if timeout > 0 {
			s.timeout = timeout
		}
	}
}

// WithTransport sets the base RoundTripper wrapped by the logging transport.
func WithTransport(t *http.Transport) Option {
	return func(s *clientSettings) {
		if t != nil {
			s.transport = t
		}
	}
}

// WithMaxBodyLogBytes enables truncated body logging when n > 0; 0 keeps metadata-only logs.
func WithMaxBodyLogBytes(n int) Option {
	return func(s *clientSettings) {
		if n > 0 {
			s.maxBodyLogBytes = n
		}
	}
}
