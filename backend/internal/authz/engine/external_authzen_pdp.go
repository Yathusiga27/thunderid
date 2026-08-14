// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package engine

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/thunder-id/thunderid/internal/system/log"
)

// AuthZENPDPConfig configures an external AuthZEN access-evaluation endpoint.
type AuthZENPDPConfig struct {
	Endpoint                string
	BearerToken             string
	Timeout                 time.Duration
	RetryCount              int
	SubjectProperties       []string
	SubjectPropertyMappings map[string]string
	ResourceType            string
	ResourceID              string
	Client                  *http.Client
}

type authZENPDP struct {
	endpoint                string
	batchEndpoint           string
	bearerToken             string
	retryCount              int
	subjectProperties       map[string]struct{}
	subjectPropertyMappings map[string]string
	resourceType            string
	resourceID              string
	client                  *http.Client
	logger                  *log.Logger
}

const (
	resourceServerIdentifierProperty = "resourceServerIdentifier"
	subjectGroupsProperty            = "groups"
)

type authZENEvaluationRequest struct {
	Subject  authZENSubject         `json:"subject"`
	Resource authZENResource        `json:"resource"`
	Action   authZENAction          `json:"action"`
	Context  map[string]interface{} `json:"context,omitempty"`
}

type authZENSubject struct {
	Type       string                 `json:"type,omitempty"`
	ID         string                 `json:"id"`
	Properties map[string]interface{} `json:"properties,omitempty"`
}

type authZENResource struct {
	Type       string                 `json:"type"`
	ID         string                 `json:"id"`
	Properties map[string]interface{} `json:"properties,omitempty"`
}

type authZENAction struct {
	Name       string                 `json:"name"`
	Properties map[string]interface{} `json:"properties,omitempty"`
}

type authZENEvaluationResponse struct {
	Decision bool                   `json:"decision"`
	Context  map[string]interface{} `json:"context,omitempty"`
}

type authZENBatchEvaluationRequest struct {
	Evaluations []authZENBatchEvaluation `json:"evaluations"`
}

type authZENBatchEvaluation struct {
	Subject  authZENSubject         `json:"subject"`
	Resource authZENResource        `json:"resource"`
	Action   authZENAction          `json:"action"`
	Context  map[string]interface{} `json:"context,omitempty"`
}

type authZENBatchEvaluationResponse struct {
	Evaluations []authZENEvaluationResponse `json:"evaluations"`
}

// NewAuthZENPDP creates an AuthorizationEngine backed by an external AuthZEN PDP.
func NewAuthZENPDP(config AuthZENPDPConfig) (AuthorizationEngine, error) {
	endpoint := strings.TrimSpace(config.Endpoint)
	parsedEndpoint, err := url.Parse(endpoint)
	if err != nil || parsedEndpoint.Scheme == "" || parsedEndpoint.Host == "" {
		return nil, fmt.Errorf("invalid AuthZEN PDP endpoint")
	}

	client := config.Client
	if client == nil {
		client = &http.Client{Timeout: config.Timeout}
	}

	subjectProperties := make(map[string]struct{}, len(config.SubjectProperties))
	for _, property := range config.SubjectProperties {
		property = strings.TrimSpace(property)
		if property != "" {
			subjectProperties[property] = struct{}{}
		}
	}

	subjectPropertyMappings := make(map[string]string, len(config.SubjectPropertyMappings))
	for source, target := range config.SubjectPropertyMappings {
		source = strings.TrimSpace(source)
		target = strings.TrimSpace(target)
		if source == "" || target == "" {
			return nil, fmt.Errorf("subject property mappings must have non-empty names")
		}
		if _, allowed := subjectProperties[source]; !allowed {
			return nil, fmt.Errorf("subject property %q has a mapping but is not allowed", source)
		}
		subjectPropertyMappings[source] = target
	}

	return &authZENPDP{
		endpoint:                endpoint,
		batchEndpoint:           strings.TrimSuffix(endpoint, "/evaluation") + "/evaluations",
		bearerToken:             config.BearerToken,
		retryCount:              max(config.RetryCount, 0),
		subjectProperties:       subjectProperties,
		subjectPropertyMappings: subjectPropertyMappings,
		resourceType:            strings.TrimSpace(config.ResourceType),
		resourceID:              strings.TrimSpace(config.ResourceID),
		client:                  client,
		logger:                  log.GetLogger().With(log.String(log.LoggerKeyComponentName, "ExternalAuthZENPDP")),
	}, nil
}

// EvaluateAccess evaluates a single authorization request with the external AuthZEN PDP.
func (p *authZENPDP) EvaluateAccess(
	ctx context.Context,
	request AccessEvaluationRequest,
) (*AccessEvaluationResponse, error) {
	response, err := p.evaluateSingle(ctx, request)
	if err != nil {
		return nil, err
	}
	return response, nil
}

// EvaluateAccessBatch evaluates multiple authorization requests using AuthZEN's batch endpoint.
func (p *authZENPDP) EvaluateAccessBatch(
	ctx context.Context,
	request AccessEvaluationsRequest,
) (*AccessEvaluationsResponse, error) {
	if len(request.Evaluations) == 0 {
		return &AccessEvaluationsResponse{Evaluations: []AccessEvaluationResponse{}}, nil
	}
	return p.evaluateBatch(ctx, request)
}

// evaluateBatch converts ThunderID evaluations into AuthZEN batch payloads and maps responses back in order.
func (p *authZENPDP) evaluateBatch(
	ctx context.Context,
	request AccessEvaluationsRequest,
) (*AccessEvaluationsResponse, error) {
	started := time.Now()
	payload := authZENBatchEvaluationRequest{
		Evaluations: make([]authZENBatchEvaluation, 0, len(request.Evaluations)),
	}
	for _, evaluation := range request.Evaluations {
		converted := toAuthZENEvaluationRequest(evaluation, p.subjectProperties, p.subjectPropertyMappings, p.resourceType, p.resourceID)
		payload.Evaluations = append(payload.Evaluations, authZENBatchEvaluation{
			Subject:  converted.Subject,
			Resource: converted.Resource,
			Action:   converted.Action,
			Context:  evaluation.Context,
		})
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to encode AuthZEN batch request: %w", err)
	}

	var response authZENBatchEvaluationResponse
	if err := p.post(ctx, p.batchEndpoint, body, &response); err != nil {
		p.logger.Error(ctx, "External AuthZEN batch evaluation failed",
			log.String("pdp_endpoint", p.batchEndpoint),
			log.Int("evaluation_count", len(request.Evaluations)),
			log.Int("latency_ms", int(time.Since(started).Milliseconds())),
			log.Error(err),
		)
		return nil, err
	}
	if len(response.Evaluations) != len(request.Evaluations) {
		return nil, fmt.Errorf("AuthZEN PDP returned %d evaluations for %d requests",
			len(response.Evaluations), len(request.Evaluations))
	}

	for index, evaluation := range request.Evaluations {
		p.logger.Info(ctx, "External AuthZEN evaluation completed",
			log.String("pdp_endpoint", p.batchEndpoint),
			log.MaskedString("subject_id", evaluation.Subject.ID),
			log.String("resource_server_id", evaluation.ResourceServer.ID),
			log.String("action", evaluation.Permission.Name),
			log.Bool("decision", response.Evaluations[index].Decision),
			log.Int("latency_ms", int(time.Since(started).Milliseconds())),
		)
	}

	results := make([]AccessEvaluationResponse, 0, len(response.Evaluations))
	for _, evaluation := range response.Evaluations {
		results = append(results, AccessEvaluationResponse{
			Decision: evaluation.Decision,
			Context:  evaluation.Context,
		})
	}
	return &AccessEvaluationsResponse{Evaluations: results}, nil
}

// evaluateSingle sends one ThunderID authorization request to the configured AuthZEN PDP.
func (p *authZENPDP) evaluateSingle(
	ctx context.Context,
	evaluation AccessEvaluationRequest,
) (response *AccessEvaluationResponse, err error) {
	started := time.Now()
	decision := false
	defer func() {
		fields := []log.Field{
			log.String("pdp_endpoint", p.endpoint),
			log.MaskedString("subject_id", evaluation.Subject.ID),
			log.String("resource_server_id", evaluation.ResourceServer.ID),
			log.String("action", evaluation.Permission.Name),
			log.Bool("decision", decision),
			log.Int("latency_ms", int(time.Since(started).Milliseconds())),
		}
		if err != nil {
			p.logger.Error(ctx, "External AuthZEN evaluation failed", append(fields, log.Error(err))...)
			return
		}
		p.logger.Info(ctx, "External AuthZEN evaluation completed", fields...)
	}()

	payload, err := json.Marshal(toAuthZENEvaluationRequest(evaluation, p.subjectProperties, p.subjectPropertyMappings, p.resourceType, p.resourceID))
	if err != nil {
		return nil, fmt.Errorf("failed to encode AuthZEN request: %w", err)
	}

	attempts := p.retryCount + 1
	for attempt := 0; attempt < attempts; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.endpoint, bytes.NewReader(payload))
		if err != nil {
			return nil, fmt.Errorf("failed to create AuthZEN request: %w", err)
		}
		req.Header.Set("Content-Type", "application/json")
		if p.bearerToken != "" {
			req.Header.Set("Authorization", "Bearer "+p.bearerToken)
		}

		resp, err := p.client.Do(req)
		if err != nil {
			if ctx.Err() != nil || attempt == attempts-1 {
				return nil, fmt.Errorf("AuthZEN PDP request failed: %w", err)
			}
			continue
		}

		if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
			statusCode := resp.StatusCode
			_ = resp.Body.Close()
			if statusCode >= http.StatusInternalServerError && attempt < attempts-1 {
				continue
			}
			return nil, fmt.Errorf("AuthZEN PDP returned HTTP %d", statusCode)
		}

		var responseDecision authZENEvaluationResponse
		if err := json.NewDecoder(resp.Body).Decode(&responseDecision); err != nil {
			_ = resp.Body.Close()
			return nil, fmt.Errorf("failed to decode AuthZEN response: %w", err)
		}
		_ = resp.Body.Close()

		decision = responseDecision.Decision
		return &AccessEvaluationResponse{
			Decision: responseDecision.Decision,
			Context:  responseDecision.Context,
		}, nil
	}

	return nil, fmt.Errorf("AuthZEN PDP request failed after retries")
}

// post sends an AuthZEN JSON request and retries transient PDP or network failures.
func (p *authZENPDP) post(
	ctx context.Context,
	endpoint string,
	payload []byte,
	result interface{},
) error {
	attempts := p.retryCount + 1
	for attempt := 0; attempt < attempts; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
		if err != nil {
			return fmt.Errorf("failed to create AuthZEN request: %w", err)
		}
		req.Header.Set("Content-Type", "application/json")
		if p.bearerToken != "" {
			req.Header.Set("Authorization", "Bearer "+p.bearerToken)
		}

		resp, err := p.client.Do(req)
		if err != nil {
			if ctx.Err() != nil || attempt == attempts-1 {
				return fmt.Errorf("AuthZEN PDP request failed: %w", err)
			}
			continue
		}
		if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
			statusCode := resp.StatusCode
			_ = resp.Body.Close()
			if statusCode >= http.StatusInternalServerError && attempt < attempts-1 {
				continue
			}
			return fmt.Errorf("AuthZEN PDP returned HTTP %d", statusCode)
		}
		if err := json.NewDecoder(resp.Body).Decode(result); err != nil {
			_ = resp.Body.Close()
			return fmt.Errorf("failed to decode AuthZEN response: %w", err)
		}
		_ = resp.Body.Close()
		return nil
	}
	return fmt.Errorf("AuthZEN PDP request failed after retries")
}

// toAuthZENEvaluationRequest maps ThunderID's internal authorization request to AuthZEN's wire format.
func toAuthZENEvaluationRequest(
	evaluation AccessEvaluationRequest,
	allowedSubjectProperties map[string]struct{},
	subjectPropertyMappings map[string]string,
	configuredResourceType string,
	configuredResourceID string,
) authZENEvaluationRequest {
	resourceIdentifier := evaluation.ResourceServer.ID
	if configuredIdentifier, ok := evaluation.ResourceServer.Properties[resourceServerIdentifierProperty].(string); ok && configuredIdentifier != "" {
		resourceIdentifier = configuredIdentifier
	}
	resourceType := resourceIdentifier
	resourceID := resourceIdentifier
	if configuredResourceType != "" {
		resourceType = configuredResourceType
	}
	if configuredResourceID != "" {
		resourceID = configuredResourceID
	}

	return authZENEvaluationRequest{
		Subject: authZENSubject{
			Type:       evaluation.Subject.Type,
			ID:         evaluation.Subject.ID,
			Properties: authZENSubjectProperties(evaluation.Subject, allowedSubjectProperties, subjectPropertyMappings),
		},
		Resource: authZENResource{
			Type:       resourceType,
			ID:         resourceID,
			Properties: evaluation.ResourceServer.Properties,
		},
		Action: authZENAction{
			Name:       evaluation.Permission.Name,
			Properties: evaluation.Permission.Properties,
		},
		Context: evaluation.Context,
	}
}

// authZENSubjectProperties filters and optionally renames subject attributes before sending them to the PDP.
func authZENSubjectProperties(
	subject Subject,
	allowedSubjectProperties map[string]struct{},
	subjectPropertyMappings map[string]string,
) map[string]interface{} {
	properties := make(map[string]interface{}, len(allowedSubjectProperties)+1)
	for key, value := range subject.Properties {
		if len(allowedSubjectProperties) > 0 {
			if _, allowed := allowedSubjectProperties[key]; !allowed {
				continue
			}
		}
		propertyName := key
		if mappedName, mapped := subjectPropertyMappings[key]; mapped {
			propertyName = mappedName
		}
		properties[propertyName] = value
	}
	if len(subject.GroupIDs) > 0 {
		if _, exists := properties[subjectGroupsProperty]; !exists {
			properties[subjectGroupsProperty] = append([]string(nil), subject.GroupIDs...)
		}
	}
	if len(properties) == 0 {
		return nil
	}
	return properties
}
