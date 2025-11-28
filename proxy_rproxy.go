package main

import (
	"crypto/tls"
	"flag"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"sync"
)

type byteBufferPool struct {
	sync.Pool
}
type byteSliceWrapper struct {
	b []byte
}

func (p *byteBufferPool) Get() []byte {
	v := p.Pool.Get()
	if v == nil {
		return make([]byte, 1024*1024)
	}
	// 断言为 *byteSliceWrapper 类型
	wrapper, ok := v.(*byteSliceWrapper)
	if !ok {
		// 如果类型断言失败，返回一个新的切片
		return make([]byte, 1024*1024)
	}
	return wrapper.b
}

func (p *byteBufferPool) Put(b []byte) {
	p.Pool.Put(&byteSliceWrapper{b})
}

func customErrorHandler(w http.ResponseWriter, r *http.Request, err error) {
	log.Printf("Error: %s", err.Error())
	http.Error(w, "Internal Server Error", http.StatusInternalServerError)
}

// Reverse Proxy
func main() {
	reverseListenFlag := flag.String("reverse-listen", "127.0.0.1:8082", "Listen address for reverse proxy")
	//remoteForwardFlag := flag.String("remote-forward", "http://127.0.0.1:8080", "Remote services that will be proxied")
	proxyFlag := flag.String("proxy", "http://127.0.0.1:8088", "Proxy address in format http://host:port or socks5://host:port") // socks or http proxy that will be used for forwarding requests
	allowInsecureFlag := flag.Bool("allow-insecure", false, "Allow insecure or self-signed SSL connections to forward proxy")
	httpsFlag := flag.Bool("https", false, "Use HTTPS for reverse proxy")
	certFlag := flag.String("cert", "server.crt", "Certificate file for HTTPS reverse proxy")
	keyFlag := flag.String("key", "server.key", "Key file for HTTPS reverse proxy")
	flag.Parse()

	reverseProxy := &httputil.ReverseProxy{
		Director: func(r *http.Request) {},
		Transport: &SNIAwareTransport{
			ProxyURL:      getProxyURL(*proxyFlag),
			AllowInsecure: *allowInsecureFlag,
		},
		ErrorHandler: customErrorHandler,
		BufferPool:   &byteBufferPool{sync.Pool{New: func() interface{} { return make([]byte, 1024*1024) }}},
		ModifyResponse: func(response *http.Response) error {
			response.Header.Set("X-Reverse-Proxy", "true")
			return nil
		},
	}

	var reverseServer *http.Server
	if *httpsFlag {
		reverseServer = &http.Server{
			Addr:      *reverseListenFlag,
			Handler:   reverseProxy,
			TLSConfig: &tls.Config{
				// Use only for testing purposes, do not use in production
				//Certificates: []tls.Certificate{cert},
			},
		}
		log.Printf("Starting reverse proxy on %s with HTTPS", *reverseListenFlag)
		if err := reverseServer.ListenAndServeTLS(*certFlag, *keyFlag); err != nil {
			log.Fatal(err)
		}
	} else {
		reverseServer = &http.Server{
			Addr:    *reverseListenFlag,
			Handler: reverseProxy,
		}
	}

	log.Printf("Starting reverse proxy on %s", *reverseListenFlag)
	if err := reverseServer.ListenAndServe(); err != nil {
		log.Fatal(err)
	}
}

type SNIAwareTransport struct {
	ProxyURL      *url.URL
	AllowInsecure bool
	baseTransport http.RoundTripper
}

func (t *SNIAwareTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if sni := req.TLS.ServerName; sni != "" {
		target, err := url.Parse("https://" + sni)
		log.Println(target)
		if err != nil {
			log.Printf("WARN: failed to parse target URL from SNI %s: %v", sni, err)
			return nil, err
		}
		req.URL.Scheme = target.Scheme
		req.URL.Host = target.Host
		req.Host = target.Host
	}
	return t.getBaseTransport().RoundTrip(req)
}
func (t *SNIAwareTransport) getBaseTransport() http.RoundTripper {
	if t.baseTransport != nil {
		return t.baseTransport
	}
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: t.AllowInsecure},
	}
	if t.ProxyURL != nil {
		transport.Proxy = http.ProxyURL(t.ProxyURL)
	}
	t.baseTransport = transport
	return transport
}

func getProxyURL(proxy string) *url.URL {
	if proxy == "" {
		return nil
	}
	proxyUrl, err := url.Parse(proxy)
	if err != nil {
		log.Fatal(err)
	}
	return proxyUrl
}
