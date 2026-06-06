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

	"github.com/thunder-id/thunderid/internal/authz"
	"github.com/thunder-id/thunderid/internal/entityprovider"
	"github.com/thunder-id/thunderid/internal/resource"
	"github.com/thunder-id/thunderid/internal/system/middleware"
)

// Initialize initializes the AuthZEN API adapter and registers its routes.
func Initialize(
	mux *http.ServeMux,
	authzService authz.AuthorizationServiceInterface,
	entityProvider entityprovider.EntityProviderInterface,
	resourceService resource.ResourceServiceInterface,
) ServiceInterface {
	service := newService(authzService, entityProvider, resourceService)
	handler := newHandler(service)
	registerRoutes(mux, handler)
	return service
}

func registerRoutes(mux *http.ServeMux, h *handler) {
	opts := middleware.CORSOptions{
		AllowedMethods:   []string{"POST"},
		AllowedHeaders:   middleware.DefaultAllowedHeaders,
		AllowCredentials: true,
		MaxAge:           600,
	}
	discoveryOpts := middleware.CORSOptions{
		AllowedMethods:   []string{"GET"},
		AllowedHeaders:   middleware.DefaultAllowedHeaders,
		AllowCredentials: false,
		MaxAge:           600,
	}

	mux.HandleFunc(middleware.WithCORS("GET /.well-known/authzen-configuration",
		h.HandleMetadataRequest, discoveryOpts))
	mux.HandleFunc(middleware.WithCORS("OPTIONS /.well-known/authzen-configuration",
		func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusNoContent)
		}, discoveryOpts))
	mux.HandleFunc(middleware.WithCORS("POST /access/v1/evaluation",
		h.HandleAccessEvaluationRequest, opts))
	mux.HandleFunc(middleware.WithCORS("OPTIONS /access/v1/evaluation",
		func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusNoContent)
		}, opts))
	mux.HandleFunc(middleware.WithCORS("POST /access/v1/evaluations",
		h.HandleAccessEvaluationsRequest, opts))
	mux.HandleFunc(middleware.WithCORS("OPTIONS /access/v1/evaluations",
		func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusNoContent)
		}, opts))
	mux.HandleFunc(middleware.WithCORS("POST /access/v1/search/action",
		h.HandleActionSearchRequest, opts))
	mux.HandleFunc(middleware.WithCORS("OPTIONS /access/v1/search/action",
		func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusNoContent)
		}, opts))
}
