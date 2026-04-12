import 'package:antinvestor_api_files/antinvestor_api_files.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import 'files_transport_provider.dart';

/// Get retention policy for a content item.
final getRetentionPolicyProvider =
    FutureProvider.family<RetentionPolicy, String>((ref, mediaId) async {
  final client = ref.watch(filesServiceClientProvider);
  final request = GetRetentionPolicyRequest()..mediaId = mediaId;
  final response = await client.getRetentionPolicy(request);
  return response.policy;
});

/// List all retention policies.
final listRetentionPoliciesProvider =
    FutureProvider<List<RetentionPolicy>>((ref) async {
  final client = ref.watch(filesServiceClientProvider);
  final request = ListRetentionPoliciesRequest();
  final response = await client.listRetentionPolicies(request);
  return response.policies;
});

/// Notifier for retention policy mutations (set).
class RetentionNotifier extends Notifier<AsyncValue<void>> {
  @override
  AsyncValue<void> build() => const AsyncValue.data(null);

  FilesServiceClient get _client => ref.read(filesServiceClientProvider);

  /// Set a retention policy for a content item.
  Future<void> setRetentionPolicy({
    required String mediaId,
    required String policyId,
  }) async {
    state = const AsyncValue.loading();
    try {
      final request = SetRetentionPolicyRequest()
        ..mediaId = mediaId
        ..policyId = policyId;
      await _client.setRetentionPolicy(request);
      state = const AsyncValue.data(null);
    } catch (e, st) {
      state = AsyncValue.error(e, st);
      rethrow;
    }
  }
}

final retentionNotifierProvider =
    NotifierProvider<RetentionNotifier, AsyncValue<void>>(
        RetentionNotifier.new);
