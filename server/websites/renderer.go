package websites

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"io"
	"mime"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const renderResourceLimit = 8 << 20

type renderer struct {
	client *http.Client
}

func startRenderer(dir string, fetcher *http.Client) (*renderer, error) {
	socket := filepath.Join(dir, "fetch.sock")
	if err := os.Remove(socket); err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	listener, err := net.Listen("unix", socket)
	if err != nil {
		return nil, err
	}
	if err := os.Chmod(socket, 0o666); err != nil {
		listener.Close()
		return nil, err
	}
	// The renderer has no Docker network. The only network path for its page resources is this socket.
	client := *fetcher
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	client.Timeout = 50 * time.Second
	slots := make(chan struct{}, 16)
	broker := &http.Server{
		ReadHeaderTimeout: 3 * time.Second,
		WriteTimeout:      65 * time.Second,
		MaxHeaderBytes:    8 << 10,
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			select {
			case slots <- struct{}{}:
				defer func() { <-slots }()
			case <-r.Context().Done():
				return
			}
			target, err := url.Parse(r.URL.Query().Get("url"))
			if (r.Method != http.MethodGet && r.Method != http.MethodPost && r.Method != http.MethodHead) || err != nil || target.Hostname() == "" ||
				target.User != nil || (target.Scheme != "http" && target.Scheme != "https") {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			var requestBody io.Reader
			if r.Method == http.MethodPost {
				data, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 1<<20))
				if err != nil {
					w.WriteHeader(http.StatusRequestEntityTooLarge)
					return
				}
				requestBody = bytes.NewReader(data)
			}
			req, err := http.NewRequestWithContext(r.Context(), r.Method, target.String(), requestBody)
			if err != nil {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			req.Header.Set("User-Agent", previewUserAgent)
			for _, header := range []string{"Cookie", "Content-Type", "Accept", "Accept-Language", "Origin", "Referer"} {
				if value := r.Header.Get(header); value != "" {
					req.Header.Set(header, value)
				}
			}
			response, err := client.Do(req)
			if err != nil {
				w.WriteHeader(http.StatusBadGateway)
				return
			}
			defer response.Body.Close()
			disposition, _, _ := mime.ParseMediaType(response.Header.Get("Content-Disposition"))
			if strings.EqualFold(disposition, "attachment") {
				w.WriteHeader(http.StatusUnsupportedMediaType)
				return
			}
			body, err := io.ReadAll(io.LimitReader(response.Body, renderResourceLimit+1))
			if err != nil || len(body) > renderResourceLimit {
				w.WriteHeader(http.StatusBadGateway)
				return
			}
			for _, header := range []string{"Content-Type", "Location", "Access-Control-Allow-Origin", "Content-Security-Policy", "Set-Cookie", "Access-Control-Allow-Credentials"} {
				for _, value := range response.Header.Values(header) {
					w.Header().Add(header, value)
				}
			}
			w.WriteHeader(response.StatusCode)
			_, _ = w.Write(body)
		}),
	}
	go func() { _ = broker.Serve(listener) }()

	return &renderer{client: &http.Client{
		Timeout: 95 * time.Second,
		Transport: &http.Transport{DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			return (&net.Dialer{}).DialContext(ctx, "unix", filepath.Join(dir, "render.sock"))
		}},
	}}, nil
}

func (b *renderer) capture(ctx context.Context, target string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://renderer/?url="+url.QueryEscape(target), nil)
	if err != nil {
		return "", err
	}
	response, err := b.client.Do(req)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK || response.Header.Get("Content-Type") != "image/jpeg" {
		return "", errors.New("website renderer did not produce an image")
	}
	image, err := io.ReadAll(io.LimitReader(response.Body, previewLimit+1))
	if err != nil || len(image) == 0 || len(image) > previewLimit {
		return "", errors.New("website image could not be read")
	}
	return "data:image/jpeg;base64," + base64.StdEncoding.EncodeToString(image), nil
}
