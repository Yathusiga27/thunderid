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
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/authz"
	"github.com/thunder-id/thunderid/internal/entityprovider"
	"github.com/thunder-id/thunderid/internal/resource"
	"github.com/thunder-id/thunderid/internal/system/error/serviceerror"
	"github.com/thunder-id/thunderid/tests/mocks/authzmock"
	"github.com/thunder-id/thunderid/tests/mocks/entityprovidermock"
	"github.com/thunder-id/thunderid/tests/mocks/resourcemock"
)

type ServiceTestSuite struct {
	suite.Suite
	authzMock          *authzmock.AuthorizationServiceInterfaceMock
	entityProviderMock *entityprovidermock.EntityProviderInterfaceMock
	resourceMock       *resourcemock.ResourceServiceInterfaceMock
	service            ServiceInterface
}

func TestServiceTestSuite(t *testing.T) {
	suite.Run(t, new(ServiceTestSuite))
}

func (s *ServiceTestSuite) SetupTest() {
	s.authzMock = authzmock.NewAuthorizationServiceInterfaceMock(s.T())
	s.entityProviderMock = entityprovidermock.NewEntityProviderInterfaceMock(s.T())
	s.resourceMock = resourcemock.NewResourceServiceInterfaceMock(s.T())
	s.service = newService(s.authzMock, s.entityProviderMock, s.resourceMock)
}

func (s *ServiceTestSuite) TestEvaluateAccessAllowed() {
	req := AccessEvaluationRequest{
		Subject:  Subject{Type: "user", ID: "user1"},
		Resource: Resource{Type: "booking", ID: "booking1"},
		Action:   Action{Name: "read"},
	}

	s.entityProviderMock.On("GetTransitiveEntityGroups", "user1").Return([]entityprovider.EntityGroup{
		{ID: "group1"},
		{ID: "group1"},
		{ID: "group2"},
	}, nil)
	s.authzMock.On("EvaluateAccess", mock.Anything, authz.AccessEvaluationRequest{
		Subject:  authz.Subject{Type: "user", ID: "user1", GroupIDs: []string{"group1", "group2"}},
		Resource: authz.Resource{Type: "booking", ID: "booking1"},
		Action:   authz.Action{Name: "read"},
	}).Return(&authz.AccessEvaluationResponse{Decision: true}, nil)

	resp, svcErr := s.service.EvaluateAccess(context.Background(), req)

	s.Nil(svcErr)
	s.NotNil(resp)
	s.True(resp.Decision)
}

func (s *ServiceTestSuite) TestEvaluateAccessDenied() {
	req := AccessEvaluationRequest{
		Subject:  Subject{Type: "user", ID: "user1"},
		Resource: Resource{Type: "booking", ID: "booking1"},
		Action:   Action{Name: "delete"},
	}

	s.entityProviderMock.On("GetTransitiveEntityGroups", "user1").Return([]entityprovider.EntityGroup{}, nil)
	s.authzMock.On("EvaluateAccess", mock.Anything, authz.AccessEvaluationRequest{
		Subject:  authz.Subject{Type: "user", ID: "user1", GroupIDs: []string{}},
		Resource: authz.Resource{Type: "booking", ID: "booking1"},
		Action:   authz.Action{Name: "delete"},
	}).Return(&authz.AccessEvaluationResponse{Decision: false}, nil)

	resp, svcErr := s.service.EvaluateAccess(context.Background(), req)

	s.Nil(svcErr)
	s.NotNil(resp)
	s.False(resp.Decision)
}

func (s *ServiceTestSuite) TestEvaluateAccessProviderNotImplementedUsesEmptyGroups() {
	req := AccessEvaluationRequest{
		Subject:  Subject{Type: "application", ID: "app1"},
		Resource: Resource{Type: "report", ID: "report1"},
		Action:   Action{Name: "read"},
	}

	s.entityProviderMock.On("GetTransitiveEntityGroups", "app1").Return(
		[]entityprovider.EntityGroup(nil),
		entityprovider.NewEntityProviderError(
			entityprovider.ErrorCodeNotImplemented, "not implemented", "not implemented"),
	)
	s.authzMock.On("EvaluateAccess", mock.Anything, authz.AccessEvaluationRequest{
		Subject:  authz.Subject{Type: "application", ID: "app1", GroupIDs: []string{}},
		Resource: authz.Resource{Type: "report", ID: "report1"},
		Action:   authz.Action{Name: "read"},
	}).Return(&authz.AccessEvaluationResponse{Decision: true}, nil)

	resp, svcErr := s.service.EvaluateAccess(context.Background(), req)

	s.Nil(svcErr)
	s.NotNil(resp)
	s.True(resp.Decision)
}

func (s *ServiceTestSuite) TestEvaluateAccessGroupResolutionFailure() {
	req := AccessEvaluationRequest{
		Subject:  Subject{Type: "user", ID: "user1"},
		Resource: Resource{Type: "booking", ID: "booking1"},
		Action:   Action{Name: "read"},
	}

	s.entityProviderMock.On("GetTransitiveEntityGroups", "user1").Return(
		[]entityprovider.EntityGroup(nil),
		entityprovider.NewEntityProviderError(entityprovider.ErrorCodeSystemError, "failed", "failed"),
	)

	resp, svcErr := s.service.EvaluateAccess(context.Background(), req)

	s.Nil(resp)
	s.NotNil(svcErr)
	s.Equal(serviceerror.InternalServerError.Code, svcErr.Code)
	s.authzMock.AssertNotCalled(s.T(), "EvaluateAccess", mock.Anything, mock.Anything)
}

func (s *ServiceTestSuite) TestEvaluateAccessAuthorizationFailure() {
	req := AccessEvaluationRequest{
		Subject:  Subject{Type: "user", ID: "user1"},
		Resource: Resource{Type: "booking", ID: "booking1"},
		Action:   Action{Name: "read"},
	}

	s.entityProviderMock.On("GetTransitiveEntityGroups", "user1").Return([]entityprovider.EntityGroup{}, nil)
	s.authzMock.On("EvaluateAccess", mock.Anything, authz.AccessEvaluationRequest{
		Subject:  authz.Subject{Type: "user", ID: "user1", GroupIDs: []string{}},
		Resource: authz.Resource{Type: "booking", ID: "booking1"},
		Action:   authz.Action{Name: "read"},
	}).Return((*authz.AccessEvaluationResponse)(nil), &serviceerror.InternalServerError)

	resp, svcErr := s.service.EvaluateAccess(context.Background(), req)

	s.Nil(resp)
	s.NotNil(svcErr)
	s.Equal(serviceerror.InternalServerError.Code, svcErr.Code)
}

func (s *ServiceTestSuite) TestEvaluateAccessValidationErrors() {
	tests := []struct {
		name string
		req  AccessEvaluationRequest
		code string
	}{
		{
			name: "missing subject",
			req: AccessEvaluationRequest{
				Resource: Resource{Type: "booking", ID: "booking1"},
				Action:   Action{Name: "read"},
			},
			code: ErrorMissingSubject.Code,
		},
		{
			name: "missing resource",
			req: AccessEvaluationRequest{
				Subject: Subject{ID: "user1"},
				Action:  Action{Name: "read"},
			},
			code: ErrorMissingResource.Code,
		},
		{
			name: "missing resource id",
			req: AccessEvaluationRequest{
				Subject:  Subject{ID: "user1"},
				Resource: Resource{Type: "booking"},
				Action:   Action{Name: "read"},
			},
			code: ErrorMissingResourceID.Code,
		},
		{
			name: "missing action",
			req: AccessEvaluationRequest{
				Subject:  Subject{ID: "user1"},
				Resource: Resource{Type: "booking", ID: "booking1"},
			},
			code: ErrorMissingAction.Code,
		},
	}

	for _, tc := range tests {
		s.Run(tc.name, func() {
			resp, svcErr := s.service.EvaluateAccess(context.Background(), tc.req)

			s.Nil(resp)
			s.NotNil(svcErr)
			s.Equal(tc.code, svcErr.Code)
		})
	}
}

func (s *ServiceTestSuite) TestEvaluateAccessBatchPreservesOrder() {
	req := AccessEvaluationsRequest{
		Evaluations: []AccessEvaluationRequest{
			{
				Subject:  Subject{Type: "user", ID: "user1"},
				Resource: Resource{Type: "booking", ID: "booking1"},
				Action:   Action{Name: "read"},
			},
			{
				Subject:  Subject{Type: "user", ID: "user1"},
				Resource: Resource{Type: "booking", ID: "booking1"},
				Action:   Action{Name: "delete"},
			},
		},
	}

	s.entityProviderMock.On("GetTransitiveEntityGroups", "user1").Return([]entityprovider.EntityGroup{}, nil).Twice()
	s.authzMock.On("EvaluateAccessBatch", mock.Anything,
		mock.MatchedBy(func(req authz.AccessEvaluationsRequest) bool {
			return len(req.Evaluations) == 2 &&
				req.Evaluations[0].Subject.ID == "user1" &&
				req.Evaluations[0].Resource.Type == "booking" &&
				req.Evaluations[0].Action.Name == "read" &&
				req.Evaluations[1].Action.Name == "delete"
		})).
		Return(&authz.AccessEvaluationsResponse{
			Evaluations: []authz.AccessEvaluationResponse{
				{Decision: true},
				{Decision: false},
			},
		}, nil)

	resp, svcErr := s.service.EvaluateAccessBatch(context.Background(), req)

	s.Nil(svcErr)
	s.NotNil(resp)
	s.Len(resp.Evaluations, 2)
	s.True(resp.Evaluations[0].Decision)
	s.False(resp.Evaluations[1].Decision)
}

func (s *ServiceTestSuite) TestEvaluateAccessBatchMissingEvaluations() {
	resp, svcErr := s.service.EvaluateAccessBatch(context.Background(), AccessEvaluationsRequest{})

	s.Nil(resp)
	s.NotNil(svcErr)
	s.Equal(ErrorMissingEvaluations.Code, svcErr.Code)
}

func (s *ServiceTestSuite) TestSearchActionsReturnsAuthorizedActions() {
	req := AccessActionSearchRequest{
		Subject:  Subject{Type: "user", ID: "user1"},
		Resource: Resource{Type: "booking:booking", ID: "booking1"},
		Context:  map[string]interface{}{"resourceServerId": "rs1"},
	}

	s.entityProviderMock.On("GetTransitiveEntityGroups", "user1").Return([]entityprovider.EntityGroup{
		{ID: "group1"},
	}, nil)
	s.resourceMock.On("GetActionList", mock.Anything, "rs1", &req.Resource.ID, mock.Anything, 0).
		Return(&resource.ActionList{
			Actions: []resource.Action{
				{Handle: "read", Permission: "booking:booking:read"},
				{Handle: "delete", Permission: "booking:booking:delete"},
			},
		}, nil)
	s.authzMock.On("EvaluateAccessBatch", mock.Anything,
		mock.MatchedBy(func(req authz.AccessEvaluationsRequest) bool {
			return len(req.Evaluations) == 2 &&
				req.Evaluations[0].Subject.ID == "user1" &&
				req.Evaluations[0].Subject.GroupIDs[0] == "group1" &&
				req.Evaluations[0].Resource.Type == "booking:booking" &&
				req.Evaluations[0].Resource.ID == "booking1" &&
				req.Evaluations[0].Action.Name == "read" &&
				req.Evaluations[1].Action.Name == "delete"
		})).
		Return(&authz.AccessEvaluationsResponse{
			Evaluations: []authz.AccessEvaluationResponse{
				{Decision: true},
				{Decision: false},
			},
		}, nil)

	resp, svcErr := s.service.SearchActions(context.Background(), req)

	s.Nil(svcErr)
	s.NotNil(resp)
	s.Len(resp.Results, 1)
	action, ok := resp.Results[0].(Action)
	s.True(ok)
	s.Equal("read", action.Name)
}

func (s *ServiceTestSuite) TestSearchActionsReturnsEmptyResultsWhenDenied() {
	req := AccessActionSearchRequest{
		Subject:  Subject{Type: "user", ID: "user1"},
		Resource: Resource{Type: "booking:booking", ID: "booking1"},
		Context:  map[string]interface{}{"resourceServerId": "rs1"},
	}

	s.entityProviderMock.On("GetTransitiveEntityGroups", "user1").Return([]entityprovider.EntityGroup{}, nil)
	s.resourceMock.On("GetActionList", mock.Anything, "rs1", &req.Resource.ID, mock.Anything, 0).
		Return(&resource.ActionList{
			Actions: []resource.Action{
				{Handle: "read", Permission: "booking:booking:read"},
			},
		}, nil)
	s.authzMock.On("EvaluateAccessBatch", mock.Anything, mock.Anything).
		Return(&authz.AccessEvaluationsResponse{
			Evaluations: []authz.AccessEvaluationResponse{{Decision: false}},
		}, nil)

	resp, svcErr := s.service.SearchActions(context.Background(), req)

	s.Nil(svcErr)
	s.NotNil(resp)
	s.Empty(resp.Results)
}

func (s *ServiceTestSuite) TestSearchActionsValidationErrors() {
	tests := []struct {
		name string
		req  AccessActionSearchRequest
		code string
	}{
		{
			name: "missing subject",
			req: AccessActionSearchRequest{
				Resource: Resource{Type: "booking:booking", ID: "booking1"},
				Context:  map[string]interface{}{"resourceServerId": "rs1"},
			},
			code: ErrorMissingSubject.Code,
		},
		{
			name: "missing resource",
			req: AccessActionSearchRequest{
				Subject: Subject{ID: "user1"},
				Context: map[string]interface{}{"resourceServerId": "rs1"},
			},
			code: ErrorMissingResource.Code,
		},
		{
			name: "missing resource id",
			req: AccessActionSearchRequest{
				Subject:  Subject{ID: "user1"},
				Resource: Resource{Type: "booking:booking"},
				Context:  map[string]interface{}{"resourceServerId": "rs1"},
			},
			code: ErrorMissingResourceID.Code,
		},
		{
			name: "missing resource server id",
			req: AccessActionSearchRequest{
				Subject:  Subject{ID: "user1"},
				Resource: Resource{Type: "booking:booking", ID: "booking1"},
			},
			code: ErrorMissingResourceServerID.Code,
		},
	}

	for _, tc := range tests {
		s.Run(tc.name, func() {
			resp, svcErr := s.service.SearchActions(context.Background(), tc.req)

			s.Nil(resp)
			s.NotNil(svcErr)
			s.Equal(tc.code, svcErr.Code)
		})
	}
}

func (s *ServiceTestSuite) TestSearchActionsResourceServiceError() {
	req := AccessActionSearchRequest{
		Subject:  Subject{Type: "user", ID: "user1"},
		Resource: Resource{Type: "booking:booking", ID: "booking1"},
		Context:  map[string]interface{}{"resourceServerId": "rs1"},
	}

	s.entityProviderMock.On("GetTransitiveEntityGroups", "user1").Return([]entityprovider.EntityGroup{}, nil)
	s.resourceMock.On("GetActionList", mock.Anything, "rs1", &req.Resource.ID, mock.Anything, 0).
		Return((*resource.ActionList)(nil), &serviceerror.InternalServerError)

	resp, svcErr := s.service.SearchActions(context.Background(), req)

	s.Nil(resp)
	s.NotNil(svcErr)
	s.Equal(serviceerror.InternalServerError.Code, svcErr.Code)
}

func (s *ServiceTestSuite) TestSearchActionsAuthorizationServiceError() {
	req := AccessActionSearchRequest{
		Subject:  Subject{Type: "user", ID: "user1"},
		Resource: Resource{Type: "booking:booking", ID: "booking1"},
		Context:  map[string]interface{}{"resourceServerId": "rs1"},
	}

	s.entityProviderMock.On("GetTransitiveEntityGroups", "user1").Return([]entityprovider.EntityGroup{}, nil)
	s.resourceMock.On("GetActionList", mock.Anything, "rs1", &req.Resource.ID, mock.Anything, 0).
		Return(&resource.ActionList{
			Actions: []resource.Action{
				{Handle: "read", Permission: "booking:booking:read"},
			},
		}, nil)
	s.authzMock.On("EvaluateAccessBatch", mock.Anything, mock.Anything).
		Return((*authz.AccessEvaluationsResponse)(nil), &serviceerror.InternalServerError)

	resp, svcErr := s.service.SearchActions(context.Background(), req)

	s.Nil(resp)
	s.NotNil(svcErr)
	s.Equal(serviceerror.InternalServerError.Code, svcErr.Code)
}
