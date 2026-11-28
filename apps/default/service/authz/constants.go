package authz

const (
	NamespaceFile          = "file"
	NamespaceProfile       = "profile_user"
	NamespaceTenancyAccess = "tenancy_access"
)

// Functional (Plane-2) permissions in NamespaceServiceFile.
const (
	PermissionContentView   = "content_view"
	PermissionContentUpload = "content_upload"
	PermissionContentManage = "content_manage"
	PermissionContentDelete = "content_delete"
)

const (
	PermissionView   = "view"
	PermissionUpload = "upload"
	PermissionEdit   = "edit"
	PermissionDelete = "delete"
)

// Service permission scopes for internal service-to-service calls.
const (
	ServiceScopeRead  = "read"  // view/download files
	ServiceScopeWrite = "write" // upload files
	ServiceScopeAdmin = "admin" // access admin endpoints (e.g. GetStorageStats)
)

const (
	RelationOwner    = "granted_owner"
	RelationViewer   = "granted_viewer"
	RelationEditor   = "granted_editor"
	RelationUploader = "granted_uploader"
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

func RelationToRole(relation string) string {
	switch relation {
	case RelationOwner:
		return "owner"
	case RelationViewer:
		return "viewer"
	case RelationEditor:
		return "editor"
	case RelationUploader:
		return "uploader"
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
