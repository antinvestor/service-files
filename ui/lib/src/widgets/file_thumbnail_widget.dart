import 'package:antinvestor_api_files/antinvestor_api_files.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../providers/files_providers.dart';

/// Resolves content metadata by ID and shows an icon-based preview thumbnail.
///
/// Uses [headContentProvider] to fetch metadata without downloading the body.
///
/// ```dart
/// FileThumbnailWidget(contentId: attachment.mediaId)
/// FileThumbnailWidget(contentId: id, size: 64)
/// ```
class FileThumbnailWidget extends ConsumerWidget {
  const FileThumbnailWidget({
    super.key,
    required this.contentId,
    this.size = 48,
    this.borderRadius = 8,
    this.onTap,
  });

  final String contentId;
  final double size;
  final double borderRadius;
  final VoidCallback? onTap;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    if (contentId.isEmpty) return SizedBox(width: size, height: size);

    final theme = Theme.of(context);
    final metaAsync = ref.watch(headContentProvider(contentId));

    return metaAsync.when(
      data: (meta) => _buildThumbnail(theme, meta),
      loading: () => _buildPlaceholder(theme, isLoading: true),
      error: (_, __) => _buildPlaceholder(theme, isError: true),
    );
  }

  Widget _buildThumbnail(ThemeData theme, MediaMetadata meta) {
    final contentType = meta.contentType.toLowerCase();
    final icon = iconForContentType(contentType);
    final color = colorForContentType(contentType);

    return GestureDetector(
      onTap: onTap,
      child: Container(
        width: size,
        height: size,
        decoration: BoxDecoration(
          color: color.withAlpha(25),
          borderRadius: BorderRadius.circular(borderRadius),
          border: Border.all(color: color.withAlpha(50)),
        ),
        child: Column(
          mainAxisAlignment: MainAxisAlignment.center,
          children: [
            Icon(icon, size: size * 0.45, color: color),
            if (size >= 48) ...[
              const SizedBox(height: 2),
              Text(
                _extensionLabel(meta),
                style: theme.textTheme.labelSmall?.copyWith(
                  color: color,
                  fontSize: 8,
                  fontWeight: FontWeight.w600,
                ),
                maxLines: 1,
                overflow: TextOverflow.ellipsis,
              ),
            ],
          ],
        ),
      ),
    );
  }

  Widget _buildPlaceholder(ThemeData theme,
      {bool isLoading = false, bool isError = false}) {
    return Container(
      width: size,
      height: size,
      decoration: BoxDecoration(
        color: theme.colorScheme.surfaceContainerHighest,
        borderRadius: BorderRadius.circular(borderRadius),
      ),
      child: Center(
        child: isLoading
            ? SizedBox(
                width: size * 0.4,
                height: size * 0.4,
                child: const CircularProgressIndicator(strokeWidth: 2),
              )
            : Icon(
                isError ? Icons.broken_image_outlined : Icons.insert_drive_file,
                size: size * 0.45,
                color: theme.colorScheme.onSurfaceVariant,
              ),
      ),
    );
  }

  String _extensionLabel(MediaMetadata meta) {
    final filename = meta.filename;
    if (filename.contains('.')) {
      return filename.split('.').last.toUpperCase();
    }
    final ct = meta.contentType;
    if (ct.contains('/')) {
      return ct.split('/').last.toUpperCase();
    }
    return 'FILE';
  }

  static IconData iconForContentType(String contentType) {
    if (contentType.startsWith('image/')) return Icons.image_outlined;
    if (contentType.startsWith('video/')) return Icons.videocam_outlined;
    if (contentType.startsWith('audio/')) return Icons.audiotrack_outlined;
    if (contentType.contains('pdf')) return Icons.picture_as_pdf_outlined;
    if (contentType.contains('spreadsheet') || contentType.contains('excel')) {
      return Icons.table_chart_outlined;
    }
    if (contentType.contains('document') || contentType.contains('word')) {
      return Icons.description_outlined;
    }
    if (contentType.contains('presentation') ||
        contentType.contains('powerpoint')) {
      return Icons.slideshow_outlined;
    }
    if (contentType.contains('zip') ||
        contentType.contains('tar') ||
        contentType.contains('compressed')) {
      return Icons.folder_zip_outlined;
    }
    if (contentType.contains('text/')) return Icons.article_outlined;
    return Icons.insert_drive_file_outlined;
  }

  static Color colorForContentType(String contentType) {
    if (contentType.startsWith('image/')) return Colors.blue;
    if (contentType.startsWith('video/')) return Colors.purple;
    if (contentType.startsWith('audio/')) return Colors.orange;
    if (contentType.contains('pdf')) return Colors.red;
    if (contentType.contains('spreadsheet') || contentType.contains('excel')) {
      return Colors.green;
    }
    if (contentType.contains('document') || contentType.contains('word')) {
      return Colors.indigo;
    }
    return Colors.grey;
  }
}
