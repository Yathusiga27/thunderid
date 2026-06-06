/*
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
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

// Package engine provides authorization engine implementations.
// It includes various authorization engines such as RBAC (Role-Based Access Control)
// that delegate authorization decisions to the appropriate services.
package engine

import (
	"context"
	"fmt"
	"slices"

	"github.com/thunder-id/thunderid/internal/role"
)

// rbacEngine implements Role-Based Access Control (RBAC) authorization.
// It delegates authorization decisions to the role service.
type rbacEngine struct {
	roleService role.RoleServiceInterface
}

// NewRBACEngine creates a new RBAC authorization engine.
func NewRBACEngine(roleService role.RoleServiceInterface) AuthorizationEngine {
	return &rbacEngine{
		roleService: roleService,
	}
}

// EvaluateAccess evaluates a single fine-grained access request.
func (e *rbacEngine) EvaluateAccess(
	ctx context.Context,
	request AccessEvaluationRequest,
) (*AccessEvaluationResponse, error) {
	response, err := e.EvaluateAccessBatch(ctx, AccessEvaluationsRequest{
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

// EvaluateAccessBatch evaluates multiple fine-grained access requests based on role assignments.
func (e *rbacEngine) EvaluateAccessBatch(
	ctx context.Context,
	request AccessEvaluationsRequest,
) (*AccessEvaluationsResponse, error) {
	if len(request.Evaluations) == 0 {
		return &AccessEvaluationsResponse{Evaluations: []AccessEvaluationResponse{}}, nil
	}

	evaluations := make([]AccessEvaluationResponse, 0, len(request.Evaluations))
	for _, evaluation := range request.Evaluations {
		permission := buildPermission(evaluation)
		authorizedPerms, svcErr := e.roleService.GetAuthorizedPermissions(
			ctx, evaluation.Subject.ID, evaluation.Subject.GroupIDs, []string{permission})
		if svcErr != nil {
			return nil, fmt.Errorf("role service error: %s", svcErr.Error)
		}
		evaluations = append(evaluations, AccessEvaluationResponse{
			Decision: slices.Contains(authorizedPerms, permission),
		})
	}
	return &AccessEvaluationsResponse{Evaluations: evaluations}, nil
}

func buildPermission(request AccessEvaluationRequest) string {
	if request.Resource.Type == "" {
		return request.Action.Name
	}
	return request.Resource.Type + ":" + request.Action.Name
}
