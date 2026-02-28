package authz

import (
	"context"
	"errors"

	"github.com/antinvestor/service-files/apps/default/service/storage"
	"github.com/antinvestor/service-files/apps/default/service/types"
	"github.com/pitabwire/frame/security"
	"github.com/pitabwire/frame/security/authorizer"
	"github.com/pitabwire/util"
)

var (
	ErrNotOwner        = errors.New("only owner can perform this action")
	ErrInvalidRelation = errors.New("invalid relation")
	ErrNotFound        = errors.New("file not found")
)

type AccessGrantInfo struct {
	PrincipalID string
	Role        string
}

type Middleware interface {
	CanViewFile(ctx context.Context, profileID, fileID string) error
	CanEditFile(ctx context.Context, profileID, fileID string) error
	CanDeleteFile(ctx context.Context, profileID, fileID string) error
	CanUploadFile(ctx context.Context, profileID string) error

	GrantFileAccess(ctx context.Context, ownerProfileID, fileID, targetProfileID, role string) error
	RevokeFileAccess(ctx context.Context, ownerProfileID, fileID, targetProfileID string) error

	GetFileOwner(ctx context.Context, fileID string) (string, error)
	ListSharedWith(ctx context.Context, ownerProfileID, fileID string) ([]string, error)
	ListUserShares(ctx context.Context, profileID string) ([]string, error)
	ListFileAccessGrants(ctx context.Context, ownerProfileID, fileID string) ([]AccessGrantInfo, error)
}

type middleware struct {
	authorizer    security.Authorizer
	mediaDB       storage.Database
	servicePolicy ServicePolicy
}

func NewMiddleware(authorizer security.Authorizer, mediaDB storage.Database) Middleware {
	return &middleware{
		authorizer:    authorizer,
		mediaDB:       mediaDB,
		servicePolicy: DefaultServicePolicy(),
	}
}

// NewMiddlewareWithPolicy allows injecting a custom service policy (for tests).
func NewMiddlewareWithPolicy(authorizer security.Authorizer, mediaDB storage.Database, policy ServicePolicy) Middleware {
	return &middleware{authorizer: authorizer, mediaDB: mediaDB, servicePolicy: policy}
}

func (m *middleware) CanViewFile(ctx context.Context, profileID, fileID string) error {
	return m.checkFilePermission(ctx, profileID, fileID, PermissionView)
}

func (m *middleware) CanEditFile(ctx context.Context, profileID, fileID string) error {
	return m.checkFilePermission(ctx, profileID, fileID, PermissionEdit)
}

func (m *middleware) CanDeleteFile(ctx context.Context, profileID, fileID string) error {
	return m.checkFilePermission(ctx, profileID, fileID, PermissionDelete)
}

func (m *middleware) CanUploadFile(ctx context.Context, profileID string) error {
	if profileID == "" {
		return authorizer.ErrInvalidSubject
	}

	if svcName := internalServiceName(ctx); svcName != "" {
		if !m.servicePolicy.HasScope(svcName, ServiceScopeWrite) {
			return authorizer.NewPermissionDeniedError(
				security.ObjectRef{Namespace: NamespaceFile},
				PermissionUpload,
				security.SubjectRef{Namespace: NamespaceProfile, ID: profileID},
				"service lacks write scope",
			)
		}
	}

	return nil
}

func (m *middleware) checkFilePermission(ctx context.Context, profileID, fileID, permission string) error {
	if profileID == "" {
		return authorizer.ErrInvalidSubject
	}

	log := util.Log(ctx).WithFields(map[string]any{
		"profile_id": profileID,
		"file_id":    fileID,
		"permission": permission,
	})

	// Check if caller is an authorized internal service
	if svcName := internalServiceName(ctx); svcName != "" {
		requiredScope := permissionToScope(permission)
		if m.servicePolicy.HasScope(svcName, requiredScope) {
			log.WithField("service_name", svcName).Debug("internal service granted access")
			return nil
		}
		// Unknown service or insufficient scope — fall through to normal auth
	}

	ownerID, err := m.GetFileOwner(ctx, fileID)
	if err != nil {
		log.WithError(err).Error("failed to get file owner")
		return err
	}

	if profileID == ownerID {
		log.Debug("user is owner, granting access")
		return nil
	}

	req := security.CheckRequest{
		Object:     security.ObjectRef{Namespace: NamespaceFile, ID: fileID},
		Permission: permission,
		Subject:    security.SubjectRef{Namespace: NamespaceProfile, ID: profileID},
	}

	result, err := m.authorizer.Check(ctx, req)
	if err != nil {
		log.WithError(err).Error("authorization check failed")
		return err
	}

	if !result.Allowed {
		return authorizer.NewPermissionDeniedError(
			req.Object,
			permission,
			req.Subject,
			result.Reason,
		)
	}

	return nil
}

func (m *middleware) GrantFileAccess(ctx context.Context, ownerProfileID, fileID, targetProfileID, role string) error {
	log := util.Log(ctx).WithFields(map[string]any{
		"owner_profile_id":  ownerProfileID,
		"file_id":           fileID,
		"target_profile_id": targetProfileID,
		"role":              role,
	})

	isOwner, err := m.isOwner(ctx, ownerProfileID, fileID)
	if err != nil {
		log.WithError(err).Error("failed to verify ownership")
		return err
	}

	if !isOwner {
		return ErrNotOwner
	}

	relation := RoleToRelation(role)
	if relation == "" {
		return ErrInvalidRelation
	}

	return m.authorizer.WriteTuple(ctx, security.RelationTuple{
		Object:   security.ObjectRef{Namespace: NamespaceFile, ID: fileID},
		Relation: relation,
		Subject:  security.SubjectRef{Namespace: NamespaceProfile, ID: targetProfileID},
	})
}

func (m *middleware) RevokeFileAccess(ctx context.Context, ownerProfileID, fileID, targetProfileID string) error {
	log := util.Log(ctx).WithFields(map[string]any{
		"owner_profile_id":  ownerProfileID,
		"file_id":           fileID,
		"target_profile_id": targetProfileID,
	})

	isOwner, err := m.isOwner(ctx, ownerProfileID, fileID)
	if err != nil {
		log.WithError(err).Error("failed to verify ownership")
		return err
	}

	if !isOwner {
		return ErrNotOwner
	}

	relations := []string{RelationViewer, RelationEditor, RelationUploader}
	tuples := make([]security.RelationTuple, len(relations))
	for i, rel := range relations {
		tuples[i] = security.RelationTuple{
			Object:   security.ObjectRef{Namespace: NamespaceFile, ID: fileID},
			Relation: rel,
			Subject:  security.SubjectRef{Namespace: NamespaceProfile, ID: targetProfileID},
		}
	}

	return m.authorizer.DeleteTuples(ctx, tuples)
}

func (m *middleware) GetFileOwner(ctx context.Context, fileID string) (string, error) {
	metadata, err := m.mediaDB.GetMediaMetadata(ctx, types.MediaID(fileID))
	if err != nil {
		return "", err
	}

	if metadata == nil {
		return "", ErrNotFound
	}

	return string(metadata.OwnerID), nil
}

func (m *middleware) isOwner(ctx context.Context, profileID, fileID string) (bool, error) {
	ownerID, err := m.GetFileOwner(ctx, fileID)
	if err != nil {
		return false, err
	}
	return profileID == ownerID, nil
}

func (m *middleware) ListSharedWith(ctx context.Context, ownerProfileID, fileID string) ([]string, error) {
	isOwner, err := m.isOwner(ctx, ownerProfileID, fileID)
	if err != nil {
		return nil, err
	}

	if !isOwner {
		return nil, ErrNotOwner
	}

	relations := []string{RelationViewer, RelationEditor, RelationUploader}
	var allSubjects []string

	for _, rel := range relations {
		tuples, err := m.authorizer.ListSubjectRelations(ctx,
			security.SubjectRef{Namespace: NamespaceProfile},
			NamespaceFile+":"+fileID+":"+rel,
		)
		if err != nil {
			continue
		}

		for _, t := range tuples {
			allSubjects = append(allSubjects, t.Subject.ID)
		}
	}

	return allSubjects, nil
}

func (m *middleware) ListUserShares(ctx context.Context, profileID string) ([]string, error) {
	tuples, err := m.authorizer.ListSubjectRelations(ctx,
		security.SubjectRef{Namespace: NamespaceProfile, ID: profileID},
		NamespaceFile,
	)
	if err != nil {
		return nil, err
	}

	files := make([]string, len(tuples))
	for i, t := range tuples {
		files[i] = t.Object.ID
	}

	return files, nil
}

func (m *middleware) ListFileAccessGrants(ctx context.Context, ownerProfileID, fileID string) ([]AccessGrantInfo, error) {
	isOwner, err := m.isOwner(ctx, ownerProfileID, fileID)
	if err != nil {
		return nil, err
	}

	if !isOwner {
		return nil, ErrNotOwner
	}

	relations := []string{RelationViewer, RelationEditor, RelationUploader}
	var grants []AccessGrantInfo

	for _, rel := range relations {
		tuples, err := m.authorizer.ListSubjectRelations(ctx,
			security.SubjectRef{Namespace: NamespaceProfile},
			NamespaceFile+":"+fileID+":"+rel,
		)
		if err != nil {
			continue
		}

		role := RelationToRole(rel)
		for _, t := range tuples {
			grants = append(grants, AccessGrantInfo{
				PrincipalID: t.Subject.ID,
				Role:        role,
			})
		}
	}

	return grants, nil
}
