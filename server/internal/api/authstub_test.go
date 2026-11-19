package api

import (
	"errors"
	"sync/atomic"

	"stock-dashboard/server/internal/auth"
)

// allowAllAuth is the harness session stub: every request counts as an
// authenticated owner so the pre-auth scenarios keep their expectations.
type allowAllAuth struct{}

func (allowAllAuth) Status(string) auth.Status {
	return auth.Status{
		SetupRequired: false,
		Authenticated: true,
		User:          &auth.Account{ID: testSessionUserID, Username: "owner", TOTPEnabled: true},
	}
}

func (allowAllAuth) BeginSetup(string, string) (*auth.SetupMaterial, error) {
	return nil, errors.New("stub: setup not supported")
}

func (allowAllAuth) Activate(string, string) (*auth.Session, error) {
	return nil, errors.New("stub: activate not supported")
}

func (allowAllAuth) Login(string, string) (*auth.Challenge, error) {
	return nil, errors.New("stub: login not supported")
}

func (allowAllAuth) CompleteTOTP(string, string) (*auth.Session, error) {
	return nil, errors.New("stub: totp not supported")
}

func (allowAllAuth) CompleteRecovery(string, string) (*auth.Session, error) {
	return nil, errors.New("stub: recovery not supported")
}

func (allowAllAuth) Logout(string) {}

func (allowAllAuth) Authenticate(string) (*auth.Account, bool) {
	return &auth.Account{ID: testSessionUserID, Username: "owner", TOTPEnabled: true}, true
}

// countingAuth records how often a guarded route authenticates the session.
type countingAuth struct {
	allowAllAuth
	calls *atomic.Int32
}

func (c *countingAuth) Authenticate(token string) (*auth.Account, bool) {
	c.calls.Add(1)
	return c.allowAllAuth.Authenticate(token)
}

// denyAllAuth models a request with no valid session.
type denyAllAuth struct{}

func (denyAllAuth) Status(string) auth.Status {
	return auth.Status{
		SetupRequired: false,
		Authenticated: false,
	}
}

func (denyAllAuth) BeginSetup(string, string) (*auth.SetupMaterial, error) {
	return nil, errors.New("stub: setup not supported")
}

func (denyAllAuth) Activate(string, string) (*auth.Session, error) {
	return nil, errors.New("stub: activate not supported")
}

func (denyAllAuth) Login(string, string) (*auth.Challenge, error) {
	return nil, errors.New("stub: login not supported")
}

func (denyAllAuth) CompleteTOTP(string, string) (*auth.Session, error) {
	return nil, errors.New("stub: totp not supported")
}

func (denyAllAuth) CompleteRecovery(string, string) (*auth.Session, error) {
	return nil, errors.New("stub: recovery not supported")
}

func (denyAllAuth) Logout(string) {}

func (denyAllAuth) Authenticate(string) (*auth.Account, bool) {
	return nil, false
}
