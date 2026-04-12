import 'package:antinvestor_api_files/antinvestor_api_files.dart';
import 'package:fixnum/fixnum.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import 'files_transport_provider.dart';

/// Get version history for a content item.
final getVersionsProvider =
    FutureProvider.family<List<FileVersion>, String>(
        (ref, contentId) async {
  final client = ref.watch(filesServiceClientProvider);
  final request = GetVersionsRequest()..mediaId = contentId;
  final response = await client.getVersions(request);
  return response.versions;
});

/// Notifier for version mutations (restore).
class VersionNotifier extends Notifier<AsyncValue<void>> {
  @override
  AsyncValue<void> build() => const AsyncValue.data(null);

  FilesServiceClient get _client => ref.read(filesServiceClientProvider);

  /// Restore a specific version of the content.
  Future<void> restoreVersion({
    required String mediaId,
    required int versionNumber,
  }) async {
    state = const AsyncValue.loading();
    try {
      final request = RestoreVersionRequest()
        ..mediaId = mediaId
        ..version = Int64(versionNumber);
      await _client.restoreVersion(request);
      state = const AsyncValue.data(null);
    } catch (e, st) {
      state = AsyncValue.error(e, st);
      rethrow;
    }
  }
}

final versionNotifierProvider =
    NotifierProvider<VersionNotifier, AsyncValue<void>>(
        VersionNotifier.new);
