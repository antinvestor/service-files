// Keto Namespace Configuration for Service Files
// Using Ory Permission Language (OPL) - TypeScript-like DSL
//
// This file defines the authorization model for the file service.
// It uses a Zanzibar-style relationship-based access control (ReBAC) model.
//
// NOTE: Class names are prefixed with "file_" to avoid collisions with other
// services sharing the same Keto instance. Keto uses the exact class name as
// the namespace identifier in API calls, so these must match the Go constants
// (e.g., NamespaceFile = "file", NamespaceProfile = "profile_user").
//
// IMPORTANT: Direct grant relations are prefixed with "granted_" to avoid
// name conflicts with permit functions. Keto skips permit evaluation when
// a relation shares the same name as a permit function.

import { Namespace, Context } from "@ory/keto-namespace-types"

// profile_user namespace represents users/actors in the file system
// This is the subject that performs actions on files
class profile_user implements Namespace {
  related: {
    self: profile_user[]
  }
}

// file namespace represents individual files/media with ownership and sharing
// Supports viewer, editor, uploader roles in addition to owner
class file implements Namespace {
  related: {
    // Owner has full control over the file
    granted_owner: profile_user[]
    // Viewers can only read the file
    granted_viewer: profile_user[]
    // Editors can modify file metadata (not the file content itself)
    granted_editor: profile_user[]
    // Uploaders can upload new versions of the file
    granted_uploader: profile_user[]
  }

  permits = {
    // View/download the file content
    view: (ctx: Context): boolean =>
      this.related.granted_owner.includes(ctx.subject) ||
      this.related.granted_viewer.includes(ctx.subject),

    // Edit file metadata (name, description, etc.)
    edit: (ctx: Context): boolean =>
      this.related.granted_owner.includes(ctx.subject) ||
      this.related.granted_editor.includes(ctx.subject),

    // Delete the file entirely
    delete: (ctx: Context): boolean =>
      this.related.granted_owner.includes(ctx.subject),

    // Upload a new version of the file
    upload: (ctx: Context): boolean =>
      this.related.granted_owner.includes(ctx.subject) ||
      this.related.granted_uploader.includes(ctx.subject),

    // Get storage usage statistics for this file
    stats: (ctx: Context): boolean =>
      this.related.granted_owner.includes(ctx.subject),

    // Manage sharing - add/remove viewers, editors, uploaders
    share: (ctx: Context): boolean =>
      this.related.granted_owner.includes(ctx.subject),

    // Update retention policy on the file
    retention_set: (ctx: Context): boolean =>
      this.related.granted_owner.includes(ctx.subject),

    // Lock/unlock file for retention
    lock: (ctx: Context): boolean =>
      this.related.granted_owner.includes(ctx.subject),
  }
}

// file_version namespace represents historical versions of files
// Inherits permissions from the parent file
class file_version implements Namespace {
  related: {
    // The file this version belongs to
    parent: file[]
    // The profile that created this version
    creator: profile_user[]
  }

  permits = {
    // View this specific version
    view: (ctx: Context): boolean =>
      this.related.parent.traverse((f) => f.permits.view(ctx)),

    // Delete/restore this version (owner only)
    delete: (ctx: Context): boolean =>
      this.related.parent.traverse((f) => f.permits.delete(ctx)),

    // Restore this version as the current version
    restore: (ctx: Context): boolean =>
      this.related.parent.traverse((f) => f.permits.edit(ctx)),
  }
}

// file_retention_policy namespace represents retention policies
// that can be applied to files
class file_retention_policy implements Namespace {
  related: {
    // The owner who created this policy
    granted_owner: profile_user[]
    // Files that have this policy applied
    files: file[]
  }

  permits = {
    // View the policy details
    view: (ctx: Context): boolean =>
      this.related.granted_owner.includes(ctx.subject),

    // Update policy (name, description, retention days)
    update: (ctx: Context): boolean =>
      this.related.granted_owner.includes(ctx.subject),

    // Delete the policy
    delete: (ctx: Context): boolean =>
      this.related.granted_owner.includes(ctx.subject),

    // Apply this policy to files
    apply: (ctx: Context): boolean =>
      this.related.granted_owner.includes(ctx.subject),
  }
}

// file_thumbnail namespace represents thumbnails generated from files
// Permissions inherit from the parent file
class file_thumbnail implements Namespace {
  related: {
    // The source file this thumbnail was generated from
    parent: file[]
  }

  permits = {
    // View the thumbnail
    view: (ctx: Context): boolean =>
      this.related.parent.traverse((f) => f.permits.view(ctx)),

    // Regenerate the thumbnail
    regenerate: (ctx: Context): boolean =>
      this.related.parent.traverse((f) => f.permits.edit(ctx)),
  }
}

// file_upload namespace represents multipart upload sessions
// Tracks in-progress uploads
class file_upload implements Namespace {
  related: {
    // The profile initiating the upload
    granted_uploader: profile_user[]
    // The file being uploaded (may not exist yet)
    target_file: file[]
  }

  permits = {
    // Continue/resume the upload
    write: (ctx: Context): boolean =>
      this.related.granted_uploader.includes(ctx.subject),

    // Complete the upload
    complete: (ctx: Context): boolean =>
      this.related.granted_uploader.includes(ctx.subject),

    // Cancel/abort the upload
    cancel: (ctx: Context): boolean =>
      this.related.granted_uploader.includes(ctx.subject),

    // View upload status
    status: (ctx: Context): boolean =>
      this.related.granted_uploader.includes(ctx.subject),
  }
}

// Export namespaces for Keto to use
export {
  profile_user,
  file,
  file_version,
  file_retention_policy,
  file_thumbnail,
  file_upload,
}
