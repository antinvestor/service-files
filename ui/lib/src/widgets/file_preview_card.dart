import 'package:antinvestor_api_files/antinvestor_api_files.dart';
import 'package:flutter/material.dart';

import 'media_state_badge.dart';
import 'scan_status_badge.dart';

/// A card showing a file preview with icon, filename, size, and status badges.
class FilePreviewCard extends StatelessWidget {
  const FilePreviewCard({
    super.key,
    required this.media,
    this.onTap,
  });

  final MediaMetadata media;
  final VoidCallback? onTap;

  /// Returns an appropriate icon based on the content type.
  static IconData iconForContentType(String contentType) {
    final type = contentType.toLowerCase();
    if (type.startsWith('image/')) return Icons.photo;
    if (type == 'application/pdf') return Icons.picture_as_pdf;
    if (type.startsWith('video/')) return Icons.videocam;
    if (type.startsWith('audio/')) return Icons.audiotrack;
    return Icons.insert_drive_file;
  }

  /// Returns a color based on the content type.
  static Color colorForContentType(String contentType, ColorScheme scheme) {
    final type = contentType.toLowerCase();
    if (type.startsWith('image/')) return Colors.blue;
    if (type == 'application/pdf') return Colors.red;
    if (type.startsWith('video/')) return Colors.purple;
    if (type.startsWith('audio/')) return Colors.orange;
    return scheme.primary;
  }

  /// Formats a byte count into a human-readable size string.
  static String formatFileSize(int bytes) {
    if (bytes < 1024) return '$bytes B';
    if (bytes < 1024 * 1024) return '${(bytes / 1024).toStringAsFixed(1)} KB';
    if (bytes < 1024 * 1024 * 1024) {
      return '${(bytes / (1024 * 1024)).toStringAsFixed(1)} MB';
    }
    return '${(bytes / (1024 * 1024 * 1024)).toStringAsFixed(1)} GB';
  }

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final icon = iconForContentType(media.contentType);
    final iconColor = colorForContentType(media.contentType, theme.colorScheme);

    return Card(
      clipBehavior: Clip.antiAlias,
      child: InkWell(
        onTap: onTap,
        child: Padding(
          padding: const EdgeInsets.all(16),
          child: Row(
            children: [
              Container(
                width: 48,
                height: 48,
                decoration: BoxDecoration(
                  color: iconColor.withAlpha(25),
                  borderRadius: BorderRadius.circular(12),
                ),
                child: Icon(icon, color: iconColor, size: 24),
              ),
              const SizedBox(width: 16),
              Expanded(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Text(
                      media.filename,
                      style: theme.textTheme.titleSmall?.copyWith(
                        fontWeight: FontWeight.w600,
                      ),
                      maxLines: 1,
                      overflow: TextOverflow.ellipsis,
                    ),
                    const SizedBox(height: 4),
                    Row(
                      children: [
                        Text(
                          formatFileSize(media.fileSizeBytes.toInt()),
                          style: theme.textTheme.bodySmall?.copyWith(
                            color: theme.colorScheme.onSurfaceVariant,
                          ),
                        ),
                        const SizedBox(width: 8),
                        Text(
                          media.contentType,
                          style: theme.textTheme.bodySmall?.copyWith(
                            color: theme.colorScheme.onSurfaceVariant,
                          ),
                        ),
                      ],
                    ),
                  ],
                ),
              ),
              const SizedBox(width: 8),
              Column(
                crossAxisAlignment: CrossAxisAlignment.end,
                children: [
                  MediaStateBadge(state: media.state),
                  const SizedBox(height: 4),
                  ScanStatusBadge(status: media.scanStatus),
                ],
              ),
            ],
          ),
        ),
      ),
    );
  }
}
