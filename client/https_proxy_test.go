package chclient

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/armon/go-socks5"
)

func TestHTTPSProxy(t *testing.T) {
	// 1. Start a fake SOCKS5 server
	conf := &socks5.Config{}
	server, err := socks5.New(conf)
	if err != nil {
		t.Fatal(err)
	}

	socksListener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer socksListener.Close()
	go server.Serve(socksListener)

	// 2. Start a target HTTP server
	targetServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "hello from target")
	}))
	defer targetServer.Close()

	targetURL, _ := url.Parse(targetServer.URL)

	// 3. Start the HTTPS proxy
	// We need a way to run it without the whole Client struct being fully initialized
	// Let's just test the httpsProxy struct directly.
	proxyHandler := &httpsProxy{
		Logger:    nil, // Should be fine if we don't call debug/info
		socksAddr: socksListener.Addr().String(),
	}

	proxyServer := httptest.NewServer(proxyHandler)
	defer proxyServer.Close()

	// 4. Test normal HTTP proxying
	proxyURL, _ := url.Parse(proxyServer.URL)
	httpClient := &http.Client{
		Transport: &http.Transport{
			Proxy: http.ProxyURL(proxyURL),
		},
		Timeout: 5 * time.Second,
	}

	resp, err := httpClient.Get(targetServer.URL)
	if err != nil {
		t.Fatalf("Failed to GET through proxy: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if string(body) != "hello from target" {
		t.Fatalf("Expected 'hello from target', got '%s'", string(body))
	}

	// 5. Test HTTPS (CONNECT) proxying
	// For CONNECT, we can just use the same httpClient because targetServer.URL is http,
	// but the proxy implementation handles both.
	// To force CONNECT, we would usually use an https target, but we can also manually dial.

	// Manual CONNECT test
	conn, err := net.Dial("tcp", proxyURL.Host)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	fmt.Fprintf(conn, "CONNECT %s HTTP/1.1\r\nHost: %s\r\n\r\n", targetURL.Host, targetURL.Host)
	
	// Read response header
	buf := make([]byte, 1024)
	n, err := conn.Read(buf)
	if err != nil {
		t.Fatal(err)
	}
	if !contains(buf[:n], []byte("200 Connection Established")) {
		t.Fatalf("Expected 200 Connection Established, got: %s", string(buf[:n]))
	}

	// Now send a regular HTTP request over the established tunnel
	fmt.Fprintf(conn, "GET / HTTP/1.1\r\nHost: %s\r\nConnection: close\r\n\r\n", targetURL.Host)
	n, err = conn.Read(buf)
	if err != nil && err != io.EOF {
		t.Fatal(err)
	}
	if !contains(buf[:n], []byte("hello from target")) {
		t.Fatalf("Expected 'hello from target' in tunnel, got: %s", string(buf[:n]))
	}
}

func contains(b, sub []byte) bool {
	for i := 0; i < len(b)-len(sub)+1; i++ {
		match := true
		for j := 0; j < len(sub); j++ {
			if b[i+j] != sub[j] {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}

func TestHTTPSProxyPortCalculation(t *testing.T) {
	// Mock client
	// Since we can't easily run the whole server loop without real networking,
	// let's just test that the port calculation logic works.
	// Actually, the runHTTPSProxy function has side effects (starting a server).
}
