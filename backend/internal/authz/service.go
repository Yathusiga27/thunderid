// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

// Package authz provides authorization service functionality.
package authz

import (
	"context"
	"encoding/json"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"

	"github.com/thunder-id/thunderid/internal/authz/engine"
	"github.com/thunder-id/thunderid/internal/system/log"
	userpkg "github.com/thunder-id/thunderid/internal/user"
)

const loggerComponentName = "AuthorizationService"

// authorizationService is the default implementation of providers.AuthorizationProvider.
type authorizationService struct {
	engine      engine.AuthorizationEngine
	userService userpkg.UserServiceInterface
}

// newAuthorizationService creates a new instance of authorizationService.
func newAuthorizationService(engine engine.AuthorizationEngine, userServices ...userpkg.UserServiceInterface) providers.AuthorizationProvider {
	var userService userpkg.UserServiceInterface
	if len(userServices) > 0 {
		userService = userServices[0]
	}
	return &authorizationService{
		engine: engine, userService: userService,
	}
}

// EvaluateAccess evaluates a single fine-grained access request.
func (s *authorizationService) EvaluateAccess(
	ctx context.Context,
	request providers.AccessEvaluationRequest,
) (*providers.AccessEvaluationResponse, *tidcommon.ServiceError) {
	response, svcErr := s.EvaluateAccessBatch(ctx, providers.AccessEvaluationsRequest{
		Evaluations: []providers.AccessEvaluationRequest{request},
	})
	if svcErr != nil {
		return nil, svcErr
	}
	if len(response.Evaluations) == 0 {
		return &providers.AccessEvaluationResponse{}, nil
	}
	return &response.Evaluations[0], nil
}

// EvaluateAccessBatch evaluates multiple fine-grained access requests.
func (s *authorizationService) EvaluateAccessBatch(
	ctx context.Context,
	request providers.AccessEvaluationsRequest,
) (*providers.AccessEvaluationsResponse, *tidcommon.ServiceError) {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, loggerComponentName))
	logger.Debug(ctx, "Evaluating authorization request",
		log.Int("evaluationCount", len(request.Evaluations)))

	if len(request.Evaluations) == 0 {
		return &providers.AccessEvaluationsResponse{
			Evaluations: []providers.AccessEvaluationResponse{},
		}, nil
	}
	enrichedRequest, svcErr := s.enrichRequest(ctx, request)
	if svcErr != nil {
		return nil, svcErr
	}

	// Delegate to engine (engine/underlying service handles validation)
	evaluationResp, err := s.engine.EvaluateAccessBatch(ctx, toEngineAccessEvaluationsRequest(enrichedRequest))
	if err != nil {
		logger.Error(ctx, "Authorization evaluation failed",
			log.Int("evaluationCount", len(request.Evaluations)),
			log.Error(err))
		return nil, &tidcommon.InternalServerError
	}

	logger.Debug(ctx, "Authorization evaluation completed",
		log.Int("evaluationCount", len(request.Evaluations)))

	return fromEngineAccessEvaluationsResponse(evaluationResp), nil
}

func (s *authorizationService) enrichRequest(ctx context.Context, request providers.AccessEvaluationsRequest) (providers.AccessEvaluationsRequest, *tidcommon.ServiceError) {
	if s.userService == nil {
		return request, nil
	}
	enriched := providers.AccessEvaluationsRequest{Evaluations: make([]providers.AccessEvaluationRequest, 0, len(request.Evaluations))}
	users := make(map[string]*userpkg.User)
	for _, evaluation := range request.Evaluations {
		if evaluation.Subject.Type != providers.EntityCategoryUser.String() {
			enriched.Evaluations = append(enriched.Evaluations, evaluation)
			continue
		}
		user, ok := users[evaluation.Subject.ID]
		if !ok {
			var svcErr *tidcommon.ServiceError
			user, svcErr = s.userService.GetUser(ctx, evaluation.Subject.ID, false)
			if svcErr != nil {
				return providers.AccessEvaluationsRequest{}, &tidcommon.InternalServerError
			}
			users[evaluation.Subject.ID] = user
		}
		if user != nil {
			properties := map[string]interface{}{}
			if len(user.Attributes) > 0 {
				if err := json.Unmarshal(user.Attributes, &properties); err != nil {
					return providers.AccessEvaluationsRequest{}, &tidcommon.InternalServerError
				}
			}
			if user.OUID != "" {
				properties["ouId"] = user.OUID
			}
			for key, value := range evaluation.Subject.Properties {
				properties[key] = value
			}
			evaluation.Subject.Properties = properties
		}
		enriched.Evaluations = append(enriched.Evaluations, evaluation)
	}
	return enriched, nil
}

func toEngineAccessEvaluationsRequest(request providers.AccessEvaluationsRequest) engine.AccessEvaluationsRequest {
	evaluations := make([]engine.AccessEvaluationRequest, 0, len(request.Evaluations))
	for _, evaluation := range request.Evaluations {
		evaluations = append(evaluations, engine.AccessEvaluationRequest{
			Subject: engine.Subject{
				Type:       evaluation.Subject.Type,
				ID:         evaluation.Subject.ID,
				GroupIDs:   evaluation.Subject.GroupIDs,
				Properties: evaluation.Subject.Properties,
			},
			ResourceServer: engine.ResourceServer{
				ID:         evaluation.ResourceServer.ID,
				Properties: evaluation.ResourceServer.Properties,
			},
			Permission: engine.Permission{
				Name:       evaluation.Permission.Name,
				Properties: evaluation.Permission.Properties,
			},
			Context: evaluation.Context,
		})
	}
	return engine.AccessEvaluationsRequest{Evaluations: evaluations}
}

func fromEngineAccessEvaluationsResponse(
	response *engine.AccessEvaluationsResponse) *providers.AccessEvaluationsResponse {
	if response == nil {
		return &providers.AccessEvaluationsResponse{Evaluations: []providers.AccessEvaluationResponse{}}
	}

	evaluations := make([]providers.AccessEvaluationResponse, 0, len(response.Evaluations))
	for _, evaluation := range response.Evaluations {
		evaluations = append(evaluations, providers.AccessEvaluationResponse{
			Decision: evaluation.Decision,
			Context:  evaluation.Context,
		})
	}
	return &providers.AccessEvaluationsResponse{Evaluations: evaluations}
}
