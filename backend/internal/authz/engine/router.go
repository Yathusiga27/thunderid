// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package engine

import (
	"context"
)

// AuthorizationRouter selects an authorization engine by resource-server ID.
type AuthorizationRouter struct {
	defaultEngine  AuthorizationEngine
	externalRoutes map[string]authorizationRoute
}

type authorizationRoute struct {
	engine AuthorizationEngine
}

// AuthorizationPDPRoute associates an AuthZEN engine with resource servers.
type AuthorizationPDPRoute struct {
	Engine          AuthorizationEngine
	ResourceServers []string
}

// NewAuthorizationRouter creates an engine that routes selected resource servers
// to an external engine and sends all other evaluations to the default engine.
func NewAuthorizationRouter(
	defaultEngine AuthorizationEngine,
	externalEngine AuthorizationEngine,
	externalResourceIDs []string,
) AuthorizationEngine {
	return newAuthorizationRouter(defaultEngine, []AuthorizationPDPRoute{{
		Engine:          externalEngine,
		ResourceServers: externalResourceIDs,
	}})
}

// NewAuthorizationMultiRouter creates an engine that routes resource servers to external PDPs.
func NewAuthorizationMultiRouter(
	defaultEngine AuthorizationEngine,
	routes []AuthorizationPDPRoute,
) AuthorizationEngine {
	return newAuthorizationRouter(defaultEngine, routes)
}

func newAuthorizationRouter(
	defaultEngine AuthorizationEngine,
	routes []AuthorizationPDPRoute,
) *AuthorizationRouter {
	externalRoutes := make(map[string]authorizationRoute)
	for _, route := range routes {
		if route.Engine == nil {
			continue
		}
		for _, resourceServer := range route.ResourceServers {
			if resourceServer != "" {
				externalRoutes[resourceServer] = authorizationRoute{
					engine: route.Engine,
				}
			}
		}
	}

	return &AuthorizationRouter{
		defaultEngine:  defaultEngine,
		externalRoutes: externalRoutes,
	}
}

// EvaluateAccess evaluates one request with the selected engine.
func (r *AuthorizationRouter) EvaluateAccess(
	ctx context.Context,
	request AccessEvaluationRequest,
) (*AccessEvaluationResponse, error) {
	response, err := r.EvaluateAccessBatch(ctx, AccessEvaluationsRequest{
		Evaluations: []AccessEvaluationRequest{request},
	})
	if err != nil {
		return nil, err
	}
	if len(response.Evaluations) == 0 {
		return &AccessEvaluationResponse{}, nil
	}
	return &response.Evaluations[0], nil
}

// EvaluateAccessBatch routes evaluations while preserving their input order.
func (r *AuthorizationRouter) EvaluateAccessBatch(
	ctx context.Context,
	request AccessEvaluationsRequest,
) (*AccessEvaluationsResponse, error) {
	responses := make([]AccessEvaluationResponse, len(request.Evaluations))
	defaultRequest := AccessEvaluationsRequest{}
	defaultIndexes := make([]int, 0, len(request.Evaluations))
	externalRequests := make(map[AuthorizationEngine]AccessEvaluationsRequest)
	externalIndexes := make(map[AuthorizationEngine][]int)

	for index, evaluation := range request.Evaluations {
		if route, ok := r.externalRoutes[resourceServerRouteID(evaluation.ResourceServer)]; ok {
			externalRequest := externalRequests[route.engine]
			externalRequest.Evaluations = append(externalRequest.Evaluations, evaluation)
			externalRequests[route.engine] = externalRequest
			externalIndexes[route.engine] = append(externalIndexes[route.engine], index)
			continue
		}
		defaultRequest.Evaluations = append(defaultRequest.Evaluations, evaluation)
		defaultIndexes = append(defaultIndexes, index)
	}

	for externalEngine, externalRequest := range externalRequests {
		if err := evaluateRoutedBatch(ctx, externalEngine, externalRequest, externalIndexes[externalEngine], responses); err != nil {
			return nil, err
		}
	}
	if err := evaluateRoutedBatch(ctx, r.defaultEngine, defaultRequest, defaultIndexes, responses); err != nil {
		return nil, err
	}

	return &AccessEvaluationsResponse{Evaluations: responses}, nil
}

// resourceServerRouteID returns the external routing key for a resource server.
func resourceServerRouteID(resource ResourceServer) string {
	if identifier, ok := resource.Properties[resourceServerIdentifierProperty].(string); ok && identifier != "" {
		return identifier
	}
	return resource.ID
}

// evaluateRoutedBatch runs one routed batch and writes each result back to its original position.
func evaluateRoutedBatch(
	ctx context.Context,
	engine AuthorizationEngine,
	request AccessEvaluationsRequest,
	indexes []int,
	responses []AccessEvaluationResponse,
) error {
	if len(request.Evaluations) == 0 {
		return nil
	}
	if engine == nil {
		return nil
	}

	engineResponse, err := engine.EvaluateAccessBatch(ctx, request)
	if err != nil {
		return err
	}
	for index, evaluation := range engineResponse.Evaluations {
		if index < len(indexes) {
			responses[indexes[index]] = evaluation
		}
	}
	return nil
}
