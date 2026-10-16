package chclient

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/jpillora/chisel/share/cio"
	"golang.org/x/net/proxy"
)

// runHTTPSProxy starts an HTTPS proxy that forwards to a SOCKS5 proxy
func (c *Client) runHTTPSProxy(ctx context.Context, httpsHost, httpsPort, socksHost, socksPort string) error {
	httpsAddr := net.JoinHostPort(httpsHost, httpsPort)
	socksAddr := net.JoinHostPort(socksHost, socksPort)

	// Create a SOCKS5 dialer that connects to the local SOCKS5 proxy
	socksDialer, err := proxy.SOCKS5("tcp", socksAddr, nil, proxy.Direct)
	if err != nil {
		return fmt.Errorf("failed to create SOCKS5 dialer: %w", err)
	}

	// Create HTTP server with custom dialer
	server := &http.Server{
		Addr: httpsAddr,
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			c.handleHTTPSProxy(w, r, socksDialer)
		}),
		ReadHeaderTimeout: 10 * time.Second,
	}

	c.Infof("HTTPS proxy listening on %s (forwarding to SOCKS5 %s)", httpsAddr, socksAddr)

	// Start server
	go func() {
		<-ctx.Done()
		server.Close()
	}()

	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return err
	}
	return nil
}

// handleHTTPSProxy handles CONNECT requests and forwards through SOCKS5
func (c *Client) handleHTTPSProxy(w http.ResponseWriter, r *http.Request, dialer proxy.Dialer) {
	if r.Method != http.MethodConnect {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Hijack the connection
	hijacker, ok := w.(http.Hijacker)
	if !ok {
		http.Error(w, "Hijacking not supported", http.StatusInternalServerError)
		return
	}

	clientConn, _, err := hijacker.Hijack()
	if err != nil {
		c.Debugf("Hijack failed: %v", err)
		return
	}
	defer clientConn.Close()

	// Send 200 Connection Established
	if _, err := clientConn.Write([]byte("HTTP/1.1 200 Connection Established\r\n\r\n")); err != nil {
		c.Debugf("Failed to write 200 response: %v", err)
		return
	}

	// Connect to target through SOCKS5
	targetConn, err := dialer.Dial("tcp", r.Host)
	if err != nil {
		c.Debugf("SOCKS5 dial failed: %v", err)
		return
	}
	defer targetConn.Close()

	// Pipe data between client and target
	cio.Pipe(clientConn, targetConn)
	c.Debugf("HTTPS proxy connection closed for %s", r.Host)
}