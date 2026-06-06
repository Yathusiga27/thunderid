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
	"net/http"

	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/error/apierror"
	"github.com/thunder-id/thunderid/internal/system/error/serviceerror"
	sysutils "github.com/thunder-id/thunderid/internal/system/utils"
)

type handler struct {
	service ServiceInterface
}

func newHandler(service ServiceInterface) *handler {
	return &handler{service: service}
}

// HandleMetadataRequest handles AuthZEN PDP metadata discovery requests.
func (h *handler) HandleMetadataRequest(w http.ResponseWriter, _ *http.Request) {
	baseURL := config.GetServerURL(&config.GetServerRuntime().Config.Server)
	resp := MetadataResponse{
		PolicyDecisionPoint:       baseURL,
		AccessEvaluationEndpoint:  baseURL + "/access/v1/evaluation",
		AccessEvaluationsEndpoint: baseURL + "/access/v1/evaluations",
		SearchActionEndpoint:      baseURL + "/access/v1/search/action",
	}

	sysutils.WriteSuccessResponse(w, http.StatusOK, resp)
}

// HandleAccessEvaluationRequest handles a single AuthZEN access evaluation request.
func (h *handler) HandleAccessEvaluationRequest(w http.ResponseWriter, r *http.Request) {
	req, err := sysutils.DecodeJSONBody[AccessEvaluationRequest](r)
	if err != nil {
		handleError(w, &ErrorInvalidRequestFormat)
		return
	}

	resp, svcErr := h.service.EvaluateAccess(r.Context(), *req)
	if svcErr != nil {
		handleError(w, svcErr)
		return
	}

	sysutils.WriteSuccessResponse(w, http.StatusOK, resp)
}

// HandleAccessEvaluationsRequest handles a batched AuthZEN access evaluations request.
func (h *handler) HandleAccessEvaluationsRequest(w http.ResponseWriter, r *http.Request) {
	req, err := sysutils.DecodeJSONBody[AccessEvaluationsRequest](r)
	if err != nil {
		handleError(w, &ErrorInvalidRequestFormat)
		return
	}

	resp, svcErr := h.service.EvaluateAccessBatch(r.Context(), *req)
	if svcErr != nil {
		handleError(w, svcErr)
		return
	}

	sysutils.WriteSuccessResponse(w, http.StatusOK, resp)
}

// HandleActionSearchRequest handles an AuthZEN action search request.
func (h *handler) HandleActionSearchRequest(w http.ResponseWriter, r *http.Request) {
	req, err := sysutils.DecodeJSONBody[AccessActionSearchRequest](r)
	if err != nil {
		handleError(w, &ErrorInvalidRequestFormat)
		return
	}

	resp, svcErr := h.service.SearchActions(r.Context(), *req)
	if svcErr != nil {
		handleError(w, svcErr)
		return
	}

	sysutils.WriteSuccessResponse(w, http.StatusOK, resp)
}

func handleError(w http.ResponseWriter, svcErr *serviceerror.ServiceError) {
	statusCode := http.StatusInternalServerError
	if svcErr.Type == serviceerror.ClientErrorType {
		statusCode = http.StatusBadRequest
	}

	errResp := apierror.ErrorResponse{
		Code:        svcErr.Code,
		Message:     svcErr.Error,
		Description: svcErr.ErrorDescription,
	}

	sysutils.WriteErrorResponse(w, statusCode, errResp)
}
