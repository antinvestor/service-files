package authz

import (
	"context"
	"strings"

	"github.com/pitabwire/frame/v3/security"
)

// ServicePermission defines what an internal service is allowed to do.
type ServicePermission struct {
	Scopes []string
}

// ServicePolicy maps service names (from JWT ServiceName claim) to permissions.
//
// The policy only ever adds access: a scope listed here is granted without a
// Keto check and across tenants. It is never a deny-list. A service that is
// not listed, or lacks the scope for an operation, is authorised by its
// explicit service_file grants like every other service account (see
// middleware.checkServiceGrant).
type ServicePolicy map[string]ServicePermission

// DefaultServicePolicy returns the built-in policy for known internal services.
func DefaultServicePolicy() ServicePolicy {
	return ServicePolicy{
		"service_ocr":      {Scopes: []string{ServiceScopeRead, ServiceScopeWrite}},
		"service_property": {Scopes: []string{ServiceScopeRead}},
	}
}

// HasScope checks if the given service has the specified scope.
func (sp ServicePolicy) HasScope(serviceName, scope string) bool {
	perm, ok := sp[serviceName]
	if !ok {
		return false
	}
	for _, s := range perm.Scopes {
		if s == scope {
			return true
		}
	}
	return false
}

// internalServiceName extracts the service name from context if the caller
// is an internal system (has system_internal role). Returns "" for normal users.
func internalServiceName(ctx context.Context) string {
	claims := security.ClaimsFromContext(ctx)
	if claims == nil {
		return ""
	}
	roles := claims.GetRoles()
	if len(roles) == 1 && strings.HasPrefix(roles[0], "internal") {
		return claims.GetServiceName()
	}
	return ""
}

// permissionToScope maps a file permission to a service scope.
func permissionToScope(permission string) string {
	switch permission {
	case PermissionView:
		return ServiceScopeRead
	case PermissionUpload:
		return ServiceScopeWrite
	case PermissionEdit:
		return ServiceScopeWrite
	case PermissionDelete:
		return ServiceScopeWrite
	default:
		return ""
	}
}

// permissionToServicePermission maps a per-file permission to the functional
// permission a service account needs in NamespaceServiceFile.
func permissionToServicePermission(permission string) string {
	switch permission {
	case PermissionView:
		return PermissionContentView
	case PermissionUpload:
		return PermissionContentUpload
	case PermissionEdit:
		return PermissionContentManage
	case PermissionDelete:
		return PermissionContentDelete
	default:
		return ""
	}
}
