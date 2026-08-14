// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package engine

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAuthZENPDPEvaluateAccess(t *testing.T) {
	server := newAuthZENTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodPost, r.Method)
		require.Equal(t, "application/json", r.Header.Get("Content-Type"))
		require.Equal(t, "Bearer pdp-token", r.Header.Get("Authorization"))

		var request authZENEvaluationRequest
		require.NoError(t, json.NewDecoder(r.Body).Decode(&request))
		require.Equal(t, "user", request.Subject.Type)
		require.Equal(t, "user-1", request.Subject.ID)
		require.Equal(t, []interface{}{"travel-agent"}, request.Subject.Properties[subjectGroupsProperty])
		require.Equal(t, "finance", request.Subject.Properties["department_name"])
		require.NotContains(t, request.Subject.Properties, "department")
		require.NotContains(t, request.Subject.Properties, "email")
		require.Equal(t, "external-resource", request.Resource.Type)
		require.Equal(t, "read", request.Action.Name)
		require.Equal(t, "authorization_code", request.Context["grant_type"])

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"decision":true,"context":{"policy":"allow-read"}}`))
	}))
	defer server.Close()

	engine, err := NewAuthZENPDP(AuthZENPDPConfig{
		Endpoint:                server.URL,
		BearerToken:             "pdp-token",
		SubjectProperties:       []string{"department"},
		SubjectPropertyMappings: map[string]string{"department": "department_name"},
	})
	require.NoError(t, err)

	response, err := engine.EvaluateAccess(context.Background(), AccessEvaluationRequest{
		Subject: Subject{
			Type:     "user",
			ID:       "user-1",
			GroupIDs: []string{"travel-agent"},
			Properties: map[string]interface{}{
				"department": "finance",
				"email":      "user@example.com",
			},
		},
		ResourceServer: ResourceServer{
			ID: "resource-server-1",
			Properties: map[string]interface{}{
				resourceServerIdentifierProperty: "external-resource",
			},
		},
		Permission: Permission{Name: "read"},
		Context:    map[string]interface{}{"grant_type": "authorization_code"},
	})
	require.NoError(t, err)
	require.True(t, response.Decision)
	require.Equal(t, "allow-read", response.Context["policy"])
}

func TestAuthZENPDPEvaluateAccessDeny(t *testing.T) {
	server := newAuthZENTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"decision":false,"context":{"reason":"denied"}}`))
	}))
	defer server.Close()

	engine, err := NewAuthZENPDP(AuthZENPDPConfig{Endpoint: server.URL})
	require.NoError(t, err)

	response, err := engine.EvaluateAccess(context.Background(), AccessEvaluationRequest{
		Subject:        Subject{ID: "user-1"},
		ResourceServer: ResourceServer{ID: "resource-server-1"},
		Permission:     Permission{Name: "delete"},
	})
	require.NoError(t, err)
	require.False(t, response.Decision)
	require.Equal(t, "denied", response.Context["reason"])
}

func TestAuthZENPDPEvaluateAccessConfiguredResourceMapping(t *testing.T) {
	server := newAuthZENTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request authZENEvaluationRequest
		require.NoError(t, json.NewDecoder(r.Body).Decode(&request))
		require.Equal(t, "resource", request.Resource.Type)
		require.Equal(t, "configured-resource-id", request.Resource.ID)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"decision":true}`))
	}))
	defer server.Close()

	pdp, err := NewAuthZENPDP(AuthZENPDPConfig{
		Endpoint:     server.URL,
		ResourceType: "resource",
		ResourceID:   "configured-resource-id",
	})
	require.NoError(t, err)

	response, err := pdp.EvaluateAccess(context.Background(), AccessEvaluationRequest{
		ResourceServer: ResourceServer{
			ID: "external-resource-server",
			Properties: map[string]interface{}{
				resourceServerIdentifierProperty: "https://api.example.com/external-resource",
			},
		},
		Permission: Permission{Name: "booking_read"},
	})
	require.NoError(t, err)
	require.True(t, response.Decision)
}

func TestAuthZENPDPEvaluateAccessBatch(t *testing.T) {
	server := newAuthZENTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/evaluations", r.URL.Path)

		var request authZENBatchEvaluationRequest
		require.NoError(t, json.NewDecoder(r.Body).Decode(&request))
		require.Len(t, request.Evaluations, 2)
		require.Equal(t, "read", request.Evaluations[0].Action.Name)
		require.Equal(t, "cancel", request.Evaluations[1].Action.Name)

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"evaluations":[{"decision":true},{"decision":false}]}`))
	}))
	defer server.Close()

	pdp, err := NewAuthZENPDP(AuthZENPDPConfig{Endpoint: server.URL + "/evaluation"})
	require.NoError(t, err)

	response, err := pdp.EvaluateAccessBatch(context.Background(), AccessEvaluationsRequest{
		Evaluations: []AccessEvaluationRequest{
			{Subject: Subject{ID: "user-1"}, ResourceServer: ResourceServer{ID: "resource-1"}, Permission: Permission{Name: "read"}},
			{Subject: Subject{ID: "user-1"}, ResourceServer: ResourceServer{ID: "resource-1"}, Permission: Permission{Name: "cancel"}},
		},
	})
	require.NoError(t, err)
	require.Len(t, response.Evaluations, 2)
	require.True(t, response.Evaluations[0].Decision)
	require.False(t, response.Evaluations[1].Decision)
}

func TestAuthZENPDPRejectsPDPError(t *testing.T) {
	server := newAuthZENTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "unavailable", http.StatusServiceUnavailable)
	}))
	defer server.Close()

	engine, err := NewAuthZENPDP(AuthZENPDPConfig{Endpoint: server.URL})
	require.NoError(t, err)

	_, err = engine.EvaluateAccess(context.Background(), AccessEvaluationRequest{})
	require.ErrorContains(t, err, "HTTP 503")
}

func TestNewAuthZENPDPRejectsInvalidEndpoint(t *testing.T) {
	_, err := NewAuthZENPDP(AuthZENPDPConfig{Endpoint: "not-a-url"})
	require.Error(t, err)
}

func TestNewAuthZENPDPRejectsMappingForDisallowedProperty(t *testing.T) {
	_, err := NewAuthZENPDP(AuthZENPDPConfig{
		Endpoint:                "http://localhost:9000/access/v1/evaluation",
		SubjectPropertyMappings: map[string]string{"department": "department_name"},
	})
	require.EqualError(t, err, "subject property \"department\" has a mapping but is not allowed")
}

func newAuthZENTestServer(t *testing.T, handler http.Handler) *httptest.Server {
	t.Helper()
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	require.NoError(t, err)

	server := httptest.NewUnstartedServer(handler)
	server.Listener = listener
	server.Start()
	t.Cleanup(server.Close)
	return server
}
