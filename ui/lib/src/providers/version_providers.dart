import 'package:antinvestor_api_files/antinvestor_api_files.dart';
import 'package:antinvestor_ui_core/api/stream_helpers.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import 'files_transport_provider.dart';

/// Get version history for a content item.
final getVersionsProvider =
    FutureProvider.family<List<FileVersion>, String>(
        (ref, contentId) async {
  final client = ref.watch(filesServiceClientProvider);
  final request = GetVersionsRequest()..mediaId = contentId;
  final stream = client.getVersions(request);
  return collectStream<GetVersionsResponse, FileVersion>(
    stream,
    extract: (r) => r.data,
  );
});

/// Notifier for version mutations (restore).
class VersionNotifier extends StateNotifier<AsyncValue<void>> {
  VersionNotifier(this._client) : super(const AsyncValue.data(null));
  final FilesServiceClient _client;

  /// Restore a specific version of the content.
  Future<void> restoreVersion({
    required String mediaId,
    required int versionNumber,
  }) async {
    state = const AsyncValue.loading();
    try {
      final request = RestoreVersionRequest()
        ..mediaId = mediaId
        ..versionNumber = versionNumber;
      await _client.restoreVersion(request);
      state = const AsyncValue.data(null);
    } catch (e, st) {
      state = AsyncValue.error(e, st);
      rethrow;
    }
  }
}

final versionNotifierProvider =
    StateNotifierProvider<VersionNotifier, AsyncValue<void>>((ref) {
  final client = ref.watch(filesServiceClientProvider);
  return VersionNotifier(client);
});
