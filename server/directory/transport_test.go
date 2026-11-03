package directory

import (
	"context"
	"encoding/pem"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestTLSRequiresTrustedCertificateAndHost(t *testing.T) {
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	server.Config.ErrorLog = log.New(io.Discard, "", 0)
	server.StartTLS()
	defer server.Close()
	c := defaults("openldap")
	c.URL = "ldaps://" + server.Listener.Addr().String()
	_, err := dial(context.Background(), c)
	require.ErrorContains(t, err, "TLS validation failed")
	c.CA = string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: server.Certificate().Raw}))
	conn, err := dial(context.Background(), c)
	require.NoError(t, err)
	require.NoError(t, conn.Close())
	cfg, err := tlsConfig(c)
	require.NoError(t, err)
	require.False(t, cfg.InsecureSkipVerify)
	require.Equal(t, "127.0.0.1", cfg.ServerName)
}
func TestStartTLSCannotFallBackToPlaintextBind(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer listener.Close()
	received := make(chan string, 1)
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			received <- ""
			return
		}
		defer conn.Close()
		_ = conn.SetDeadline(time.Now().Add(3 * time.Second))
		buf := make([]byte, 4096)
		n, _ := conn.Read(buf)
		received <- string(buf[:n])
		_, _ = conn.Write([]byte("TLS refused"))
	}()
	c := defaults("openldap")
	c.URL = "ldap://" + listener.Addr().String()
	c.BindDN = "cn=reader"
	c.BindPassword = "fixture-service-secret"
	_, err = dial(context.Background(), c)
	require.ErrorContains(t, err, "StartTLS failed")
	wire := <-received
	require.True(t, strings.Contains(wire, "1.3.6.1.4.1.1466.20037"))
	require.NotContains(t, wire, c.BindDN)
	require.NotContains(t, wire, c.BindPassword)
}
