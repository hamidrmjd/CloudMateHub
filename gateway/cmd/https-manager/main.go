package main

import (
	"context"
	"crypto/tls"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/shellhub-io/shellhub/gateway/httpsmanager"
)

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
func main() {
	if len(os.Args) > 1 && os.Args[1] == "recovery-url" {
		key, err := os.ReadFile(filepath.Join(env("HTTPS_STATE_DIR", "/data"), "recovery.key"))
		if err != nil {
			log.Fatal("Start the HTTPS controller first")
		}
		fmt.Println("http://localhost:9080/#" + strings.TrimSpace(string(key)))
		return
	}

	m, err := httpsmanager.New(httpsmanager.Config{Dir: env("HTTPS_STATE_DIR", "/data"), Host: env("SHELLHUB_DOMAIN", "localhost"), HTTPSPort: env("SHELLHUB_HTTPS_PORT", "8443"), TrialPort: env("CLOUDMATEHUB_HTTPS_TRIAL_PORT", "9443"), Upstream: env("HTTPS_UPSTREAM", "http://gateway:80"), Identity: env("HTTPS_IDENTITY", "http://server:8080")})
	if err != nil {
		log.Fatal("HTTPS storage initialization failed; check the mounted state and master key: ", err)
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	go m.Watch(ctx)
	recovery := &http.Server{Addr: ":9000", Handler: http.HandlerFunc(m.Recovery), ReadHeaderTimeout: 10 * time.Second}
	plain := &http.Server{Addr: ":80", Handler: m, ReadHeaderTimeout: 10 * time.Second}
	trial := &http.Server{Handler: m.TrialHandler(), ReadHeaderTimeout: 10 * time.Second}
	secure := &http.Server{Handler: m, ReadHeaderTimeout: 10 * time.Second}
	// Bind all listeners before accepting requests; recovery never depends on a certificate.
	r, err := net.Listen("tcp", recovery.Addr)
	if err != nil {
		log.Fatal(err)
	}
	p, err := net.Listen("tcp", plain.Addr)
	if err != nil {
		log.Fatal(err)
	}
	s, err := net.Listen("tcp", ":443")
	if err != nil {
		log.Fatal(err)
	}
	t, err := net.Listen("tcp", ":444")
	if err != nil {
		log.Fatal(err)
	}
	go trial.Serve(tls.NewListener(t, m.TLSConfig(true)))
	go recovery.Serve(r)
	go plain.Serve(p)
	go secure.Serve(tls.NewListener(s, m.TLSConfig(false)))
	log.Print("HTTPS controller and server-local recovery are ready")
	<-ctx.Done()
	shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	recovery.Shutdown(shutdown)
	plain.Shutdown(shutdown)
	secure.Shutdown(shutdown)
	trial.Shutdown(shutdown)
}
