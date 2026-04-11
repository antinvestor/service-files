# antinvestor_ui_files

Embeddable file management UI for Antinvestor applications. Provides screens and widgets for uploading, browsing, access control, versioning, and retention management.

## Installation

```yaml
dependencies:
  antinvestor_ui_files: ^0.1.0
```

## Features

- **File Browser**: Browse and search files with preview cards
- **File Upload**: Upload with progress tracking and metadata
- **File Detail**: View file info, preview, versions, and access control
- **Access Control**: Role-based file access management
- **Version History**: Version timeline with diff support
- **Retention Policies**: Configure file retention and cleanup rules
- **Storage Dashboard**: Storage usage metrics and analytics
- **Embeddable Widgets**: `FilePreviewCard`, `FileThumbnailWidget`, `FileAttachmentChip`, `FileAccessChip`, `MediaStateBadge`, `ScanStatusBadge`, `UploadProgressIndicator`, `StorageUsageBar`, `VersionHistoryTimeline`, `AccessRoleBadge`
- **Routing**: `FilesRouteModule` with GoRouter integration

## Usage

```dart
import 'package:antinvestor_ui_files/antinvestor_ui_files.dart';

// File thumbnail for inline display
FileThumbnailWidget(contentId: 'file-abc')

// File attachment chip for forms
FileAttachmentChip(contentId: 'file-abc', onRemove: () {})

// Register routes in your host app
final module = FilesRouteModule();
ShellRoute(
  routes: [...ownRoutes, ...module.buildRoutes()],
);
```

## Routes

| Path | Screen |
|------|--------|
| `/files` | File browser |
| `/files/upload` | Upload new file |
| `/files/storage` | Storage dashboard |
| `/files/retention` | Retention policies |
| `/files/:contentId` | File detail |
| `/files/:contentId/access` | Access management |

## Embedding Widgets

```dart
// File preview with thumbnail
FilePreviewCard(media: mediaObject)

// Upload progress bar
UploadProgressIndicator(uploadState: state)

// Storage usage visualization
StorageUsageBar(used: usedBytes, total: totalBytes)

// Version history
VersionHistoryTimeline(contentId: 'file-abc')
```
