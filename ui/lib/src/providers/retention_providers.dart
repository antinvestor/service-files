import 'package:antinvestor_api_files/antinvestor_api_files.dart';
import 'package:antinvestor_ui_core/api/stream_helpers.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import 'files_transport_provider.dart';

/// Get retention policy for a content item.
final getRetentionPolicyProvider =
    FutureProvider.family<RetentionPolicy, String>((ref, mediaId) async {
  final client = ref.watch(filesServiceClientProvider);
  final request = GetRetentionPolicyRequest()..mediaId = mediaId;
  final response = await client.getRetentionPolicy(request);
  return response.data;
});

/// List all retention policies.
final listRetentionPoliciesProvider =
    FutureProvider<List<RetentionPolicy>>((ref) async {
  final client = ref.watch(filesServiceClientProvider);
  final request = ListRetentionPoliciesRequest();
  final stream = client.listRetentionPolicies(request);
  return collectStream<ListRetentionPoliciesResponse, RetentionPolicy>(
    stream,
    extract: (r) => r.data,
  );
});

/// Notifier for retention policy mutations (set).
class RetentionNotifier extends StateNotifier<AsyncValue<void>> {
  RetentionNotifier(this._client) : super(const AsyncValue.data(null));
  final FilesServiceClient _client;

  /// Set a retention policy for a content item.
  Future<void> setRetentionPolicy({
    required String mediaId,
    required RetentionMode mode,
    required String duration,
  }) async {
    state = const AsyncValue.loading();
    try {
      final request = SetRetentionPolicyRequest()
        ..mediaId = mediaId
        ..mode = mode
        ..duration = duration;
      await _client.setRetentionPolicy(request);
      state = const AsyncValue.data(null);
    } catch (e, st) {
      state = AsyncValue.error(e, st);
      rethrow;
    }
  }
}

final retentionNotifierProvider =
    StateNotifierProvider<RetentionNotifier, AsyncValue<void>>((ref) {
  final client = ref.watch(filesServiceClientProvider);
  return RetentionNotifier(client);
});
