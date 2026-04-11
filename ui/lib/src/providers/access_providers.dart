import 'package:antinvestor_api_files/antinvestor_api_files.dart';
import 'package:antinvestor_ui_core/api/stream_helpers.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import 'files_transport_provider.dart';

/// List access grants for a content item.
final listAccessProvider =
    FutureProvider.family<List<AccessGrant>, String>((ref, contentId) async {
  final client = ref.watch(filesServiceClientProvider);
  final request = ListAccessRequest()..mediaId = contentId;
  final stream = client.listAccess(request);
  return collectStream<ListAccessResponse, AccessGrant>(
    stream,
    extract: (r) => r.data,
  );
});

/// Notifier for access mutations (grant, revoke).
class AccessNotifier extends StateNotifier<AsyncValue<void>> {
  AccessNotifier(this._client) : super(const AsyncValue.data(null));
  final FilesServiceClient _client;

  /// Grant access to a principal.
  Future<void> grantAccess({
    required String mediaId,
    required String principalId,
    required PrincipalType principalType,
    required AccessRole role,
  }) async {
    state = const AsyncValue.loading();
    try {
      final request = GrantAccessRequest()
        ..mediaId = mediaId
        ..grant = (AccessGrant()
          ..principalId = principalId
          ..principalType = principalType
          ..role = role);
      await _client.grantAccess(request);
      state = const AsyncValue.data(null);
    } catch (e, st) {
      state = AsyncValue.error(e, st);
      rethrow;
    }
  }

  /// Revoke access from a principal.
  Future<void> revokeAccess({
    required String mediaId,
    required String principalId,
  }) async {
    state = const AsyncValue.loading();
    try {
      final request = RevokeAccessRequest()
        ..mediaId = mediaId
        ..principalId = principalId;
      await _client.revokeAccess(request);
      state = const AsyncValue.data(null);
    } catch (e, st) {
      state = AsyncValue.error(e, st);
      rethrow;
    }
  }
}

final accessNotifierProvider =
    StateNotifierProvider<AccessNotifier, AsyncValue<void>>((ref) {
  final client = ref.watch(filesServiceClientProvider);
  return AccessNotifier(client);
});
