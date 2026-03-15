package authz

import (
	"context"
	"testing"

	"github.com/golang-jwt/jwt/v5"
	"github.com/pitabwire/frame/security"
	"github.com/stretchr/testify/assert"
)

func TestServicePolicy_HasScope(t *testing.T) {
	policy := DefaultServicePolicy()

	tests := []struct {
		name        string
		serviceName string
		scope       string
		want        bool
	}{
		{"ocr_has_read", "service_ocr", ServiceScopeRead, true},
		{"ocr_has_write", "service_ocr", ServiceScopeWrite, true},
		{"ocr_no_admin", "service_ocr", ServiceScopeAdmin, false},
		{"property_has_read", "service_property", ServiceScopeRead, true},
		{"property_no_write", "service_property", ServiceScopeWrite, false},
		{"unknown_service", "service_unknown", ServiceScopeRead, false},
		{"empty_service", "", ServiceScopeRead, false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := policy.HasScope(tc.serviceName, tc.scope)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestInternalServiceName(t *testing.T) {
	tests := []struct {
		name   string
		claims *security.AuthenticationClaims
		want   string
	}{
		{
			name:   "nil_claims",
			claims: nil,
			want:   "",
		},
		{
			name: "internal_service",
			claims: &security.AuthenticationClaims{
				RegisteredClaims: jwt.RegisteredClaims{Subject: "service_ocr"},
				ServiceName:      "service_ocr",
				Roles:            []string{"internal"},
			},
			want: "service_ocr",
		},
		{
			name: "regular_user",
			claims: &security.AuthenticationClaims{
				RegisteredClaims: jwt.RegisteredClaims{Subject: "user123"},
				Roles:            []string{"user"},
			},
			want: "",
		},
		{
			name: "multiple_roles",
			claims: &security.AuthenticationClaims{
				RegisteredClaims: jwt.RegisteredClaims{Subject: "service_ocr"},
				ServiceName:      "service_ocr",
				Roles:            []string{"internal", "admin"},
			},
			want: "",
		},
		{
			name: "no_roles",
			claims: &security.AuthenticationClaims{
				RegisteredClaims: jwt.RegisteredClaims{Subject: "user123"},
			},
			want: "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			if tc.claims != nil {
				ctx = tc.claims.ClaimsToContext(ctx)
			}
			got := internalServiceName(ctx)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestPermissionToScope(t *testing.T) {
	tests := []struct {
		permission string
		want       string
	}{
		{PermissionView, ServiceScopeRead},
		{PermissionUpload, ServiceScopeWrite},
		{PermissionEdit, ServiceScopeWrite},
		{PermissionDelete, ServiceScopeWrite},
		{"unknown", ""},
	}

	for _, tc := range tests {
		t.Run(tc.permission, func(t *testing.T) {
			got := permissionToScope(tc.permission)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestDefaultServicePolicy(t *testing.T) {
	policy := DefaultServicePolicy()
	assert.Len(t, policy, 2)
	assert.Contains(t, policy, "service_ocr")
	assert.Contains(t, policy, "service_property")
}
