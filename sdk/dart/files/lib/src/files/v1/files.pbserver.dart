//
//  Generated code. Do not modify.
//  source: files/v1/files.proto
//
// @dart = 2.12

// ignore_for_file: annotate_overrides, camel_case_types, comment_references
// ignore_for_file: constant_identifier_names
// ignore_for_file: deprecated_member_use_from_same_package, library_prefixes
// ignore_for_file: non_constant_identifier_names, prefer_final_fields
// ignore_for_file: unnecessary_import, unnecessary_this, unused_import

import 'dart:async' as $async;
import 'dart:core' as $core;

import 'package:protobuf/protobuf.dart' as $pb;

import 'files.pb.dart' as $8;
import 'files.pbjson.dart';

export 'files.pb.dart';

abstract class FilesServiceBase extends $pb.GeneratedService {
  $async.Future<$8.UploadContentResponse> uploadContent($pb.ServerContext ctx, $8.UploadContentRequest request);
  $async.Future<$8.CreateContentResponse> createContent($pb.ServerContext ctx, $8.CreateContentRequest request);
  $async.Future<$8.CreateMultipartUploadResponse> createMultipartUpload($pb.ServerContext ctx, $8.CreateMultipartUploadRequest request);
  $async.Future<$8.GetMultipartUploadResponse> getMultipartUpload($pb.ServerContext ctx, $8.GetMultipartUploadRequest request);
  $async.Future<$8.UploadMultipartPartResponse> uploadMultipartPart($pb.ServerContext ctx, $8.UploadMultipartPartRequest request);
  $async.Future<$8.CompleteMultipartUploadResponse> completeMultipartUpload($pb.ServerContext ctx, $8.CompleteMultipartUploadRequest request);
  $async.Future<$8.AbortMultipartUploadResponse> abortMultipartUpload($pb.ServerContext ctx, $8.AbortMultipartUploadRequest request);
  $async.Future<$8.ListMultipartPartsResponse> listMultipartParts($pb.ServerContext ctx, $8.ListMultipartPartsRequest request);
  $async.Future<$8.HeadContentResponse> headContent($pb.ServerContext ctx, $8.HeadContentRequest request);
  $async.Future<$8.PatchContentResponse> patchContent($pb.ServerContext ctx, $8.PatchContentRequest request);
  $async.Future<$8.GetSignedUploadUrlResponse> getSignedUploadUrl($pb.ServerContext ctx, $8.GetSignedUploadUrlRequest request);
  $async.Future<$8.FinalizeSignedUploadResponse> finalizeSignedUpload($pb.ServerContext ctx, $8.FinalizeSignedUploadRequest request);
  $async.Future<$8.GetSignedDownloadUrlResponse> getSignedDownloadUrl($pb.ServerContext ctx, $8.GetSignedDownloadUrlRequest request);
  $async.Future<$8.DeleteContentResponse> deleteContent($pb.ServerContext ctx, $8.DeleteContentRequest request);
  $async.Future<$8.GetContentResponse> getContent($pb.ServerContext ctx, $8.GetContentRequest request);
  $async.Future<$8.GetContentOverrideNameResponse> getContentOverrideName($pb.ServerContext ctx, $8.GetContentOverrideNameRequest request);
  $async.Future<$8.DownloadContentResponse> downloadContent($pb.ServerContext ctx, $8.DownloadContentRequest request);
  $async.Future<$8.DownloadContentRangeResponse> downloadContentRange($pb.ServerContext ctx, $8.DownloadContentRangeRequest request);
  $async.Future<$8.GetContentThumbnailResponse> getContentThumbnail($pb.ServerContext ctx, $8.GetContentThumbnailRequest request);
  $async.Future<$8.GetUrlPreviewResponse> getUrlPreview($pb.ServerContext ctx, $8.GetUrlPreviewRequest request);
  $async.Future<$8.GetConfigResponse> getConfig($pb.ServerContext ctx, $8.GetConfigRequest request);
  $async.Future<$8.SearchMediaResponse> searchMedia($pb.ServerContext ctx, $8.SearchMediaRequest request);
  $async.Future<$8.BatchGetContentResponse> batchGetContent($pb.ServerContext ctx, $8.BatchGetContentRequest request);
  $async.Future<$8.BatchDeleteContentResponse> batchDeleteContent($pb.ServerContext ctx, $8.BatchDeleteContentRequest request);
  $async.Future<$8.GrantAccessResponse> grantAccess($pb.ServerContext ctx, $8.GrantAccessRequest request);
  $async.Future<$8.RevokeAccessResponse> revokeAccess($pb.ServerContext ctx, $8.RevokeAccessRequest request);
  $async.Future<$8.ListAccessResponse> listAccess($pb.ServerContext ctx, $8.ListAccessRequest request);
  $async.Future<$8.GetVersionsResponse> getVersions($pb.ServerContext ctx, $8.GetVersionsRequest request);
  $async.Future<$8.RestoreVersionResponse> restoreVersion($pb.ServerContext ctx, $8.RestoreVersionRequest request);
  $async.Future<$8.SetRetentionPolicyResponse> setRetentionPolicy($pb.ServerContext ctx, $8.SetRetentionPolicyRequest request);
  $async.Future<$8.GetRetentionPolicyResponse> getRetentionPolicy($pb.ServerContext ctx, $8.GetRetentionPolicyRequest request);
  $async.Future<$8.ListRetentionPoliciesResponse> listRetentionPolicies($pb.ServerContext ctx, $8.ListRetentionPoliciesRequest request);
  $async.Future<$8.GetUserUsageResponse> getUserUsage($pb.ServerContext ctx, $8.GetUserUsageRequest request);
  $async.Future<$8.GetStorageStatsResponse> getStorageStats($pb.ServerContext ctx, $8.GetStorageStatsRequest request);

  $pb.GeneratedMessage createRequest($core.String methodName) {
    switch (methodName) {
      case 'UploadContent': return $8.UploadContentRequest();
      case 'CreateContent': return $8.CreateContentRequest();
      case 'CreateMultipartUpload': return $8.CreateMultipartUploadRequest();
      case 'GetMultipartUpload': return $8.GetMultipartUploadRequest();
      case 'UploadMultipartPart': return $8.UploadMultipartPartRequest();
      case 'CompleteMultipartUpload': return $8.CompleteMultipartUploadRequest();
      case 'AbortMultipartUpload': return $8.AbortMultipartUploadRequest();
      case 'ListMultipartParts': return $8.ListMultipartPartsRequest();
      case 'HeadContent': return $8.HeadContentRequest();
      case 'PatchContent': return $8.PatchContentRequest();
      case 'GetSignedUploadUrl': return $8.GetSignedUploadUrlRequest();
      case 'FinalizeSignedUpload': return $8.FinalizeSignedUploadRequest();
      case 'GetSignedDownloadUrl': return $8.GetSignedDownloadUrlRequest();
      case 'DeleteContent': return $8.DeleteContentRequest();
      case 'GetContent': return $8.GetContentRequest();
      case 'GetContentOverrideName': return $8.GetContentOverrideNameRequest();
      case 'DownloadContent': return $8.DownloadContentRequest();
      case 'DownloadContentRange': return $8.DownloadContentRangeRequest();
      case 'GetContentThumbnail': return $8.GetContentThumbnailRequest();
      case 'GetUrlPreview': return $8.GetUrlPreviewRequest();
      case 'GetConfig': return $8.GetConfigRequest();
      case 'SearchMedia': return $8.SearchMediaRequest();
      case 'BatchGetContent': return $8.BatchGetContentRequest();
      case 'BatchDeleteContent': return $8.BatchDeleteContentRequest();
      case 'GrantAccess': return $8.GrantAccessRequest();
      case 'RevokeAccess': return $8.RevokeAccessRequest();
      case 'ListAccess': return $8.ListAccessRequest();
      case 'GetVersions': return $8.GetVersionsRequest();
      case 'RestoreVersion': return $8.RestoreVersionRequest();
      case 'SetRetentionPolicy': return $8.SetRetentionPolicyRequest();
      case 'GetRetentionPolicy': return $8.GetRetentionPolicyRequest();
      case 'ListRetentionPolicies': return $8.ListRetentionPoliciesRequest();
      case 'GetUserUsage': return $8.GetUserUsageRequest();
      case 'GetStorageStats': return $8.GetStorageStatsRequest();
      default: throw $core.ArgumentError('Unknown method: $methodName');
    }
  }

  $async.Future<$pb.GeneratedMessage> handleCall($pb.ServerContext ctx, $core.String methodName, $pb.GeneratedMessage request) {
    switch (methodName) {
      case 'UploadContent': return this.uploadContent(ctx, request as $8.UploadContentRequest);
      case 'CreateContent': return this.createContent(ctx, request as $8.CreateContentRequest);
      case 'CreateMultipartUpload': return this.createMultipartUpload(ctx, request as $8.CreateMultipartUploadRequest);
      case 'GetMultipartUpload': return this.getMultipartUpload(ctx, request as $8.GetMultipartUploadRequest);
      case 'UploadMultipartPart': return this.uploadMultipartPart(ctx, request as $8.UploadMultipartPartRequest);
      case 'CompleteMultipartUpload': return this.completeMultipartUpload(ctx, request as $8.CompleteMultipartUploadRequest);
      case 'AbortMultipartUpload': return this.abortMultipartUpload(ctx, request as $8.AbortMultipartUploadRequest);
      case 'ListMultipartParts': return this.listMultipartParts(ctx, request as $8.ListMultipartPartsRequest);
      case 'HeadContent': return this.headContent(ctx, request as $8.HeadContentRequest);
      case 'PatchContent': return this.patchContent(ctx, request as $8.PatchContentRequest);
      case 'GetSignedUploadUrl': return this.getSignedUploadUrl(ctx, request as $8.GetSignedUploadUrlRequest);
      case 'FinalizeSignedUpload': return this.finalizeSignedUpload(ctx, request as $8.FinalizeSignedUploadRequest);
      case 'GetSignedDownloadUrl': return this.getSignedDownloadUrl(ctx, request as $8.GetSignedDownloadUrlRequest);
      case 'DeleteContent': return this.deleteContent(ctx, request as $8.DeleteContentRequest);
      case 'GetContent': return this.getContent(ctx, request as $8.GetContentRequest);
      case 'GetContentOverrideName': return this.getContentOverrideName(ctx, request as $8.GetContentOverrideNameRequest);
      case 'DownloadContent': return this.downloadContent(ctx, request as $8.DownloadContentRequest);
      case 'DownloadContentRange': return this.downloadContentRange(ctx, request as $8.DownloadContentRangeRequest);
      case 'GetContentThumbnail': return this.getContentThumbnail(ctx, request as $8.GetContentThumbnailRequest);
      case 'GetUrlPreview': return this.getUrlPreview(ctx, request as $8.GetUrlPreviewRequest);
      case 'GetConfig': return this.getConfig(ctx, request as $8.GetConfigRequest);
      case 'SearchMedia': return this.searchMedia(ctx, request as $8.SearchMediaRequest);
      case 'BatchGetContent': return this.batchGetContent(ctx, request as $8.BatchGetContentRequest);
      case 'BatchDeleteContent': return this.batchDeleteContent(ctx, request as $8.BatchDeleteContentRequest);
      case 'GrantAccess': return this.grantAccess(ctx, request as $8.GrantAccessRequest);
      case 'RevokeAccess': return this.revokeAccess(ctx, request as $8.RevokeAccessRequest);
      case 'ListAccess': return this.listAccess(ctx, request as $8.ListAccessRequest);
      case 'GetVersions': return this.getVersions(ctx, request as $8.GetVersionsRequest);
      case 'RestoreVersion': return this.restoreVersion(ctx, request as $8.RestoreVersionRequest);
      case 'SetRetentionPolicy': return this.setRetentionPolicy(ctx, request as $8.SetRetentionPolicyRequest);
      case 'GetRetentionPolicy': return this.getRetentionPolicy(ctx, request as $8.GetRetentionPolicyRequest);
      case 'ListRetentionPolicies': return this.listRetentionPolicies(ctx, request as $8.ListRetentionPoliciesRequest);
      case 'GetUserUsage': return this.getUserUsage(ctx, request as $8.GetUserUsageRequest);
      case 'GetStorageStats': return this.getStorageStats(ctx, request as $8.GetStorageStatsRequest);
      default: throw $core.ArgumentError('Unknown method: $methodName');
    }
  }

  $core.Map<$core.String, $core.dynamic> get $json => FilesServiceBase$json;
  $core.Map<$core.String, $core.Map<$core.String, $core.dynamic>> get $messageJson => FilesServiceBase$messageJson;
}

