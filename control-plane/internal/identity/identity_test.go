package identity

import (
	"context"
	"errors"
	"testing"
)

type testVerifier struct {
	claims Claims
	err    error
}

func (v testVerifier) Verify(context.Context, string) (Claims, error) { return v.claims, v.err }

type testResolver struct {
	principal Principal
	err       error
}

func (r testResolver) Resolve(context.Context, Claims) (Principal, error) { return r.principal, r.err }

func TestAuthenticatorResolvesBearerSubject(t *testing.T) {
	auth := NewAuthenticator(testVerifier{claims: Claims{Subject: "subject-1"}}, testResolver{principal: Principal{ID: "principal-1"}})
	principal, err := auth.Authenticate(context.Background(), "Bearer access-token")
	if err != nil || principal.ID != "principal-1" {
		t.Fatalf("principal=%#v err=%v", principal, err)
	}
}
func TestAuthenticatorRejectsInvalidAuthorization(t *testing.T) {
	auth := NewAuthenticator(testVerifier{}, testResolver{})
	if _, err := auth.Authenticate(context.Background(), "Basic value"); !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("err=%v", err)
	}
}
