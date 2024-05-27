// This file has been adapted from https://github.com/elazarl/goproxy/blob/6741dbfc16a1/examples/goproxy-eavesdropper/main.go

package proxy

import (
	"bufio"
	"bytes"
	"context"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"time"

	"github.com/lima-vm/lima/pkg/downloader"

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

func sendFile(req *http.Request, path string, lastModified time.Time, contentType string) (*http.Response, error) {
	resp := &http.Response{}
	resp.Request = req
	resp.TransferEncoding = req.TransferEncoding
	resp.Header = make(http.Header)
	status := http.StatusOK
	resp.StatusCode = status
	resp.Status = http.StatusText(status)
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	resp.Body = io.NopCloser(bytes.NewBuffer(b))
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	resp.Header.Set("Content-Type", contentType)
	if !lastModified.IsZero() {
		resp.Header.Set("Last-Modified", lastModified.Format(http.TimeFormat))
	}
	resp.ContentLength = int64(len(b))
	return resp, nil
}

func listenAndServe(opts ServerOptions) (*http.Server, error) {
	ucd, err := os.UserCacheDir()
	if err != nil {
		return nil, err
	}
	cacheDir := filepath.Join(ucd, "lima")
	downloader.HideProgress = true

	addr := net.JoinHostPort(opts.Address, strconv.Itoa(opts.TCPPort))
	proxy := goproxy.NewProxyHttpServer()
	proxy.OnRequest(goproxy.ReqHostMatches(regexp.MustCompile("^.*$"))).
		HandleConnect(goproxy.AlwaysMitm)
	proxy.OnRequest().DoFunc(func(req *http.Request, _ *goproxy.ProxyCtx) (*http.Request, *http.Response) {
		url := req.URL.String()
		if res, err := downloader.Cached(url, downloader.WithCacheDir(cacheDir)); err == nil {
			if resp, err := sendFile(req, res.CachePath, res.LastModified, res.ContentType); err == nil {
				return nil, resp
			}
		}
		if res, err := downloader.Download(context.Background(), "", url, downloader.WithCacheDir(cacheDir)); err == nil {
			if resp, err := sendFile(req, res.CachePath, res.LastModified, res.ContentType); err == nil {
				return nil, resp
			}
		}
		return req, nil
	})
	proxy.OnRequest(goproxy.ReqHostMatches(regexp.MustCompile("^.*:80$"))).
		HijackConnect(func(req *http.Request, client net.Conn, ctx *goproxy.ProxyCtx) {
			defer func() {
				if e := recover(); e != nil {
					ctx.Logf("error connecting to remote: %v", e)
					_, _ = client.Write([]byte("HTTP/1.1 500 Cannot reach destination\r\n\r\n"))
				}
				client.Close()
			}()
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
			if e != http.ErrServerClosed {
				logrus.Fatal(e)
			}
		}
	}()

	return s, nil
}
