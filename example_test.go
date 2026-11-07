package httplib_test

import (
	"fmt"
	"time"

	"github.com/andywang2011/go-httplib"
)

func ExampleNewClient() {
	client := httplib.NewClient(httplib.WithTimeout(5 * time.Second))
	fmt.Println(client.Timeout > 0)
	// Output: true
}

func ExampleNewClientFromConfig() {
	client := httplib.NewClientFromConfig(httplib.ClientConfig{
		Timeout:         10 * time.Second,
		MaxBodyLogBytes: 0,
	})
	fmt.Println(client.Timeout)
	// Output: 10s
}

func ExampleNewTransport() {
	rt := httplib.NewTransport(nil)
	fmt.Println(rt != nil)
	// Output: true
}
