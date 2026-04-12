import 'package:antinvestor_api_files/antinvestor_api_files.dart';
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
  final response = await client.searchMedia(request);
  return response.results;
});

/// Get content metadata by ID.
final getContentProvider =
    FutureProvider.family<MediaMetadata, String>((ref, contentId) async {
  final client = ref.watch(filesServiceClientProvider);
  final request = GetContentRequest()..mediaId = contentId;
  final response = await client.getContent(request);
  return response.metadata;
});

/// Head content (metadata without body) by ID.
final headContentProvider =
    FutureProvider.family<MediaMetadata, String>((ref, contentId) async {
  final client = ref.watch(filesServiceClientProvider);
  final request = HeadContentRequest()..mediaId = contentId;
  final response = await client.headContent(request);
  return response.metadata;
});

/// Get storage stats.
final getStorageStatsProvider =
    FutureProvider<GetStorageStatsResponse>((ref) async {
  final client = ref.watch(filesServiceClientProvider);
  final request = GetStorageStatsRequest();
  return client.getStorageStats(request);
});

/// Get user usage stats.
final getUserUsageProvider =
    FutureProvider.family<GetUserUsageResponse, String>((ref, userId) async {
  final client = ref.watch(filesServiceClientProvider);
  final request = GetUserUsageRequest()..userId = userId;
  return client.getUserUsage(request);
});

/// Notifier for content mutations (delete, patch).
class ContentNotifier extends Notifier<AsyncValue<void>> {
  @override
  AsyncValue<void> build() => const AsyncValue.data(null);

  FilesServiceClient get _client => ref.read(filesServiceClientProvider);

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
    Map<String, String>? labels,
  }) async {
    state = const AsyncValue.loading();
    try {
      final request = PatchContentRequest()..mediaId = mediaId;
      if (filename != null) request.filename = filename;
      if (labels != null) request.setLabels.addAll(labels);
      await _client.patchContent(request);
      state = const AsyncValue.data(null);
    } catch (e, st) {
      state = AsyncValue.error(e, st);
      rethrow;
    }
  }
}

final contentNotifierProvider =
    NotifierProvider<ContentNotifier, AsyncValue<void>>(
        ContentNotifier.new);
