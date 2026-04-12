import 'package:antinvestor_ui_core/widgets/error_helpers.dart';
import 'package:antinvestor_ui_core/widgets/form_field_card.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';

import '../providers/upload_providers.dart';
import '../widgets/upload_progress_indicator.dart';

/// Screen for uploading a new file with filename, content type, and progress tracking.
class FileUploadScreen extends ConsumerStatefulWidget {
  const FileUploadScreen({super.key});

  @override
  ConsumerState<FileUploadScreen> createState() => _FileUploadScreenState();
}

class _FileUploadScreenState extends ConsumerState<FileUploadScreen> {
  final _formKey = GlobalKey<FormState>();
  final _filenameController = TextEditingController();
  final _contentTypeController = TextEditingController(text: 'application/octet-stream');

  final bool _fileSelected = false;
  String? _selectedFileName;

  @override
  void dispose() {
    _filenameController.dispose();
    _contentTypeController.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final uploadState = ref.watch(uploadNotifierProvider);

    // Navigate to detail on successful content creation
    ref.listen<UploadState>(uploadNotifierProvider, (prev, next) {
      if (next.contentUri != null && !next.isUploading && next.error == null) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(
            content: Text(
              'Content URI allocated. Upload bytes via HTTP PUT to: ${next.contentUri}',
            ),
          ),
        );
        context.go('/files/${next.mediaId}');
        ref.read(uploadNotifierProvider.notifier).reset();
      }
    });

    return SingleChildScrollView(
      padding: const EdgeInsets.all(24),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          // Header
          Row(
            children: [
              Icon(Icons.upload_file, size: 28, color: theme.colorScheme.primary),
              const SizedBox(width: 12),
              Text(
                'Upload File',
                style: theme.textTheme.headlineSmall?.copyWith(
                  fontWeight: FontWeight.w600,
                  letterSpacing: -0.3,
                ),
              ),
            ],
          ),
          const SizedBox(height: 24),

          // Upload progress (shown during upload)
          if (uploadState.isUploading || uploadState.error != null)
            Padding(
              padding: const EdgeInsets.only(bottom: 24),
              child: UploadProgressIndicator(
                filename: uploadState.filename,
                progress: uploadState.progress,
                error: uploadState.error,
                onCancel: uploadState.isUploading
                    ? () => ref.read(uploadNotifierProvider.notifier).cancel()
                    : null,
              ),
            ),

          // Upload form
          Form(
            key: _formKey,
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.stretch,
              children: [
                // File selector
                FormFieldCard(
                  label: 'File',
                  isRequired: true,
                  description: 'Select a file to upload.',
                  child: InkWell(
                    onTap: uploadState.isUploading ? null : _selectFile,
                    borderRadius: BorderRadius.circular(12),
                    child: Container(
                      padding: const EdgeInsets.all(24),
                      decoration: BoxDecoration(
                        border: Border.all(
                          color: theme.colorScheme.outlineVariant,
                          width: 2,
                          strokeAlign: BorderSide.strokeAlignInside,
                        ),
                        borderRadius: BorderRadius.circular(12),
                      ),
                      child: Column(
                        children: [
                          Icon(
                            _fileSelected
                                ? Icons.check_circle
                                : Icons.cloud_upload_outlined,
                            size: 40,
                            color: _fileSelected
                                ? Colors.green
                                : theme.colorScheme.onSurfaceVariant,
                          ),
                          const SizedBox(height: 8),
                          Text(
                            _selectedFileName ?? 'Tap to select a file',
                            style: theme.textTheme.bodyMedium?.copyWith(
                              color: theme.colorScheme.onSurfaceVariant,
                            ),
                          ),
                        ],
                      ),
                    ),
                  ),
                ),

                FormFieldCard(
                  label: 'Filename',
                  isRequired: true,
                  description: 'Name for the uploaded file.',
                  child: TextFormField(
                    controller: _filenameController,
                    enabled: !uploadState.isUploading,
                    decoration: const InputDecoration(
                      hintText: 'e.g. report.pdf',
                    ),
                    validator: (value) {
                      if (value == null || value.trim().isEmpty) {
                        return 'Filename is required';
                      }
                      return null;
                    },
                  ),
                ),

                FormFieldCard(
                  label: 'Content Type',
                  isRequired: true,
                  description: 'MIME type of the file.',
                  child: TextFormField(
                    controller: _contentTypeController,
                    enabled: !uploadState.isUploading,
                    decoration: const InputDecoration(
                      hintText: 'e.g. application/pdf',
                    ),
                    validator: (value) {
                      if (value == null || value.trim().isEmpty) {
                        return 'Content type is required';
                      }
                      return null;
                    },
                  ),
                ),

                const SizedBox(height: 24),

                Row(
                  mainAxisAlignment: MainAxisAlignment.end,
                  children: [
                    TextButton(
                      onPressed:
                          uploadState.isUploading ? null : () => context.go('/files'),
                      child: const Text('Cancel'),
                    ),
                    const SizedBox(width: 12),
                    FilledButton.icon(
                      onPressed: uploadState.isUploading ? null : _upload,
                      icon: const Icon(Icons.upload, size: 18),
                      label: const Text('Upload'),
                    ),
                  ],
                ),
              ],
            ),
          ),
        ],
      ),
    );
  }

  void _selectFile() {
    // TODO: Integrate platform-specific file picking using the `file_picker`
    // or `image_picker` package. The selected file's bytes will be uploaded
    // via HTTP PUT to the content URI returned by createContent.
    ScaffoldMessenger.of(context).showSnackBar(
      const SnackBar(
        content: Text(
          'File picker integration needed. Add file_picker or image_picker package.',
        ),
      ),
    );
  }

  /// Pre-allocate a content URI. The actual byte upload happens via HTTP PUT
  /// to the returned content URI.
  Future<void> _upload() async {
    if (!_formKey.currentState!.validate()) return;

    try {
      await ref.read(uploadNotifierProvider.notifier).createContent(
            filename: _filenameController.text.trim(),
            contentType: _contentTypeController.text.trim(),
          );
    } catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: Text(friendlyError(e))),
        );
      }
    }
  }
}
