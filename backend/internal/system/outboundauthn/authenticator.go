// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

// Package outboundauthn provides authentication strategies for outbound HTTP requests.
package outboundauthn

import (
	"fmt"
	"net/http"
	"strings"
)

const (
	// SchemeNone sends no authentication headers.
	SchemeNone = "NONE"
	// SchemeBearer sends an OAuth bearer token in the Authorization header.
	SchemeBearer = "BEARER"
	// SchemeAPIKey sends an API key in a configured header.
	SchemeAPIKey = "API_KEY"
)

// Config configures authentication for an outbound HTTP connection.
type Config struct {
	Scheme       string
	BearerToken  string
	APIKeyHeader string
	APIKeyValue  string
}

// RequestAuthenticator applies configured authentication to an HTTP request.
type RequestAuthenticator interface {
	Apply(*http.Request)
}

type requestAuthenticator struct {
	scheme       string
	bearerToken  string
	apiKeyHeader string
	apiKeyValue  string
}

// New creates an outbound HTTP request authenticator.
func New(config Config) (RequestAuthenticator, error) {
	scheme := normalizeScheme(config.Scheme, config.BearerToken, config.APIKeyHeader, config.APIKeyValue)
	if scheme != SchemeNone && scheme != SchemeBearer && scheme != SchemeAPIKey {
		return nil, fmt.Errorf("unsupported outbound authentication scheme %q", config.Scheme)
	}
	if scheme == SchemeAPIKey && strings.TrimSpace(config.APIKeyHeader) == "" {
		return nil, fmt.Errorf("API key header is required")
	}

	return &requestAuthenticator{
		scheme:       scheme,
		bearerToken:  config.BearerToken,
		apiKeyHeader: strings.TrimSpace(config.APIKeyHeader),
		apiKeyValue:  config.APIKeyValue,
	}, nil
}

// Apply adds the configured authentication headers to req.
func (a *requestAuthenticator) Apply(req *http.Request) {
	if req == nil {
		return
	}

	switch a.scheme {
	case SchemeBearer:
		if a.bearerToken != "" {
			req.Header.Set("Authorization", "Bearer "+a.bearerToken)
		}
	case SchemeAPIKey:
		if a.apiKeyValue != "" {
			req.Header.Set(a.apiKeyHeader, a.apiKeyValue)
		}
	}
}

func normalizeScheme(value, bearerToken, apiKeyHeader, apiKeyValue string) string {
	switch strings.ToUpper(strings.TrimSpace(value)) {
	case SchemeNone, SchemeBearer, SchemeAPIKey:
		return strings.ToUpper(strings.TrimSpace(value))
	default:
		if strings.TrimSpace(bearerToken) != "" {
			return SchemeBearer
		}
		if strings.TrimSpace(apiKeyHeader) != "" && strings.TrimSpace(apiKeyValue) != "" {
			return SchemeAPIKey
		}
		return SchemeNone
	}
}
