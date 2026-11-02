package app

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/shellhub-io/shellhub/pkg/cache"
	cachemock "github.com/shellhub-io/shellhub/pkg/cache/mocks"
	"github.com/shellhub-io/shellhub/pkg/envs"
	"github.com/shellhub-io/shellhub/server/api/services"
	"github.com/shellhub-io/shellhub/server/api/store"
	storemock "github.com/shellhub-io/shellhub/server/api/store/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestLicenseEvaluatorWiring verifies the license-evaluator factory wiring in server setup.
// It exercises licenseEvaluatorOption, the helper extracted from Setup() so the logic is
// testable without a live database or Redis connection.
func TestLicenseEvaluatorWiring(t *testing.T) {
	original := services.LicenseEvaluatorFactory()
	t.Cleanup(func() { services.RegisterLicenseEvaluator(original) })

	t.Run("no factory registered: returns empty option slice", func(t *testing.T) {
		services.RegisterLicenseEvaluator(nil)

		s := &Server{env: &Env{}}
		mockStore := storemock.NewMockStore(t)
		mockCache := cachemock.NewMockCache(t)

		opts, err := s.licenseEvaluatorOption(context.Background(), mockStore, mockCache)
		require.NoError(t, err)
		assert.Empty(t, opts)
	})

	t.Run("factory returns non-nil evaluator: option is appended", func(t *testing.T) {
		stub := &stubLicenseEvaluator{}

		services.RegisterLicenseEvaluator(func(_ context.Context, _ store.Store, _ cache.Cache) (services.LicenseEvaluator, error) {
			return stub, nil
		})

		s := &Server{env: &Env{}}
		mockStore := storemock.NewMockStore(t)
		mockCache := cachemock.NewMockCache(t)

		opts, err := s.licenseEvaluatorOption(context.Background(), mockStore, mockCache)
		require.NoError(t, err)
		assert.Len(t, opts, 1)
	})

	t.Run("factory returns untyped nil evaluator: option is NOT appended", func(t *testing.T) {
		services.RegisterLicenseEvaluator(func(_ context.Context, _ store.Store, _ cache.Cache) (services.LicenseEvaluator, error) {
			return nil, nil
		})

		s := &Server{env: &Env{}}
		mockStore := storemock.NewMockStore(t)
		mockCache := cachemock.NewMockCache(t)

		opts, err := s.licenseEvaluatorOption(context.Background(), mockStore, mockCache)
		require.NoError(t, err)
		assert.Empty(t, opts)
	})

	t.Run("factory returns error: error is propagated", func(t *testing.T) {
		factoryErr := errors.New("license init failed")

		services.RegisterLicenseEvaluator(func(_ context.Context, _ store.Store, _ cache.Cache) (services.LicenseEvaluator, error) {
			return nil, factoryErr
		})

		s := &Server{env: &Env{}}
		mockStore := storemock.NewMockStore(t)
		mockCache := cachemock.NewMockCache(t)

		opts, err := s.licenseEvaluatorOption(context.Background(), mockStore, mockCache)
		require.Error(t, err)
		assert.Empty(t, opts)
	})
}

type stubLicenseEvaluator struct{}

func (s *stubLicenseEvaluator) CanAcceptDevice(_ context.Context) (bool, error) {
	return true, nil
}

func (s *stubLicenseEvaluator) CanConnectDevice(_ context.Context) (bool, error) {
	return true, nil
}

func TestInstanceIssuer(t *testing.T) {
	cases := []struct {
		description string
		env         *Env
		expected    string
	}{
		{
			description: "derives http from the domain when the gateway serves plain HTTP",
			env:         &Env{Domain: "localhost"},
			expected:    "http://localhost",
		},
		{
			description: "derives https from the domain when the gateway terminates TLS",
			env:         &Env{Domain: "cloudmatehub.example.com", AutoSSL: true},
			expected:    "https://cloudmatehub.example.com",
		},
	}

	for _, tc := range cases {
		t.Run(tc.description, func(t *testing.T) {
			s := &Server{env: tc.env}

			assert.Equal(t, tc.expected, s.instanceIssuer())
		})
	}
}

func TestTunnelPingInterval(t *testing.T) {
	parse := func(values map[string]string) (*sshEnv, error) {
		return envs.ParseWithPrefix[sshEnv](envs.Map(values), "SSH_")
	}

	t.Run("is zero when unset, leaving each transport its own interval", func(t *testing.T) {
		for _, values := range []map[string]string{{}, {"SHELLHUB_TUNNEL_PING_INTERVAL": ""}} {
			env, err := parse(values)
			require.NoError(t, err)
			assert.Zero(t, env.TunnelPingInterval)
		}
	})

	for value, interval := range map[string]time.Duration{"15s": 15 * time.Second, "59s": 59 * time.Second} {
		t.Run("honours "+value, func(t *testing.T) {
			env, err := parse(map[string]string{"SHELLHUB_TUNNEL_PING_INTERVAL": value})
			require.NoError(t, err)
			assert.Equal(t, interval, time.Duration(env.TunnelPingInterval))
		})
	}

	for _, value := range []string{"0s", "-1s", "soon", "1m", "3m"} {
		t.Run("refuses "+value+" naming the variable", func(t *testing.T) {
			_, err := parse(map[string]string{"SHELLHUB_TUNNEL_PING_INTERVAL": value})
			require.ErrorIs(t, err, envs.ErrParseWithPrefix)
			assert.ErrorContains(t, err, "SHELLHUB_TUNNEL_PING_INTERVAL")
		})
	}
}

func TestAccountLockoutConfiguration(t *testing.T) {
	parse := func(values map[string]string) (*Env, error) {
		return envs.ParseWithPrefix[Env](envs.Map(values), "API_")
	}

	t.Run("defaults to a one minute base capped at 60 minutes", func(t *testing.T) {
		env, err := parse(map[string]string{})
		require.NoError(t, err)
		assert.Equal(t, cache.LockoutBase(time.Minute), env.Cache.AccountLockoutBase)
		assert.Equal(t, 60, env.Cache.MaximumAccountLockout)
	})

	for value, base := range map[string]time.Duration{"1s": time.Second, "5s": 5 * time.Second, "2m": 2 * time.Minute} {
		t.Run("honours a base of "+value, func(t *testing.T) {
			env, err := parse(map[string]string{"ACCOUNT_LOCKOUT_BASE": value})
			require.NoError(t, err)
			assert.Equal(t, cache.LockoutBase(base), env.Cache.AccountLockoutBase)
		})
	}

	for _, value := range []string{"999ms", "0s", "-1s", "5"} {
		t.Run("refuses a base of "+value+" naming the variable", func(t *testing.T) {
			_, err := parse(map[string]string{"ACCOUNT_LOCKOUT_BASE": value})
			require.ErrorIs(t, err, envs.ErrParseWithPrefix)
			assert.ErrorContains(t, err, "SHELLHUB_ACCOUNT_LOCKOUT_BASE")
		})
	}

	t.Run("still reads the maximum as whole minutes", func(t *testing.T) {
		env, err := parse(map[string]string{"MAXIMUM_ACCOUNT_LOCKOUT": "30"})
		require.NoError(t, err)
		assert.Equal(t, 30, env.Cache.MaximumAccountLockout)
	})
}

func TestConsoleHostUsesPublishedBrowserPort(t *testing.T) {
	for _, tc := range []struct {
		name, domain, httpPort, httpsPort, want string
		tls                                     bool
	}{
		{name: "LAN HTTP", domain: "192.168.1.103", httpPort: "8080", want: "192.168.1.103:8080"},
		{name: "standard HTTP", domain: "hub.example.com", httpPort: "80", want: "hub.example.com"},
		{name: "mapped HTTPS", domain: "hub.example.com", httpsPort: "8443", tls: true, want: "hub.example.com:8443"},
		{name: "standard HTTPS", domain: "hub.example.com", httpsPort: "443", tls: true, want: "hub.example.com"},
		{name: "explicit port", domain: "hub.example.com:9000", httpPort: "8080", want: "hub.example.com:9000"},
		{name: "IPv6", domain: "::1", httpPort: "8080", want: "[::1]:8080"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := &Server{env: &Env{Domain: tc.domain, HTTPPort: tc.httpPort, HTTPSPort: tc.httpsPort, AutoSSL: tc.tls}}
			assert.Equal(t, tc.want, s.consoleHost())
			assert.NotContains(t, s.instanceIssuer(), ":8080", "published browser ports must not change existing token issuers")
		})
	}
}
