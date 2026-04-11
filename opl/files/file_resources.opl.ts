// Copyright 2023-2026 Ant Investor Ltd
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// ---------------------------------------------------------------------------
// Resource-level OPL definitions for files.v1
//
// These namespaces define per-resource access control for file objects.
// They live alongside the proto definitions so other services that reference
// file resources (e.g., chat service attaching files to messages) can
// discover and reuse these namespace definitions.
//
// The service-level namespace (service_file) with functional role grants
// is generated from proto annotations and lives in apps/default/.
// ---------------------------------------------------------------------------

import { Namespace, Context } from "@ory/keto-namespace-types"

// ---------------------------------------------------------------------------
// Plane 0 -- Platform identity (shared across all services)
// ---------------------------------------------------------------------------

class profile_user implements Namespace {
  related: {
    self: profile_user[]
  }
}

// ---------------------------------------------------------------------------
// Plane 3 -- Per-resource namespaces (file, file_version, etc.)
// These enforce fine-grained, object-level access control.
// ---------------------------------------------------------------------------

// file namespace represents individual files/media with ownership and sharing.
// Supports viewer, editor, uploader roles in addition to owner.
class file implements Namespace {
  related: {
    granted_owner: profile_user[]
    granted_viewer: profile_user[]
    granted_editor: profile_user[]
    granted_uploader: profile_user[]
  }

  permits = {
    view: (ctx: Context): boolean =>
      this.related.granted_owner.includes(ctx.subject) ||
      this.related.granted_viewer.includes(ctx.subject),

    edit: (ctx: Context): boolean =>
      this.related.granted_owner.includes(ctx.subject) ||
      this.related.granted_editor.includes(ctx.subject),

    delete: (ctx: Context): boolean =>
      this.related.granted_owner.includes(ctx.subject),

    upload: (ctx: Context): boolean =>
      this.related.granted_owner.includes(ctx.subject) ||
      this.related.granted_uploader.includes(ctx.subject),

    stats: (ctx: Context): boolean =>
      this.related.granted_owner.includes(ctx.subject),

    share: (ctx: Context): boolean =>
      this.related.granted_owner.includes(ctx.subject),

    retention_set: (ctx: Context): boolean =>
      this.related.granted_owner.includes(ctx.subject),

    lock: (ctx: Context): boolean =>
      this.related.granted_owner.includes(ctx.subject),
  }
}

// file_version namespace represents historical versions of files.
// Inherits permissions from the parent file.
class file_version implements Namespace {
  related: {
    parent: file[]
    creator: profile_user[]
  }

  permits = {
    view: (ctx: Context): boolean =>
      this.related.parent.traverse((f) => f.permits.view(ctx)),

    delete: (ctx: Context): boolean =>
      this.related.parent.traverse((f) => f.permits.delete(ctx)),

    restore: (ctx: Context): boolean =>
      this.related.parent.traverse((f) => f.permits.edit(ctx)),
  }
}

// file_retention_policy namespace represents retention policies
// that can be applied to files.
class file_retention_policy implements Namespace {
  related: {
    granted_owner: profile_user[]
    files: file[]
  }

  permits = {
    view: (ctx: Context): boolean =>
      this.related.granted_owner.includes(ctx.subject),

    update: (ctx: Context): boolean =>
      this.related.granted_owner.includes(ctx.subject),

    delete: (ctx: Context): boolean =>
      this.related.granted_owner.includes(ctx.subject),

    apply: (ctx: Context): boolean =>
      this.related.granted_owner.includes(ctx.subject),
  }
}

// file_thumbnail namespace represents thumbnails generated from files.
// Permissions inherit from the parent file.
class file_thumbnail implements Namespace {
  related: {
    parent: file[]
  }

  permits = {
    view: (ctx: Context): boolean =>
      this.related.parent.traverse((f) => f.permits.view(ctx)),

    regenerate: (ctx: Context): boolean =>
      this.related.parent.traverse((f) => f.permits.edit(ctx)),
  }
}

// file_upload namespace represents multipart upload sessions.
// Tracks in-progress uploads.
class file_upload implements Namespace {
  related: {
    granted_uploader: profile_user[]
    target_file: file[]
  }

  permits = {
    write: (ctx: Context): boolean =>
      this.related.granted_uploader.includes(ctx.subject),

    complete: (ctx: Context): boolean =>
      this.related.granted_uploader.includes(ctx.subject),

    cancel: (ctx: Context): boolean =>
      this.related.granted_uploader.includes(ctx.subject),

    status: (ctx: Context): boolean =>
      this.related.granted_uploader.includes(ctx.subject),
  }
}
