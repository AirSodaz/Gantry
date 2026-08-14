package identity

import (
	"context"
	"crypto/subtle"
	"errors"
	"fmt"
	"strings"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrUnauthorized = errors.New("unauthorized")
var ErrUnknownPrincipal = errors.New("unknown principal")

type Principal struct {
	ID             string
	OrganizationID string
	Subject        string
	DisplayName    string
}

type Verifier interface {
	Verify(context.Context, string) (Claims, error)
}

type Claims struct {
	Subject       string `json:"sub"`
	PreferredName string `json:"preferred_username"`
}

type OIDCVerifier struct{ verifier *oidc.IDTokenVerifier }

func NewOIDCVerifier(ctx context.Context, issuer, audience string) (*OIDCVerifier, error) {
	provider, err := oidc.NewProvider(ctx, issuer)
	if err != nil {
		return nil, err
	}
	return &OIDCVerifier{verifier: provider.Verifier(&oidc.Config{ClientID: audience})}, nil
}

func (v *OIDCVerifier) Verify(ctx context.Context, raw string) (Claims, error) {
	token, err := v.verifier.Verify(ctx, raw)
	if err != nil {
		return Claims{}, fmt.Errorf("%w: %v", ErrUnauthorized, err)
	}
	var claims Claims
	if err := token.Claims(&claims); err != nil || claims.Subject == "" {
		return Claims{}, ErrUnauthorized
	}
	return claims, nil
}

type Resolver struct{ pool *pgxpool.Pool }

func NewResolver(pool *pgxpool.Pool) *Resolver { return &Resolver{pool: pool} }

func (r *Resolver) Resolve(ctx context.Context, claims Claims) (Principal, error) {
	var principal Principal
	err := r.pool.QueryRow(ctx, `
		SELECT id, organization_id, external_subject, display_name
		FROM gantry.principals WHERE external_subject=$1`, claims.Subject,
	).Scan(&principal.ID, &principal.OrganizationID, &principal.Subject, &principal.DisplayName)
	if err != nil {
		return Principal{}, fmt.Errorf("%w: %v", ErrUnknownPrincipal, err)
	}
	return principal, nil
}

type PrincipalResolver interface {
	Resolve(context.Context, Claims) (Principal, error)
}

type Authenticator struct {
	verifier Verifier
	resolver PrincipalResolver
}

func NewAuthenticator(verifier Verifier, resolver PrincipalResolver) *Authenticator {
	return &Authenticator{verifier: verifier, resolver: resolver}
}

func (a *Authenticator) Authenticate(ctx context.Context, authorization string) (Principal, error) {
	const prefix = "Bearer "
	if !strings.HasPrefix(authorization, prefix) || len(authorization) <= len(prefix) {
		return Principal{}, ErrUnauthorized
	}
	// Reject the wrong authentication scheme without accepting case variants.
	if subtle.ConstantTimeCompare([]byte(authorization[:len(prefix)]), []byte(prefix)) != 1 {
		return Principal{}, ErrUnauthorized
	}
	claims, err := a.verifier.Verify(ctx, authorization[len(prefix):])
	if err != nil {
		return Principal{}, err
	}
	return a.resolver.Resolve(ctx, claims)
}
