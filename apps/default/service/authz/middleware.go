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
}

type middleware struct {
	authorizer security.Authorizer
	mediaDB    storage.Database
}

func NewMiddleware(authorizer security.Authorizer, mediaDB storage.Database) Middleware {
	return &middleware{
		authorizer: authorizer,
		mediaDB:    mediaDB,
	}
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
