package services

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"errors"
	"testing"

	"github.com/shellhub-io/shellhub/pkg/api/authorizer"
	"github.com/shellhub-io/shellhub/pkg/cache"
	"github.com/shellhub-io/shellhub/pkg/models"
	"github.com/shellhub-io/shellhub/server/api/store"
	"github.com/shellhub-io/shellhub/server/api/store/mocks"
	"github.com/stretchr/testify/require"
)

type directoryCheck struct {
	err   error
	calls int
}

func (d *directoryCheck) ValidateUser(context.Context, *models.User) error       { d.calls++; return d.err }
func (d *directoryCheck) VerifyUser(context.Context, *models.User, string) error { return d.err }
func TestAuthUserClaimsDirectory(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	for _, tc := range []struct {
		name     string
		origin   models.UserOrigin
		verifier *directoryCheck
		denied   bool
	}{{"AD fail closed", models.UserOriginActiveDirectory, nil, true}, {"LDAP fail closed", models.UserOriginLDAP, &directoryCheck{err: errors.New("offline")}, true}, {"AD valid", models.UserOriginActiveDirectory, &directoryCheck{}, false}, {"local independent", models.UserOriginLocal, &directoryCheck{err: errors.New("offline")}, false}} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			db := mocks.NewMockStore(t)
			u := &models.User{ID: "fixture-user", Origin: tc.origin}
			db.On("UserResolve", ctx, store.UserIDResolver, u.ID).Return(u, nil).Once()
			opts := []Option{}
			if tc.verifier != nil {
				opts = append(opts, WithDirectoryVerifier(tc.verifier))
			}
			s := NewService(db, key, &key.PublicKey, cache.NewNullCache(), opts...)
			_, err := s.AuthUserClaims(ctx, &authorizer.UserClaims{ID: u.ID})
			if tc.denied {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			if tc.origin == models.UserOriginLocal {
				require.Zero(t, tc.verifier.calls)
			}
		})
	}
}
