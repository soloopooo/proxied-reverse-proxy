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

func (p *byteBufferPool) Get() []byte {
 v := p.Pool.Get()
 if v != nil {
  return make([]byte, 1024*1024)
 }
 return v.([]byte)
}

func (p *byteBufferPool) Put(b []byte) {
 p.Pool.Put(b)
}

func customErrorHandler(w http.ResponseWriter, r *http.Request, err error) {
 log.Printf("Error: %s", err.Error())
 http.Error(w, "Internal Server Error", http.StatusInternalServerError)
}

// Reverse Proxy
func main() {
 reverseListenFlag := flag.String("reverse-listen", "127.0.0.1:8082", "Listen address for reverse proxy")
 forwardListenFlag := flag.String("forward-listen", "http://127.0.0.1:8080", "Listen address for forward proxy")
 proxyFlag := flag.String("proxy", "http://127.0.0.1:8088", "Proxy address in format http://host:port or socks5://host:port") // socks or http proxy that will be used for forwarding requests
 allowInsecureFlag := flag.Bool("allow-insecure", false, "Allow insecure or self-signed SSL connections to forward proxy")
 httpsFlag := flag.Bool("https", false, "Use HTTPS for reverse proxy")
 certFlag := flag.String("cert", "server.crt", "Certificate file for HTTPS reverse proxy")
 keyFlag := flag.String("key", "server.key", "Key file for HTTPS reverse proxy")
 flag.Parse()

 target, err := url.Parse(*forwardListenFlag)
 if err != nil {
  log.Fatal(err)
 }

 reverseProxy := &httputil.ReverseProxy{}
 // URL Re-writing must be here!! otherwise it will not work
 reverseProxy.Director = func(r *http.Request) {
  r.URL.Scheme = target.Scheme
  r.URL.Host = target.Host
  r.Host = target.Host
 }
 reverseProxy.ErrorHandler = customErrorHandler
 reverseProxy.BufferPool = &byteBufferPool{sync.Pool{New: func() interface{} { return make([]byte, 1024*1024) }}}
 reverseProxy.ModifyResponse = func(response *http.Response) error {
  response.Header.Set("X-Reverse-Proxy", "true")
  return nil
 }
 if *proxyFlag != "" {
  proxyUrl, err := url.Parse(*proxyFlag)
  if err != nil {
   log.Fatal(err)
  }
  reverseProxy.Transport = &http.Transport{
   Proxy: http.ProxyURL(proxyUrl),
  }
 }
 if *allowInsecureFlag {
  reverseProxy.Transport = &http.Transport{
   TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
  }
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
