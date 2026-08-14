// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package authz

import (
	"github.com/thunder-id/thunderid/internal/authz/engine"
	userpkg "github.com/thunder-id/thunderid/internal/user"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// Initialize creates and initializes the authorization service with the selected engine.
func Initialize(authorizationEngine engine.AuthorizationEngine, userServices ...userpkg.UserServiceInterface) providers.AuthorizationProvider {
	return newAuthorizationService(authorizationEngine, userServices...)
}
