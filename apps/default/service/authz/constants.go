package authz

const (
	NamespaceFile    = "file"
	NamespaceProfile = "profile"
)

const (
	PermissionView   = "view"
	PermissionUpload = "upload"
	PermissionEdit   = "edit"
	PermissionDelete = "delete"
)

const (
	RelationOwner    = "owner"
	RelationViewer   = "viewer"
	RelationEditor   = "editor"
	RelationUploader = "uploader"
)

func RoleToRelation(role string) string {
	switch role {
	case "owner":
		return RelationOwner
	case "editor":
		return RelationEditor
	case "viewer":
		return RelationViewer
	case "uploader":
		return RelationUploader
	default:
		return ""
	}
}

func RelationToPermission(relation string) string {
	switch relation {
	case RelationOwner:
		return PermissionDelete
	case RelationEditor:
		return PermissionEdit
	case RelationViewer:
		return PermissionView
	case RelationUploader:
		return PermissionUpload
	default:
		return ""
	}
}
