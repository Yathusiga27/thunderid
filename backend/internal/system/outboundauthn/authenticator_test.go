// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package outboundauthn

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewAppliesNoAuthentication(t *testing.T) {
	authenticator, err := New(Config{Scheme: SchemeNone})
	require.NoError(t, err)

	req := httptestRequest(t)
	authenticator.Apply(req)

	require.Empty(t, req.Header)
}

func TestNewAppliesBearerAuthentication(t *testing.T) {
	authenticator, err := New(Config{Scheme: SchemeBearer, BearerToken: "token"})
	require.NoError(t, err)

	req := httptestRequest(t)
	authenticator.Apply(req)

	require.Equal(t, "Bearer token", req.Header.Get("Authorization"))
}

func TestNewAppliesAPIKeyAuthentication(t *testing.T) {
	authenticator, err := New(Config{
		Scheme:       SchemeAPIKey,
		APIKeyHeader: "X-API-Key",
		APIKeyValue:  "secret",
	})
	require.NoError(t, err)

	req := httptestRequest(t)
	authenticator.Apply(req)

	require.Equal(t, "secret", req.Header.Get("X-API-Key"))
	require.Empty(t, req.Header.Get("Authorization"))
}

func TestNewRejectsAPIKeyWithoutHeader(t *testing.T) {
	_, err := New(Config{Scheme: SchemeAPIKey, APIKeyValue: "secret"})
	require.EqualError(t, err, "API key header is required")
}

func TestNewInfersAuthenticationScheme(t *testing.T) {
	authenticator, err := New(Config{BearerToken: "token"})
	require.NoError(t, err)

	req := httptestRequest(t)
	authenticator.Apply(req)

	require.Equal(t, "Bearer token", req.Header.Get("Authorization"))
}

func httptestRequest(t *testing.T) *http.Request {
	t.Helper()
	return httptest.NewRequest(http.MethodGet, "https://example.com", nil)
}
