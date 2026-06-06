/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package authzen

import (
	"context"
	"slices"
	"strings"

	"github.com/thunder-id/thunderid/internal/authz"
	"github.com/thunder-id/thunderid/internal/entityprovider"
	"github.com/thunder-id/thunderid/internal/resource"
	serverconst "github.com/thunder-id/thunderid/internal/system/constants"
	"github.com/thunder-id/thunderid/internal/system/error/serviceerror"
	"github.com/thunder-id/thunderid/internal/system/log"
)

const loggerComponentName = "AuthZENService"

// ServiceInterface defines AuthZEN access evaluation operations.
type ServiceInterface interface {
	EvaluateAccess(ctx context.Context, request AccessEvaluationRequest) (
		*AccessEvaluationResponse, *serviceerror.ServiceError)
	EvaluateAccessBatch(ctx context.Context, request AccessEvaluationsRequest) (
		*AccessEvaluationsResponse, *serviceerror.ServiceError)
	SearchActions(ctx context.Context, request AccessActionSearchRequest) (
		*AccessSearchResponse, *serviceerror.ServiceError)
}

type service struct {
	authzService    authz.AuthorizationServiceInterface
	entityProvider  entityprovider.EntityProviderInterface
	resourceService resource.ResourceServiceInterface
	logger          *log.Logger
}

func newService(
	authzService authz.AuthorizationServiceInterface,
	entityProvider entityprovider.EntityProviderInterface,
	resourceService resource.ResourceServiceInterface,
) ServiceInterface {
	return &service{
		authzService:    authzService,
		entityProvider:  entityProvider,
		resourceService: resourceService,
		logger:          log.GetLogger().With(log.String(log.LoggerKeyComponentName, loggerComponentName)),
	}
}

// EvaluateAccess evaluates one AuthZEN access request against Thunder authorization.
func (s *service) EvaluateAccess(ctx context.Context, request AccessEvaluationRequest) (
	*AccessEvaluationResponse, *serviceerror.ServiceError) {
	if svcErr := validateEvaluationRequest(request); svcErr != nil {
		return nil, svcErr
	}

	groupIDs, svcErr := s.resolveGroupIDs(request.Subject.ID)
	if svcErr != nil {
		return nil, svcErr
	}

	authzResp, svcErr := s.authzService.EvaluateAccess(ctx, toAuthzAccessEvaluationRequest(request, groupIDs))
	if svcErr != nil {
		s.logger.Error("Authorization evaluation failed",
			log.MaskedString(log.LoggerKeyUserID, request.Subject.ID),
			log.String("error", svcErr.Error.DefaultValue))
		return nil, &serviceerror.InternalServerError
	}

	return &AccessEvaluationResponse{
		Decision: authzResp.Decision,
	}, nil
}

// EvaluateAccessBatch evaluates multiple AuthZEN access requests and preserves request order.
func (s *service) EvaluateAccessBatch(ctx context.Context, request AccessEvaluationsRequest) (
	*AccessEvaluationsResponse, *serviceerror.ServiceError) {
	if len(request.Evaluations) == 0 {
		return nil, &ErrorMissingEvaluations
	}

	authzEvaluations := make([]authz.AccessEvaluationRequest, 0, len(request.Evaluations))
	for _, evaluation := range request.Evaluations {
		if svcErr := validateEvaluationRequest(evaluation); svcErr != nil {
			return nil, svcErr
		}

		groupIDs, svcErr := s.resolveGroupIDs(evaluation.Subject.ID)
		if svcErr != nil {
			return nil, svcErr
		}
		authzEvaluations = append(authzEvaluations, toAuthzAccessEvaluationRequest(evaluation, groupIDs))
	}

	authzResp, svcErr := s.authzService.EvaluateAccessBatch(ctx, authz.AccessEvaluationsRequest{
		Evaluations: authzEvaluations,
	})
	if svcErr != nil {
		s.logger.Error("Authorization batch evaluation failed",
			log.Int("evaluationCount", len(request.Evaluations)),
			log.String("error", svcErr.Error.DefaultValue))
		return nil, &serviceerror.InternalServerError
	}

	responses := make([]AccessEvaluationResponse, 0, len(authzResp.Evaluations))
	for _, evaluation := range authzResp.Evaluations {
		responses = append(responses, AccessEvaluationResponse{Decision: evaluation.Decision})
	}

	return &AccessEvaluationsResponse{
		Evaluations: responses,
	}, nil
}

// SearchActions returns actions the subject is authorized to perform on the resource.
func (s *service) SearchActions(ctx context.Context, request AccessActionSearchRequest) (
	*AccessSearchResponse, *serviceerror.ServiceError) {
	if strings.TrimSpace(request.Subject.ID) == "" {
		return nil, &ErrorMissingSubject
	}
	if strings.TrimSpace(request.Resource.Type) == "" {
		return nil, &ErrorMissingResource
	}
	if strings.TrimSpace(request.Resource.ID) == "" {
		return nil, &ErrorMissingResourceID
	}

	resourceServerID, ok := request.Context["resourceServerId"].(string)
	if !ok || strings.TrimSpace(resourceServerID) == "" {
		return nil, &ErrorMissingResourceServerID
	}

	groupIDs, svcErr := s.resolveGroupIDs(request.Subject.ID)
	if svcErr != nil {
		return nil, svcErr
	}

	actionList, svcErr := s.resourceService.GetActionList(
		ctx, resourceServerID, &request.Resource.ID, serverconst.MaxPageSize, 0)
	if svcErr != nil {
		s.logger.Error("Failed to retrieve resource actions",
			log.String("resourceServerID", resourceServerID),
			log.String("resourceID", request.Resource.ID),
			log.String("error", svcErr.Error.DefaultValue))
		return nil, &serviceerror.InternalServerError
	}

	requestedPermissions := make([]string, 0, len(actionList.Actions))
	actionByPermission := make(map[string]Action, len(actionList.Actions))
	for _, action := range actionList.Actions {
		if action.Permission == "" {
			continue
		}
		requestedPermissions = append(requestedPermissions, action.Permission)
		actionByPermission[action.Permission] = Action{Name: action.Handle}
	}

	authzEvaluations := make([]authz.AccessEvaluationRequest, 0, len(requestedPermissions))
	for _, permission := range requestedPermissions {
		action := actionByPermission[permission]
		authzEvaluations = append(authzEvaluations, authz.AccessEvaluationRequest{
			Subject: authz.Subject{
				ID:       request.Subject.ID,
				Type:     request.Subject.Type,
				GroupIDs: groupIDs,
			},
			Resource: authz.Resource{
				Type: request.Resource.Type,
				ID:   request.Resource.ID,
			},
			Action:  authz.Action{Name: action.Name},
			Context: request.Context,
		})
	}

	authzResp, svcErr := s.authzService.EvaluateAccessBatch(ctx, authz.AccessEvaluationsRequest{
		Evaluations: authzEvaluations,
	})
	if svcErr != nil {
		s.logger.Error("Authorization action search failed",
			log.MaskedString(log.LoggerKeyUserID, request.Subject.ID),
			log.String("error", svcErr.Error.DefaultValue))
		return nil, &serviceerror.InternalServerError
	}

	results := make([]interface{}, 0, len(authzResp.Evaluations))
	for i, evaluation := range authzResp.Evaluations {
		if evaluation.Decision && i < len(requestedPermissions) {
			if action, ok := actionByPermission[requestedPermissions[i]]; ok {
				results = append(results, action)
			}
		}
	}
	return &AccessSearchResponse{Results: results}, nil
}

func validateEvaluationRequest(request AccessEvaluationRequest) *serviceerror.ServiceError {
	if strings.TrimSpace(request.Subject.ID) == "" {
		return &ErrorMissingSubject
	}
	if strings.TrimSpace(request.Resource.Type) == "" {
		return &ErrorMissingResource
	}
	if strings.TrimSpace(request.Resource.ID) == "" {
		return &ErrorMissingResourceID
	}
	if strings.TrimSpace(request.Action.Name) == "" {
		return &ErrorMissingAction
	}
	return nil
}

func (s *service) resolveGroupIDs(entityID string) ([]string, *serviceerror.ServiceError) {
	if s.entityProvider == nil {
		return []string{}, nil
	}

	groups, err := s.entityProvider.GetTransitiveEntityGroups(entityID)
	if err != nil {
		if err.Code == entityprovider.ErrorCodeNotImplemented {
			return []string{}, nil
		}
		s.logger.Error("Failed to resolve entity groups",
			log.MaskedString(log.LoggerKeyUserID, entityID),
			log.String("error", err.Error()))
		return nil, &serviceerror.InternalServerError
	}

	groupIDs := make([]string, 0, len(groups))
	for _, group := range groups {
		if group.ID != "" && !slices.Contains(groupIDs, group.ID) {
			groupIDs = append(groupIDs, group.ID)
		}
	}
	return groupIDs, nil
}

func toAuthzAccessEvaluationRequest(request AccessEvaluationRequest, groupIDs []string) authz.AccessEvaluationRequest {
	return authz.AccessEvaluationRequest{
		Subject: authz.Subject{
			Type:     request.Subject.Type,
			ID:       request.Subject.ID,
			GroupIDs: groupIDs,
		},
		Resource: authz.Resource{
			Type: request.Resource.Type,
			ID:   request.Resource.ID,
		},
		Action:  authz.Action{Name: request.Action.Name},
		Context: request.Context,
	}
}
