package httplib

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/andywang2011/go-logging"
)

func TestMain(m *testing.M) {
	if err := logging.SetupFromConfig(logging.Config{
		Output:      logging.OutputStd,
		AESDisabled: true,
		AsyncSync:   true,
	}); err != nil {
		panic(err)
	}
	defer func() { _ = logging.Close() }()
	m.Run()
}

func TestNewClient(t *testing.T) {
	c := NewClient()
	if c == nil {
		t.Fatal("NewClient() returned nil")
	}
	if c.Timeout != defaultTimeout {
		t.Errorf("Timeout = %v, want %v", c.Timeout, defaultTimeout)
	}
	if c.Transport == nil {
		t.Error("Transport is nil")
	}
}

func TestNewClient_WithTimeout(t *testing.T) {
	want := 10 * time.Second
	c := NewClient(WithTimeout(want))
	if c.Timeout != want {
		t.Errorf("Timeout = %v, want %v", c.Timeout, want)
	}
}

func TestNewClient_WithMaxBodyLogBytes(t *testing.T) {
	c := NewClient(WithMaxBodyLogBytes(512))
	if c.Transport == nil {
		t.Fatal("transport nil")
	}
}

func TestNewClientFromConfig(t *testing.T) {
	cfg := ClientConfig{Timeout: 5 * time.Second}
	c := NewClientFromConfig(cfg)
	if c == nil {
		t.Fatal("NewClientFromConfig returned nil")
	}
	if c.Timeout != 5*time.Second {
		t.Errorf("Timeout = %v", c.Timeout)
	}
}

func TestNewClientFromConfig_DefaultTransport(t *testing.T) {
	cfg := ClientConfig{}
	c := NewClientFromConfig(cfg)
	if c.Transport == nil {
		t.Error("Transport is nil")
	}
}

func TestDoer_ClientImplements(t *testing.T) {
	var _ Doer = (*Client)(nil)
}

func TestRoundTrip_Integration(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}))
	defer srv.Close()

	client := NewClient(WithTimeout(time.Second))
	req, _ := http.NewRequest("GET", srv.URL, nil)
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("Do err = %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("StatusCode = %d", resp.StatusCode)
	}
}

// spyRoundTripper records whether the inner transport saw a request body.
type spyRoundTripper struct {
	inner   http.RoundTripper
	gotBody atomic.Value // []byte
}

func (s *spyRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.Body != nil {
		b, err := io.ReadAll(req.Body)
		if err != nil {
			return nil, err
		}
		s.gotBody.Store(b)
		req.Body = io.NopCloser(strings.NewReader(string(b)))
	}
	if s.inner != nil {
		return s.inner.RoundTrip(req)
	}
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Length": []string{"2"}},
		Body:       io.NopCloser(strings.NewReader("ok")),
	}, nil
}

func TestTransport_MetadataMode_DoesNotConsumeRequestBodyBeforeInner(t *testing.T) {
	spy := &spyRoundTripper{}
	rt := newLogTransport(spy, 0)
	client := &http.Client{Transport: rt}

	body := strings.NewReader("payload-bytes")
	req, err := http.NewRequest(http.MethodPost, "https://example.test/path", body)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()

	got, _ := spy.gotBody.Load().([]byte)
	if string(got) != "payload-bytes" {
		t.Fatalf("inner transport saw body %q, want full payload (metadata mode must not strip body)", string(got))
	}
}

func TestTransport_DebugMode_ReadsAndRestoresBodies(t *testing.T) {
	spy := &spyRoundTripper{}
	rt := newLogTransport(spy, 100)
	client := &http.Client{Transport: rt}

	req, err := http.NewRequest(http.MethodPost, "https://example.test/path", strings.NewReader("hello"))
	if err != nil {
		t.Fatal(err)
	}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != "ok" {
		t.Fatalf("response body = %q", string(b))
	}
	got, _ := spy.gotBody.Load().([]byte)
	if string(got) != "hello" {
		t.Fatalf("inner saw %q", string(got))
	}
}

func TestRedactHeaders_Authorization(t *testing.T) {
	h := http.Header{"Authorization": []string{"Bearer secret"}, "X-Ok": []string{"1"}}
	r := redactHeaders(h)
	if r.Get("Authorization") != "***" {
		t.Fatalf("Authorization not redacted: %q", r.Get("Authorization"))
	}
	if r.Get("X-Ok") != "1" {
		t.Fatal("innocent header modified")
	}
}

func TestRedactQuery_sensitiveKey(t *testing.T) {
	q := map[string][]string{"token": {"secret"}, "page": {"1"}}
	r := redactQuery(q)
	if r.Get("token") != "***" {
		t.Fatalf("token = %q", r.Get("token"))
	}
	if r.Get("page") != "1" {
		t.Fatal("innocent query modified")
	}
}

// requestLogMeta is shared by metadata-only and body-logging modes, so redaction
// can no longer be disabled by enabling body logs.
func TestRequestLogMeta_RedactsSensitiveQuery(t *testing.T) {
	req, err := http.NewRequest(http.MethodGet, "https://example.test/p?token=abc&secret=s3&page=1", nil)
	if err != nil {
		t.Fatal(err)
	}
	raw := fmt.Sprint(requestLogMeta(req)["query"])
	if strings.Contains(raw, "abc") || strings.Contains(raw, "s3") {
		t.Fatalf("sensitive query value leaked: %s", raw)
	}
	if !strings.Contains(raw, "1") {
		t.Fatalf("innocent query missing: %s", raw)
	}
}

func TestTransport_BodyLogMode_StillRoundTripsWithSensitiveQuery(t *testing.T) {
	spy := &spyRoundTripper{}
	client := &http.Client{Transport: newLogTransport(spy, 100)}
	req, err := http.NewRequest(http.MethodPost, "https://example.test/p?access_token=zzz", strings.NewReader("hi"))
	if err != nil {
		t.Fatal(err)
	}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	if got, _ := spy.gotBody.Load().([]byte); string(got) != "hi" {
		t.Fatalf("inner saw %q", got)
	}
}

func TestRedactQuery_Nil(t *testing.T) {
	if redactQuery(nil) != nil {
		t.Fatal("expected nil")
	}
}

func TestLogLabel(t *testing.T) {
	req, _ := http.NewRequest(http.MethodGet, "https://api.example.com/v1/users", nil)
	if got := logLabel(req); got != "api.example.com/v1/users" {
		t.Fatalf("logLabel = %q", got)
	}
}

func TestNewTransport_and_DebugTransport(t *testing.T) {
	if NewTransport(nil) == nil {
		t.Fatal("NewTransport nil")
	}
	if NewDebugTransport(nil, 0) == nil {
		t.Fatal("NewDebugTransport nil")
	}
}

func TestRoundTrip_WithLoggingContext(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	ctx := logging.NewContext("req-1", "outbound", srv.URL, "test")
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL, nil)
	if err != nil {
		t.Fatal(err)
	}
	client := NewClient(WithTimeout(time.Second))
	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
}
