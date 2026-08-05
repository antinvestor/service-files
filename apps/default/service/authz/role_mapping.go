package authz

import (
	"context"

	"github.com/pitabwire/frame/v2/security"
	"github.com/pitabwire/util"
)

const (
	// NamespaceServiceFile is the Plane-2 functional namespace for files.
	NamespaceServiceFile = "service_file"
	// RoleMember is the Plane-1 relation for regular users.
	RoleMember = "member"
	// RoleService is the Plane-1 relation for internal service bots.
	RoleService = "service"
)

// BuildAccessTuple creates a tenancy_access#member tuple for a user.
func BuildAccessTuple(tenancyPath, profileID string) security.RelationTuple {
	return security.RelationTuple{
		Object:   security.ObjectRef{Namespace: NamespaceTenancyAccess, ID: tenancyPath},
		Relation: RoleMember,
		Subject:  security.SubjectRef{Namespace: NamespaceProfile, ID: profileID},
	}
}

// BuildServiceAccessTuple creates a tenancy_access#service tuple for a service bot.
func BuildServiceAccessTuple(tenancyPath, profileID string) security.RelationTuple {
	return security.RelationTuple{
		Object:   security.ObjectRef{Namespace: NamespaceTenancyAccess, ID: tenancyPath},
		Relation: RoleService,
		Subject:  security.SubjectRef{Namespace: NamespaceProfile, ID: profileID},
	}
}

// HealServiceTenancyAccess provisions a missing Plane-1 #service tuple when an
// internal system caller is denied tenancy access. Wire with
// authorizer.WithOnTenancyAccessDenied(authz.HealServiceTenancyAccess).
// Non-internal denials are logged only (no grant).
func HealServiceTenancyAccess(
	ctx context.Context,
	auth security.Authorizer,
	tenancyPath, subjectID string,
) error {
	fields := map[string]any{
		"tenant_id":  tenancyPath,
		"subject_id": subjectID,
	}
	claims := security.ClaimsFromContext(ctx)
	if claims == nil || !claims.IsInternalSystem() {
		util.Log(ctx).WithFields(fields).Error("PERMISSION DENIED: tenancy access denied")
		return nil
	}
	if err := auth.WriteTuple(ctx, BuildServiceAccessTuple(tenancyPath, subjectID)); err != nil {
		util.Log(ctx).WithFields(fields).WithError(err).
			Error("PERMISSION DENIED: self-heal of tenancy service access failed")
		return err
	}
	util.Log(ctx).WithFields(fields).
		Info("self-healed missing tenancy service tuple for internal caller")
	return nil
}
