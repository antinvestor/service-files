// This is a generated file - do not edit.
//
// Generated from files/v1/files.proto.

// @dart = 3.3

// ignore_for_file: annotate_overrides, camel_case_types, comment_references
// ignore_for_file: constant_identifier_names
// ignore_for_file: curly_braces_in_flow_control_structures
// ignore_for_file: deprecated_member_use_from_same_package, library_prefixes
// ignore_for_file: non_constant_identifier_names, prefer_relative_imports

import 'dart:core' as $core;

import 'package:protobuf/protobuf.dart' as $pb;

/// ThumbnailMethod defines how thumbnails should be generated from source media.
///
/// - SCALE: Scales the image to fit within the target dimensions while
///   preserving the original aspect ratio. The result may have dimensions
///   smaller than or equal to the requested size.
/// - CROP: Crops the image to exactly match the target dimensions. This
///   may result in losing some of the original image content.
///
/// Choosing between methods:
///   - Use SCALE for: photographs, images where all content matters
///   - Use CROP: avatars, preview tiles where exact dimensions are critical
/// buf:lint:ignore ENUM_VALUE_PREFIX
class ThumbnailMethod extends $pb.ProtobufEnum {
  /// buf:lint:ignore ENUM_ZERO_VALUE_SUFFIX
  /// Scale the image to fit within dimensions, preserving aspect ratio.
  /// The output dimensions will be <= the requested width/height.
  static const ThumbnailMethod SCALE =
      ThumbnailMethod._(0, _omitEnumNames ? '' : 'SCALE');

  /// Crop the image to exactly match the requested dimensions.
  /// May lose edge content; useful for avatars and uniform tiles.
  static const ThumbnailMethod CROP =
      ThumbnailMethod._(1, _omitEnumNames ? '' : 'CROP');

  static const $core.List<ThumbnailMethod> values = <ThumbnailMethod>[
    SCALE,
    CROP,
  ];

  static final $core.List<ThumbnailMethod?> _byValue =
      $pb.ProtobufEnum.$_initByValueList(values, 1);
  static ThumbnailMethod? valueOf($core.int value) =>
      value < 0 || value >= _byValue.length ? null : _byValue[value];

  const ThumbnailMethod._(super.value, super.name);
}

/// MediaState represents the lifecycle state of a media object.
///
/// State Transitions:
///   CREATING -> AVAILABLE (normal upload complete or finalize signed upload)
///   CREATING -> FAILED (upload failed or was aborted)
///   AVAILABLE -> ARCHIVED (retention policy or manual archive)
///   AVAILABLE -> DELETED (soft delete)
///   AVAILABLE -> CREATING (re-upload to same media_id)
///   ARCHIVED -> AVAILABLE (restore from archive)
///   DELETED -> AVAILABLE (restore from soft delete)
///   DELETED -> (hard delete after retention period expires)
///
/// IMPORTANT: Content in CREATING state cannot be downloaded. Clients must
/// poll or wait for AVAILABLE state before serving content.
class MediaState extends $pb.ProtobufEnum {
  /// Default state - should not be used explicitly.
  static const MediaState MEDIA_STATE_UNSPECIFIED =
      MediaState._(0, _omitEnumNames ? '' : 'MEDIA_STATE_UNSPECIFIED');

  /// Pre-created URI allocated, or multipart upload in progress.
  /// Content is not yet available for download. For signed uploads,
  /// FinalizeSignedUpload must be called to transition to AVAILABLE.
  static const MediaState MEDIA_STATE_CREATING =
      MediaState._(1, _omitEnumNames ? '' : 'MEDIA_STATE_CREATING');

  /// Upload complete and content is available for download.
  /// This is the only state from which content can be retrieved.
  static const MediaState MEDIA_STATE_AVAILABLE =
      MediaState._(2, _omitEnumNames ? '' : 'MEDIA_STATE_AVAILABLE');

  /// Content has been archived per retention policy or manual action.
  /// Archived content may require restoration before download.
  static const MediaState MEDIA_STATE_ARCHIVED =
      MediaState._(3, _omitEnumNames ? '' : 'MEDIA_STATE_ARCHIVED');

  /// Content has been soft-deleted. The data still exists but is hidden
  /// from normal queries. Can be restored via restore operation.
  /// Permanently removed after configured retention period if not restored.
  static const MediaState MEDIA_STATE_DELETED =
      MediaState._(4, _omitEnumNames ? '' : 'MEDIA_STATE_DELETED');

  /// Upload or processing failed. Content is not available.
  /// Check extra.error_message for failure details.
  static const MediaState MEDIA_STATE_FAILED =
      MediaState._(5, _omitEnumNames ? '' : 'MEDIA_STATE_FAILED');

  static const $core.List<MediaState> values = <MediaState>[
    MEDIA_STATE_UNSPECIFIED,
    MEDIA_STATE_CREATING,
    MEDIA_STATE_AVAILABLE,
    MEDIA_STATE_ARCHIVED,
    MEDIA_STATE_DELETED,
    MEDIA_STATE_FAILED,
  ];

  static final $core.List<MediaState?> _byValue =
      $pb.ProtobufEnum.$_initByValueList(values, 5);
  static MediaState? valueOf($core.int value) =>
      value < 0 || value >= _byValue.length ? null : _byValue[value];

  const MediaState._(super.value, super.name);
}

/// ScanStatus represents the antivirus/antimalware scan status of media.
///
/// Security Note: Download should be blocked for INFECTED content unless
/// the organization policy explicitly allows it. PENDING content should
/// also be blocked until scan completes with CLEAN status.
class ScanStatus extends $pb.ProtobufEnum {
  /// Scan status not determined or scan not requested.
  static const ScanStatus SCAN_STATUS_UNSPECIFIED =
      ScanStatus._(0, _omitEnumNames ? '' : 'SCAN_STATUS_UNSPECIFIED');

  /// Scan queued but not yet completed. Content should NOT be served
  /// to end users until scan completes with CLEAN status.
  static const ScanStatus SCAN_STATUS_PENDING =
      ScanStatus._(1, _omitEnumNames ? '' : 'SCAN_STATUS_PENDING');

  /// Antivirus scan completed and content verified safe.
  /// Content can be served to users.
  static const ScanStatus SCAN_STATUS_CLEAN =
      ScanStatus._(2, _omitEnumNames ? '' : 'SCAN_STATUS_CLEAN');

  /// Antivirus scan detected malware or potentially unwanted content.
  /// Content MUST NOT be served to users. Quarantine recommended.
  static const ScanStatus SCAN_STATUS_INFECTED =
      ScanStatus._(3, _omitEnumNames ? '' : 'SCAN_STATUS_INFECTED');

  /// Antivirus scan failed to complete (engine error, timeout, etc.).
  /// Content should be held in quarantine until manual review.
  static const ScanStatus SCAN_STATUS_FAILED =
      ScanStatus._(4, _omitEnumNames ? '' : 'SCAN_STATUS_FAILED');

  static const $core.List<ScanStatus> values = <ScanStatus>[
    SCAN_STATUS_UNSPECIFIED,
    SCAN_STATUS_PENDING,
    SCAN_STATUS_CLEAN,
    SCAN_STATUS_INFECTED,
    SCAN_STATUS_FAILED,
  ];

  static final $core.List<ScanStatus?> _byValue =
      $pb.ProtobufEnum.$_initByValueList(values, 4);
  static ScanStatus? valueOf($core.int value) =>
      value < 0 || value >= _byValue.length ? null : _byValue[value];

  const ScanStatus._(super.value, super.name);
}

/// AccessRole defines the role granted to a principal for accessing media.
///
/// Permission Matrix:
///   | Operation          | READER | WRITER | OWNER |
///   |-------------------|--------|--------|-------|
///   | GetContent        |   Y    |   Y    |   Y   |
///   | GetThumbnail      |   Y    |   Y    |   Y   |
///   | PatchContent      |   N    |   Y    |   Y   |
///   | DeleteContent     |   N    |   N    |   Y   |
///   | GrantAccess       |   N    |   N    |   Y   |
///   | RevokeAccess      |   N    |   N    |   Y   |
///   | SetRetention      |   N    |   N    |   Y   |
///   | RestoreVersion    |   N    |   Y    |   Y   |
///
/// OWNER role also includes ability to transfer ownership to another
/// principal and permanently delete (hard delete) content.
class AccessRole extends $pb.ProtobufEnum {
  /// Default role - no permissions.
  static const AccessRole ACCESS_ROLE_UNSPECIFIED =
      AccessRole._(0, _omitEnumNames ? '' : 'ACCESS_ROLE_UNSPECIFIED');

  /// Reader role: can read content, view metadata, list versions.
  /// Cannot modify, delete, or manage access.
  static const AccessRole ACCESS_ROLE_READER =
      AccessRole._(1, _omitEnumNames ? '' : 'ACCESS_ROLE_READER');

  /// Writer role: all reader permissions plus can upload new versions,
  /// patch metadata, and restore previous versions.
  /// Cannot delete, manage access, or change retention.
  static const AccessRole ACCESS_ROLE_WRITER =
      AccessRole._(2, _omitEnumNames ? '' : 'ACCESS_ROLE_WRITER');

  /// Owner role: full control including delete, access management,
  /// retention policy, and ownership transfer.
  static const AccessRole ACCESS_ROLE_OWNER =
      AccessRole._(3, _omitEnumNames ? '' : 'ACCESS_ROLE_OWNER');

  static const $core.List<AccessRole> values = <AccessRole>[
    ACCESS_ROLE_UNSPECIFIED,
    ACCESS_ROLE_READER,
    ACCESS_ROLE_WRITER,
    ACCESS_ROLE_OWNER,
  ];

  static final $core.List<AccessRole?> _byValue =
      $pb.ProtobufEnum.$_initByValueList(values, 3);
  static AccessRole? valueOf($core.int value) =>
      value < 0 || value >= _byValue.length ? null : _byValue[value];

  const AccessRole._(super.value, super.name);
}

/// DeleteOutcome represents the outcome of a delete operation.
///
/// This field clarifies what happened during deletion, which is important
/// for client logic and audit trails.
class DeleteOutcome extends $pb.ProtobufEnum {
  /// Outcome not specified - should not occur.
  static const DeleteOutcome DELETE_OUTCOME_UNSPECIFIED =
      DeleteOutcome._(0, _omitEnumNames ? '' : 'DELETE_OUTCOME_UNSPECIFIED');

  /// Content was soft-deleted. Data still exists but marked as DELETED.
  /// Can be restored within the retention period.
  static const DeleteOutcome DELETE_OUTCOME_SOFT =
      DeleteOutcome._(1, _omitEnumNames ? '' : 'DELETE_OUTCOME_SOFT');

  /// Content was permanently deleted. Data is irretrievably lost.
  static const DeleteOutcome DELETE_OUTCOME_HARD =
      DeleteOutcome._(2, _omitEnumNames ? '' : 'DELETE_OUTCOME_HARD');

  /// Delete was denied by retention policy. The content is under
  /// retention protection and cannot be deleted until the retention
  /// period expires.
  static const DeleteOutcome DELETE_OUTCOME_DENIED_BY_RETENTION =
      DeleteOutcome._(
          3, _omitEnumNames ? '' : 'DELETE_OUTCOME_DENIED_BY_RETENTION');

  static const $core.List<DeleteOutcome> values = <DeleteOutcome>[
    DELETE_OUTCOME_UNSPECIFIED,
    DELETE_OUTCOME_SOFT,
    DELETE_OUTCOME_HARD,
    DELETE_OUTCOME_DENIED_BY_RETENTION,
  ];

  static final $core.List<DeleteOutcome?> _byValue =
      $pb.ProtobufEnum.$_initByValueList(values, 3);
  static DeleteOutcome? valueOf($core.int value) =>
      value < 0 || value >= _byValue.length ? null : _byValue[value];

  const DeleteOutcome._(super.value, super.name);
}

/// MultipartUploadState represents the state of a multipart upload session.
class MultipartUploadState extends $pb.ProtobufEnum {
  static const MultipartUploadState MULTIPART_UPLOAD_STATE_UNSPECIFIED =
      MultipartUploadState._(
          0, _omitEnumNames ? '' : 'MULTIPART_UPLOAD_STATE_UNSPECIFIED');
  static const MultipartUploadState MULTIPART_UPLOAD_STATE_UPLOADING =
      MultipartUploadState._(
          1, _omitEnumNames ? '' : 'MULTIPART_UPLOAD_STATE_UPLOADING');
  static const MultipartUploadState MULTIPART_UPLOAD_STATE_COMPLETING =
      MultipartUploadState._(
          2, _omitEnumNames ? '' : 'MULTIPART_UPLOAD_STATE_COMPLETING');
  static const MultipartUploadState MULTIPART_UPLOAD_STATE_COMPLETED =
      MultipartUploadState._(
          3, _omitEnumNames ? '' : 'MULTIPART_UPLOAD_STATE_COMPLETED');
  static const MultipartUploadState MULTIPART_UPLOAD_STATE_ABORTED =
      MultipartUploadState._(
          4, _omitEnumNames ? '' : 'MULTIPART_UPLOAD_STATE_ABORTED');
  static const MultipartUploadState MULTIPART_UPLOAD_STATE_EXPIRED =
      MultipartUploadState._(
          5, _omitEnumNames ? '' : 'MULTIPART_UPLOAD_STATE_EXPIRED');

  static const $core.List<MultipartUploadState> values = <MultipartUploadState>[
    MULTIPART_UPLOAD_STATE_UNSPECIFIED,
    MULTIPART_UPLOAD_STATE_UPLOADING,
    MULTIPART_UPLOAD_STATE_COMPLETING,
    MULTIPART_UPLOAD_STATE_COMPLETED,
    MULTIPART_UPLOAD_STATE_ABORTED,
    MULTIPART_UPLOAD_STATE_EXPIRED,
  ];

  static final $core.List<MultipartUploadState?> _byValue =
      $pb.ProtobufEnum.$_initByValueList(values, 5);
  static MultipartUploadState? valueOf($core.int value) =>
      value < 0 || value >= _byValue.length ? null : _byValue[value];

  const MultipartUploadState._(super.value, super.name);
}

/// PrincipalType defines the type of principal in an access grant.
///
/// This helps the system understand how to resolve group membership
/// when checking access permissions.
class PrincipalType extends $pb.ProtobufEnum {
  /// Principal type not specified.
  static const PrincipalType PRINCIPAL_TYPE_UNSPECIFIED =
      PrincipalType._(0, _omitEnumNames ? '' : 'PRINCIPAL_TYPE_UNSPECIFIED');

  /// Individual user account.
  /// ID format: "user:<user_id>" or just "<user_id>"
  static const PrincipalType PRINCIPAL_TYPE_USER =
      PrincipalType._(1, _omitEnumNames ? '' : 'PRINCIPAL_TYPE_USER');

  /// Service account or application.
  /// ID format: "service:<service_name>"
  static const PrincipalType PRINCIPAL_TYPE_SERVICE =
      PrincipalType._(2, _omitEnumNames ? '' : 'PRINCIPAL_TYPE_SERVICE');

  /// Organization unit (e.g., department, team).
  /// ID format: "org:<org_id>" or "org:<org_id>/<unit>"
  /// All members of the org/unit get access.
  static const PrincipalType PRINCIPAL_TYPE_ORGANIZATION =
      PrincipalType._(3, _omitEnumNames ? '' : 'PRINCIPAL_TYPE_ORGANIZATION');

  /// Chat group (e.g., Matrix room, Slack channel).
  /// ID format: "room:<room_id>" or "chat:<chat_id>"
  /// All members of the chat group get access.
  static const PrincipalType PRINCIPAL_TYPE_CHAT_GROUP =
      PrincipalType._(4, _omitEnumNames ? '' : 'PRINCIPAL_TYPE_CHAT_GROUP');

  static const $core.List<PrincipalType> values = <PrincipalType>[
    PRINCIPAL_TYPE_UNSPECIFIED,
    PRINCIPAL_TYPE_USER,
    PRINCIPAL_TYPE_SERVICE,
    PRINCIPAL_TYPE_ORGANIZATION,
    PRINCIPAL_TYPE_CHAT_GROUP,
  ];

  static final $core.List<PrincipalType?> _byValue =
      $pb.ProtobufEnum.$_initByValueList(values, 4);
  static PrincipalType? valueOf($core.int value) =>
      value < 0 || value >= _byValue.length ? null : _byValue[value];

  const PrincipalType._(super.value, super.name);
}

class MediaMetadata_Visibility extends $pb.ProtobufEnum {
  /// Visibility not specified - defaults to PRIVATE.
  static const MediaMetadata_Visibility VISIBILITY_UNSPECIFIED =
      MediaMetadata_Visibility._(
          0, _omitEnumNames ? '' : 'VISIBILITY_UNSPECIFIED');

  /// Public content: accessible to any authenticated user.
  /// No grants required for read access.
  /// Use for: public assets, marketing materials, shared resources.
  static const MediaMetadata_Visibility VISIBILITY_PUBLIC =
      MediaMetadata_Visibility._(1, _omitEnumNames ? '' : 'VISIBILITY_PUBLIC');

  /// Private content: only accessible to owner and principals with explicit grants.
  /// Default for all uploads. Use for sensitive content.
  static const MediaMetadata_Visibility VISIBILITY_PRIVATE =
      MediaMetadata_Visibility._(2, _omitEnumNames ? '' : 'VISIBILITY_PRIVATE');

  static const $core.List<MediaMetadata_Visibility> values =
      <MediaMetadata_Visibility>[
    VISIBILITY_UNSPECIFIED,
    VISIBILITY_PUBLIC,
    VISIBILITY_PRIVATE,
  ];

  static final $core.List<MediaMetadata_Visibility?> _byValue =
      $pb.ProtobufEnum.$_initByValueList(values, 2);
  static MediaMetadata_Visibility? valueOf($core.int value) =>
      value < 0 || value >= _byValue.length ? null : _byValue[value];

  const MediaMetadata_Visibility._(super.value, super.name);
}

class SearchMediaRequest_SortBy extends $pb.ProtobufEnum {
  static const SearchMediaRequest_SortBy SORT_BY_UNSPECIFIED =
      SearchMediaRequest_SortBy._(
          0, _omitEnumNames ? '' : 'SORT_BY_UNSPECIFIED');
  static const SearchMediaRequest_SortBy SORT_BY_CREATED_AT =
      SearchMediaRequest_SortBy._(
          1, _omitEnumNames ? '' : 'SORT_BY_CREATED_AT');
  static const SearchMediaRequest_SortBy SORT_BY_UPDATED_AT =
      SearchMediaRequest_SortBy._(
          2, _omitEnumNames ? '' : 'SORT_BY_UPDATED_AT');
  static const SearchMediaRequest_SortBy SORT_BY_FILENAME =
      SearchMediaRequest_SortBy._(3, _omitEnumNames ? '' : 'SORT_BY_FILENAME');
  static const SearchMediaRequest_SortBy SORT_BY_FILE_SIZE =
      SearchMediaRequest_SortBy._(4, _omitEnumNames ? '' : 'SORT_BY_FILE_SIZE');

  static const $core.List<SearchMediaRequest_SortBy> values =
      <SearchMediaRequest_SortBy>[
    SORT_BY_UNSPECIFIED,
    SORT_BY_CREATED_AT,
    SORT_BY_UPDATED_AT,
    SORT_BY_FILENAME,
    SORT_BY_FILE_SIZE,
  ];

  static final $core.List<SearchMediaRequest_SortBy?> _byValue =
      $pb.ProtobufEnum.$_initByValueList(values, 4);
  static SearchMediaRequest_SortBy? valueOf($core.int value) =>
      value < 0 || value >= _byValue.length ? null : _byValue[value];

  const SearchMediaRequest_SortBy._(super.value, super.name);
}

/// Retention mode.
/// DELETE: remove after retention period
/// ARCHIVE: move to cold storage after period
class RetentionPolicy_Mode extends $pb.ProtobufEnum {
  static const RetentionPolicy_Mode MODE_UNSPECIFIED =
      RetentionPolicy_Mode._(0, _omitEnumNames ? '' : 'MODE_UNSPECIFIED');
  static const RetentionPolicy_Mode MODE_DELETE =
      RetentionPolicy_Mode._(1, _omitEnumNames ? '' : 'MODE_DELETE');
  static const RetentionPolicy_Mode MODE_ARCHIVE =
      RetentionPolicy_Mode._(2, _omitEnumNames ? '' : 'MODE_ARCHIVE');

  static const $core.List<RetentionPolicy_Mode> values = <RetentionPolicy_Mode>[
    MODE_UNSPECIFIED,
    MODE_DELETE,
    MODE_ARCHIVE,
  ];

  static final $core.List<RetentionPolicy_Mode?> _byValue =
      $pb.ProtobufEnum.$_initByValueList(values, 2);
  static RetentionPolicy_Mode? valueOf($core.int value) =>
      value < 0 || value >= _byValue.length ? null : _byValue[value];

  const RetentionPolicy_Mode._(super.value, super.name);
}

const $core.bool _omitEnumNames =
    $core.bool.fromEnvironment('protobuf.omit_enum_names');
