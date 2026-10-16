package chclient

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"time"

	"github.com/jpillora/chisel/share/cio"
	"golang.org/x/net/proxy"
)

type httpsProxy struct {
	*cio.Logger
	socksAddr string
}

func (p *httpsProxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodConnect {
		p.handleConnect(w, r)
		return
	}
	p.handleHTTP(w, r)
}

func (p *httpsProxy) handleConnect(w http.ResponseWriter, r *http.Request) {
	hijacker, ok := w.(http.Hijacker)
	if !ok {
		http.Error(w, "Hijacking not supported", http.StatusInternalServerError)
		return
	}

	clientConn, _, err := hijacker.Hijack()
	if err != nil {
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}
	defer clientConn.Close()

	// Dial the destination via SOCKS5
	dialer, err := proxy.SOCKS5("tcp", p.socksAddr, nil, proxy.Direct)
	if err != nil {
		p.Debugf("SOCKS5 dialer error: %v", err)
		return
	}

	targetConn, err := dialer.Dial("tcp", r.URL.Host)
	if err != nil {
		p.Debugf("SOCKS5 dial error: %v", err)
		return
	}
	defer targetConn.Close()

	clientConn.Write([]byte("HTTP/1.1 200 Connection Established\r\n\r\n"))

	cio.Pipe(clientConn, targetConn)
}

func (p *httpsProxy) handleHTTP(w http.ResponseWriter, r *http.Request) {
	// Dial the destination via SOCKS5
	dialer, err := proxy.SOCKS5("tcp", p.socksAddr, nil, proxy.Direct)
	if err != nil {
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}

	transport := &http.Transport{
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			return dialer.Dial(network, addr)
		},
	}

	outReq := new(http.Request)
	*outReq = *r
	outReq.RequestURI = ""

	// Remove hop-by-hop headers
	removeHopByHop(outReq.Header)

	resp, err := transport.RoundTrip(outReq)
	if err != nil {
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}
	defer resp.Body.Close()

	removeHopByHop(resp.Header)
	copyHeader(w.Header(), resp.Header)
	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)
}

func copyHeader(dst, src http.Header) {
	for k, vv := range src {
		for _, v := range vv {
			dst.Add(k, v)
		}
	}
}

func removeHopByHop(header http.Header) {
	hopByHop := []string{
		"Connection",
		"Keep-Alive",
		"Proxy-Authenticate",
		"Proxy-Authorization",
		"Te",
		"Trailers",
		"Transfer-Encoding",
		"Upgrade",
	}
	for _, h := range hopByHop {
		header.Del(h)
	}
}

func (c *Client) runHTTPSProxy(ctx context.Context, socksHost, socksPort string) error {
	httpsPortInt, _ := net.LookupPort("tcp", socksPort)
	httpsPort := httpsPortInt + 1
	httpsAddr := net.JoinHostPort(socksHost, fmt.Sprintf("%d", httpsPort))
	socksAddr := net.JoinHostPort(socksHost, socksPort)

	p := &httpsProxy{
		Logger:    c.Logger.Fork("https-proxy"),
		socksAddr: socksAddr,
	}

	server := &http.Server{
		Addr:    httpsAddr,
		Handler: p,
	}

	c.Infof("HTTPS proxy listening on %s (forwarding to SOCKS5 %s)", httpsAddr, socksAddr)

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		server.Shutdown(shutdownCtx)
	}()

	if err := server.ListenAndServe(); err != http.ErrServerClosed {
		return err
	}
	return nil
}
