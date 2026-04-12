import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../providers/files_providers.dart';
import 'file_thumbnail_widget.dart';

/// A compact chip that shows a file's name, size, and type icon.
///
/// Resolves content metadata by ID using [headContentProvider].
///
/// ```dart
/// FileAttachmentChip(contentId: attachment.mediaId)
/// FileAttachmentChip(contentId: id, onTap: () => openFile(id))
/// ```
class FileAttachmentChip extends ConsumerWidget {
  const FileAttachmentChip({
    super.key,
    required this.contentId,
    this.onTap,
    this.onRemove,
  });

  final String contentId;
  final VoidCallback? onTap;

  /// If provided, shows a close/remove button.
  final VoidCallback? onRemove;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    if (contentId.isEmpty) return const SizedBox.shrink();

    final theme = Theme.of(context);
    final metaAsync = ref.watch(headContentProvider(contentId));

    return metaAsync.when(
      data: (meta) {
        final filename = meta.filename.isNotEmpty ? meta.filename : 'file';
        final sizeLabel = _formatSize(meta.fileSizeBytes.toInt());
        final contentType = meta.contentType.toLowerCase();
        final icon = FileThumbnailWidget.iconForContentType(contentType);
        final color = FileThumbnailWidget.colorForContentType(contentType);

        return _buildChip(
          theme,
          icon: icon,
          color: color,
          label: filename,
          subtitle: sizeLabel,
        );
      },
      loading: () => _buildChip(
        theme,
        icon: Icons.hourglass_empty,
        color: Colors.grey,
        label: _truncateId(contentId),
        subtitle: null,
      ),
      error: (_, _) => _buildChip(
        theme,
        icon: Icons.error_outline,
        color: theme.colorScheme.error,
        label: _truncateId(contentId),
        subtitle: 'unavailable',
      ),
    );
  }

  Widget _buildChip(
    ThemeData theme, {
    required IconData icon,
    required Color color,
    required String label,
    String? subtitle,
  }) {
    return InkWell(
      onTap: onTap,
      borderRadius: BorderRadius.circular(8),
      child: Container(
        padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 6),
        decoration: BoxDecoration(
          color: color.withAlpha(15),
          borderRadius: BorderRadius.circular(8),
          border: Border.all(color: color.withAlpha(40)),
        ),
        child: Row(
          mainAxisSize: MainAxisSize.min,
          children: [
            Icon(icon, size: 16, color: color),
            const SizedBox(width: 8),
            Flexible(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                mainAxisSize: MainAxisSize.min,
                children: [
                  Text(
                    label,
                    style: theme.textTheme.labelSmall?.copyWith(
                      fontWeight: FontWeight.w600,
                      color: theme.colorScheme.onSurface,
                    ),
                    maxLines: 1,
                    overflow: TextOverflow.ellipsis,
                  ),
                  if (subtitle != null && subtitle.isNotEmpty)
                    Text(
                      subtitle,
                      style: theme.textTheme.labelSmall?.copyWith(
                        fontSize: 10,
                        color: theme.colorScheme.onSurfaceVariant,
                      ),
                    ),
                ],
              ),
            ),
            if (onRemove != null) ...[
              const SizedBox(width: 4),
              GestureDetector(
                onTap: onRemove,
                child: Icon(
                  Icons.close,
                  size: 14,
                  color: theme.colorScheme.onSurfaceVariant,
                ),
              ),
            ],
          ],
        ),
      ),
    );
  }

  static String _truncateId(String id) =>
      id.length > 12 ? '${id.substring(0, 12)}...' : id;

  static String _formatSize(int bytes) {
    if (bytes <= 0) return '';
    if (bytes < 1024) return '$bytes B';
    if (bytes < 1024 * 1024) return '${(bytes / 1024).toStringAsFixed(1)} KB';
    if (bytes < 1024 * 1024 * 1024) {
      return '${(bytes / (1024 * 1024)).toStringAsFixed(1)} MB';
    }
    return '${(bytes / (1024 * 1024 * 1024)).toStringAsFixed(1)} GB';
  }
}
