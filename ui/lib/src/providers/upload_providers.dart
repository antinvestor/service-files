import 'package:antinvestor_api_files/antinvestor_api_files.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import 'files_transport_provider.dart';

/// State for tracking an upload in progress.
///
/// The upload flow uses CreateContent to pre-allocate a content URI,
/// then the caller uploads bytes via HTTP PUT to the returned URI.
class UploadState {
  const UploadState({
    this.filename = '',
    this.progress = 0.0,
    this.isUploading = false,
    this.error,
    this.mediaId,
    this.contentUri,
  });

  final String filename;
  final double progress;
  final bool isUploading;
  final String? error;
  final String? mediaId;

  /// The pre-allocated content URI for uploading bytes via HTTP PUT.
  final String? contentUri;

  UploadState copyWith({
    String? filename,
    double? progress,
    bool? isUploading,
    String? error,
    String? mediaId,
    String? contentUri,
  }) {
    return UploadState(
      filename: filename ?? this.filename,
      progress: progress ?? this.progress,
      isUploading: isUploading ?? this.isUploading,
      error: error,
      mediaId: mediaId ?? this.mediaId,
      contentUri: contentUri ?? this.contentUri,
    );
  }
}

/// Manages file upload lifecycle via the signed-URL pre-allocation flow.
///
/// 1. Calls [createContent] to pre-allocate a content URI (no bytes sent).
/// 2. Returns the content URI so the caller can upload bytes via HTTP PUT.
///
/// The actual byte upload happens outside of gRPC -- callers should
/// HTTP PUT the file data to [UploadState.contentUri].
class UploadNotifier extends StateNotifier<UploadState> {
  UploadNotifier(this._client) : super(const UploadState());
  final FilesServiceClient _client;

  bool _cancelled = false;

  /// Pre-allocate a content URI for the given file metadata.
  ///
  /// This does NOT upload file bytes. The caller must HTTP PUT the
  /// actual file data to the returned [UploadState.contentUri].
  Future<void> createContent({
    required String filename,
    required String contentType,
    Map<String, String>? labels,
  }) async {
    _cancelled = false;
    state = UploadState(filename: filename, isUploading: true);

    try {
      final request = CreateContentRequest()
        ..filename = filename
        ..contentType = contentType;
      if (labels != null) request.labels.addAll(labels);

      state = state.copyWith(progress: 0.3);

      if (_cancelled) {
        state = state.copyWith(isUploading: false, error: 'Upload cancelled');
        return;
      }

      final response = await _client.createContent(request);

      if (_cancelled) {
        state = state.copyWith(isUploading: false, error: 'Upload cancelled');
        return;
      }

      state = state.copyWith(
        progress: 1.0,
        isUploading: false,
        mediaId: response.mediaId,
        contentUri: response.contentUri,
      );
    } catch (e) {
      state = state.copyWith(
        isUploading: false,
        error: e.toString(),
      );
      rethrow;
    }
  }

  /// Cancel the current upload.
  void cancel() {
    _cancelled = true;
    state = state.copyWith(isUploading: false, error: 'Upload cancelled');
  }

  /// Reset upload state.
  void reset() {
    _cancelled = false;
    state = const UploadState();
  }
}

final uploadNotifierProvider =
    StateNotifierProvider<UploadNotifier, UploadState>((ref) {
  final client = ref.watch(filesServiceClientProvider);
  return UploadNotifier(client);
});
