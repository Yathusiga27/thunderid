// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package engine

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

type routerTestEngine struct {
	decision bool
	called   int
}

func (e *routerTestEngine) EvaluateAccess(
	ctx context.Context,
	request AccessEvaluationRequest,
) (*AccessEvaluationResponse, error) {
	response, err := e.EvaluateAccessBatch(ctx, AccessEvaluationsRequest{
		Evaluations: []AccessEvaluationRequest{request},
	})
	if err != nil {
		return nil, err
	}
	return &response.Evaluations[0], nil
}

func (e *routerTestEngine) EvaluateAccessBatch(
	_ context.Context,
	request AccessEvaluationsRequest,
) (*AccessEvaluationsResponse, error) {
	e.called++
	responses := make([]AccessEvaluationResponse, len(request.Evaluations))
	for index := range responses {
		responses[index] = AccessEvaluationResponse{Decision: e.decision}
	}
	return &AccessEvaluationsResponse{Evaluations: responses}, nil
}

type routerErrorEngine struct{}

func (routerErrorEngine) EvaluateAccess(context.Context, AccessEvaluationRequest) (*AccessEvaluationResponse, error) {
	return nil, errors.New("engine error")
}

func (routerErrorEngine) EvaluateAccessBatch(context.Context, AccessEvaluationsRequest) (*AccessEvaluationsResponse, error) {
	return nil, errors.New("engine error")
}

func TestAuthorizationRouterRoutesByResourceServer(t *testing.T) {
	fallback := &routerTestEngine{decision: false}
	external := &routerTestEngine{decision: true}
	router := NewAuthorizationRouter(fallback, external, []string{"external-api-identifier"})

	response, err := router.EvaluateAccessBatch(context.Background(), AccessEvaluationsRequest{
		Evaluations: []AccessEvaluationRequest{
			{ResourceServer: ResourceServer{ID: "local-api"}},
			{ResourceServer: ResourceServer{
				ID: "external-api",
				Properties: map[string]interface{}{
					resourceServerIdentifierProperty: "external-api-identifier",
				},
			}},
		},
	})

	require.NoError(t, err)
	require.Equal(t, []AccessEvaluationResponse{{Decision: false}, {Decision: true}}, response.Evaluations)
	require.Equal(t, 1, fallback.called)
	require.Equal(t, 1, external.called)
}

func TestAuthorizationRouterRoutesToMultipleExternalEngines(t *testing.T) {
	fallback := &routerTestEngine{decision: false}
	primaryExternal := &routerTestEngine{decision: true}
	other := &routerTestEngine{decision: false}
	router := NewAuthorizationMultiRouter(fallback, []AuthorizationPDPRoute{
		{Engine: primaryExternal, ResourceServers: []string{"external-api-1"}},
		{Engine: other, ResourceServers: []string{"other-api"}},
	})

	response, err := router.EvaluateAccessBatch(context.Background(), AccessEvaluationsRequest{
		Evaluations: []AccessEvaluationRequest{
			{ResourceServer: ResourceServer{ID: "external-api-1"}},
			{ResourceServer: ResourceServer{ID: "other-api"}},
			{ResourceServer: ResourceServer{ID: "local-api"}},
		},
	})

	require.NoError(t, err)
	require.Equal(t, []AccessEvaluationResponse{
		{Decision: true},
		{Decision: false},
		{Decision: false},
	}, response.Evaluations)
	require.Equal(t, 1, primaryExternal.called)
	require.Equal(t, 1, other.called)
	require.Equal(t, 1, fallback.called)
}

func TestAuthorizationRouterUsesFallbackWhenExternalEngineIsMissing(t *testing.T) {
	fallback := &routerTestEngine{decision: true}
	router := NewAuthorizationRouter(fallback, nil, []string{"external-api"})

	response, err := router.EvaluateAccess(context.Background(), AccessEvaluationRequest{
		ResourceServer: ResourceServer{ID: "external-api"},
	})

	require.NoError(t, err)
	require.True(t, response.Decision)
	require.Equal(t, 1, fallback.called)
}

func TestAuthorizationRouterPropagatesEngineError(t *testing.T) {
	router := NewAuthorizationRouter(routerErrorEngine{}, nil, nil)

	_, err := router.EvaluateAccess(context.Background(), AccessEvaluationRequest{})

	require.EqualError(t, err, "engine error")
}

func TestAuthorizationRouterPropagatesExternalEngineError(t *testing.T) {
	router := NewAuthorizationRouter(&routerTestEngine{decision: true}, routerErrorEngine{}, []string{"external-api"})

	_, err := router.EvaluateAccess(context.Background(), AccessEvaluationRequest{
		ResourceServer: ResourceServer{ID: "external-api"},
	})

	require.EqualError(t, err, "engine error")
}
