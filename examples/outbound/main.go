// Outbound HTTP with logging context (requires go-logging setup).
package main

import (
	"fmt"
	"io"
	"net/http"

	"github.com/andywang2011/go-httplib"
	"github.com/andywang2011/go-logging"
)

func main() {
	if err := logging.SetupFromConfig(logging.Config{
		Output:      logging.OutputFmt,
		AESDisabled: true,
		AsyncSync:   true,
	}); err != nil {
		panic(err)
	}
	defer logging.Close()

	ctx := logging.NewContext("req-99", "demo", "GET /httpbin/get", "fetch")
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, "https://httpbin.org/get", nil)
	client := httplib.NewClient()
	resp, err := client.Do(req)
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	fmt.Printf("status=%d bytes=%d\n", resp.StatusCode, len(body))
}
