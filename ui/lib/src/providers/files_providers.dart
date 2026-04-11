import 'package:antinvestor_api_files/antinvestor_api_files.dart';
import 'package:antinvestor_ui_core/api/stream_helpers.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import 'files_transport_provider.dart';

/// Parameters for searching media content.
class MediaSearchParams {
  const MediaSearchParams({
    this.query = '',
    this.state,
  });

  final String query;
  final MediaState? state;

  @override
  bool operator ==(Object other) =>
      identical(this, other) ||
      other is MediaSearchParams &&
          query == other.query &&
          state == other.state;

  @override
  int get hashCode => Object.hash(query, state);
}

/// Search media with optional filters.
final searchMediaProvider =
    FutureProvider.family<List<MediaMetadata>, MediaSearchParams>(
        (ref, params) async {
  final client = ref.watch(filesServiceClientProvider);
  final request = SearchMediaRequest()..query = params.query;
  if (params.state != null) {
    request.state = params.state!;
  }
  final stream = client.searchMedia(request);
  return collectStream<SearchMediaResponse, MediaMetadata>(
    stream,
    extract: (r) => r.data,
  );
});

/// Get content metadata by ID.
final getContentProvider =
    FutureProvider.family<MediaMetadata, String>((ref, contentId) async {
  final client = ref.watch(filesServiceClientProvider);
  final request = GetContentRequest()..mediaId = contentId;
  final response = await client.getContent(request);
  return response.data;
});

/// Head content (metadata without body) by ID.
final headContentProvider =
    FutureProvider.family<MediaMetadata, String>((ref, contentId) async {
  final client = ref.watch(filesServiceClientProvider);
  final request = HeadContentRequest()..mediaId = contentId;
  final response = await client.headContent(request);
  return response.data;
});

/// Get storage stats.
final getStorageStatsProvider =
    FutureProvider<StorageStatsResponse>((ref) async {
  final client = ref.watch(filesServiceClientProvider);
  final request = StorageStatsRequest();
  return client.getStorageStats(request);
});

/// Get user usage stats.
final getUserUsageProvider =
    FutureProvider.family<UserUsageResponse, String>((ref, userId) async {
  final client = ref.watch(filesServiceClientProvider);
  final request = UserUsageRequest()..userId = userId;
  return client.getUserUsage(request);
});

/// Notifier for content mutations (delete, patch).
class ContentNotifier extends StateNotifier<AsyncValue<void>> {
  ContentNotifier(this._client) : super(const AsyncValue.data(null));
  final FilesServiceClient _client;

  Future<void> deleteContent(String mediaId) async {
    state = const AsyncValue.loading();
    try {
      final request = DeleteContentRequest()..mediaId = mediaId;
      await _client.deleteContent(request);
      state = const AsyncValue.data(null);
    } catch (e, st) {
      state = AsyncValue.error(e, st);
      rethrow;
    }
  }

  Future<void> patchContent({
    required String mediaId,
    String? filename,
    String? contentType,
    Map<String, String>? labels,
  }) async {
    state = const AsyncValue.loading();
    try {
      final request = PatchContentRequest()..mediaId = mediaId;
      if (filename != null) request.filename = filename;
      if (contentType != null) request.contentType = contentType;
      if (labels != null) request.labels.addAll(labels);
      await _client.patchContent(request);
      state = const AsyncValue.data(null);
    } catch (e, st) {
      state = AsyncValue.error(e, st);
      rethrow;
    }
  }
}

final contentNotifierProvider =
    StateNotifierProvider<ContentNotifier, AsyncValue<void>>((ref) {
  final client = ref.watch(filesServiceClientProvider);
  return ContentNotifier(client);
});
