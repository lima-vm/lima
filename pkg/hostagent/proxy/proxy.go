// This file has been adapted from https://github.com/elazarl/goproxy/blob/6741dbfc16a1/examples/goproxy-eavesdropper/main.go

package proxy

import (
	"bufio"
	"context"
	"net"
	"net/http"
	"regexp"
	"strconv"
	"time"

	"github.com/elazarl/goproxy"
	"github.com/sirupsen/logrus"
)

// CACert has the CA certificate text.
var CACert = string(goproxy.CA_CERT)

type ServerOptions struct {
	Address string
	TCPPort int
}

type Server struct {
	srv *http.Server
}

func (s *Server) Shutdown() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if s.srv != nil {
		_ = s.srv.Shutdown(ctx)
	}
}

func Start(opts ServerOptions) (*Server, error) {
	server := &Server{}
	if opts.TCPPort > 0 {
		srv, err := listenAndServe(opts)
		if err != nil {
			return nil, err
		}
		server.srv = srv
	}
	return server, nil
}

func listenAndServe(opts ServerOptions) (*http.Server, error) {
	addr := net.JoinHostPort(opts.Address, strconv.Itoa(opts.TCPPort))
	proxy := goproxy.NewProxyHttpServer()
	proxy.OnRequest(goproxy.ReqHostMatches(regexp.MustCompile("^.*$"))).
		HandleConnect(goproxy.AlwaysMitm)
	proxy.OnRequest(goproxy.ReqHostMatches(regexp.MustCompile("^.*:80$"))).
		HijackConnect(func(req *http.Request, client net.Conn, ctx *goproxy.ProxyCtx) {
			defer func() {
				if e := recover(); e != nil {
					ctx.Logf("error connecting to remote: %v", e)
					_, _ = client.Write([]byte("HTTP/1.1 500 Cannot reach destination\r\n\r\n"))
				}
				client.Close()
			}()
			url := req.URL.String()
			ctx.Logf("URL: %s", url)
			clientBuf := bufio.NewReadWriter(bufio.NewReader(client), bufio.NewWriter(client))
			remote, err := net.Dial("tcp", req.URL.Host)
			if err != nil {
				ctx.Logf("%v", err)
				return
			}
			_, _ = client.Write([]byte("HTTP/1.1 200 Ok\r\n\r\n"))
			remoteBuf := bufio.NewReadWriter(bufio.NewReader(remote), bufio.NewWriter(remote))
			for {
				req, err := http.ReadRequest(clientBuf.Reader)
				if err != nil {
					ctx.Logf("%v", err)
					return
				}
				_ = req.Write(remoteBuf)
				_ = remoteBuf.Flush()
				resp, err := http.ReadResponse(remoteBuf.Reader, req)
				if err != nil {
					ctx.Logf("%v", err)
					return
				}
				_ = resp.Write(clientBuf.Writer)
				_ = clientBuf.Flush()
				resp.Body.Close()
			}
		})
	proxy.Verbose = true
	s := &http.Server{Addr: addr, Handler: proxy}
	go func() {
		logrus.Debugf("Start HTTP proxy listening on: %v", addr)
		if e := s.ListenAndServe(); e != nil {
			logrus.Fatal(e)
		}
	}()

	return s, nil
}
