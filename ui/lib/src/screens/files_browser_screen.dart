import 'package:antinvestor_api_files/antinvestor_api_files.dart';
import 'package:antinvestor_ui_core/widgets/entity_list_page.dart';
import 'package:antinvestor_ui_core/widgets/error_helpers.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';

import '../providers/files_providers.dart';
import '../widgets/file_preview_card.dart';
import '../widgets/storage_usage_bar.dart';

/// Main file browser screen with grid/list toggle, search, filters, and stats.
class FilesBrowserScreen extends ConsumerStatefulWidget {
  const FilesBrowserScreen({super.key});

  @override
  ConsumerState<FilesBrowserScreen> createState() =>
      _FilesBrowserScreenState();
}

class _FilesBrowserScreenState extends ConsumerState<FilesBrowserScreen> {
  String _searchQuery = '';
  MediaState? _stateFilter;
  bool _isGridView = false;

  MediaSearchParams get _searchParams => MediaSearchParams(
        query: _searchQuery,
        state: _stateFilter,
      );

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final asyncMedia = ref.watch(searchMediaProvider(_searchParams));
    final asyncStats = ref.watch(getStorageStatsProvider);

    return asyncMedia.when(
      loading: () => _buildShell(theme, isLoading: true, items: const []),
      error: (error, _) => _buildShell(
        theme,
        error: friendlyError(error),
        items: const [],
      ),
      data: (media) => _buildShell(theme, items: media, stats: asyncStats),
    );
  }

  Widget _buildShell(
    ThemeData theme, {
    required List<MediaMetadata> items,
    bool isLoading = false,
    String? error,
    AsyncValue<StorageStatsResponse>? stats,
  }) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        // Storage stats header
        if (stats != null)
          stats.whenOrNull(
                data: (s) => Padding(
                  padding: const EdgeInsets.fromLTRB(24, 16, 24, 0),
                  child: StorageUsageBar(
                    usedBytes: s.usedBytes.toInt(),
                    totalBytes: s.totalBytes.toInt(),
                    label: 'Storage Usage',
                  ),
                ),
              ) ??
              const SizedBox.shrink(),

        // Filter chips row
        Padding(
          padding: const EdgeInsets.fromLTRB(24, 12, 24, 0),
          child: Row(
            children: [
              Expanded(
                child: SingleChildScrollView(
                  scrollDirection: Axis.horizontal,
                  child: Row(
                    children: [
                      _filterChip(theme, null, 'All'),
                      const SizedBox(width: 8),
                      _filterChip(theme, MediaState.AVAILABLE, 'Available'),
                      const SizedBox(width: 8),
                      _filterChip(theme, MediaState.CREATING, 'Creating'),
                      const SizedBox(width: 8),
                      _filterChip(theme, MediaState.ARCHIVED, 'Archived'),
                      const SizedBox(width: 8),
                      _filterChip(theme, MediaState.DELETED, 'Deleted'),
                    ],
                  ),
                ),
              ),
              const SizedBox(width: 8),
              IconButton(
                icon: Icon(
                  _isGridView ? Icons.view_list : Icons.grid_view,
                  size: 22,
                ),
                onPressed: () => setState(() => _isGridView = !_isGridView),
                tooltip: _isGridView ? 'List view' : 'Grid view',
              ),
            ],
          ),
        ),

        // Main content
        Expanded(
          child: _isGridView
              ? _buildGridView(theme, items, isLoading, error)
              : _buildListView(theme, items, isLoading, error),
        ),
      ],
    );
  }

  Widget _buildListView(
    ThemeData theme,
    List<MediaMetadata> items,
    bool isLoading,
    String? error,
  ) {
    return EntityListPage<MediaMetadata>(
      title: 'Files',
      icon: Icons.folder,
      items: items,
      isLoading: isLoading,
      error: error,
      onRetry: _refresh,
      searchHint: 'Search files...',
      onSearchChanged: (query) {
        setState(() => _searchQuery = query.trim());
      },
      actionLabel: 'Upload',
      onAction: () => context.go('/files/upload'),
      itemBuilder: (context, media) {
        return FilePreviewCard(
          media: media,
          onTap: () => context.go('/files/${media.mediaId}', extra: media),
        );
      },
    );
  }

  Widget _buildGridView(
    ThemeData theme,
    List<MediaMetadata> items,
    bool isLoading,
    String? error,
  ) {
    if (isLoading) {
      return const Center(child: CircularProgressIndicator());
    }
    if (error != null) {
      return Center(
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            Icon(Icons.error_outline, size: 48, color: theme.colorScheme.error),
            const SizedBox(height: 16),
            Text(error, style: theme.textTheme.bodyLarge),
            const SizedBox(height: 16),
            FilledButton.tonal(
              onPressed: _refresh,
              child: const Text('Retry'),
            ),
          ],
        ),
      );
    }
    if (items.isEmpty) {
      return Center(
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            Icon(Icons.folder_open, size: 64,
                color: theme.colorScheme.primary.withAlpha(120)),
            const SizedBox(height: 16),
            Text('No files found',
                style: theme.textTheme.titleMedium?.copyWith(
                  color: theme.colorScheme.onSurfaceVariant,
                )),
            const SizedBox(height: 12),
            FilledButton.icon(
              onPressed: () => context.go('/files/upload'),
              icon: const Icon(Icons.upload, size: 18),
              label: const Text('Upload'),
            ),
          ],
        ),
      );
    }

    return Column(
      children: [
        // Search row for grid view
        Padding(
          padding: const EdgeInsets.fromLTRB(24, 8, 24, 8),
          child: Row(
            children: [
              Expanded(
                child: TextField(
                  onChanged: (q) => setState(() => _searchQuery = q.trim()),
                  decoration: const InputDecoration(
                    hintText: 'Search files...',
                    prefixIcon: Icon(Icons.search, size: 20),
                  ),
                ),
              ),
              const SizedBox(width: 12),
              FilledButton.icon(
                onPressed: () => context.go('/files/upload'),
                icon: const Icon(Icons.upload, size: 18),
                label: const Text('Upload'),
              ),
            ],
          ),
        ),
        Expanded(
          child: GridView.builder(
            padding: const EdgeInsets.all(24),
            gridDelegate: const SliverGridDelegateWithMaxCrossAxisExtent(
              maxCrossAxisExtent: 240,
              mainAxisSpacing: 12,
              crossAxisSpacing: 12,
              childAspectRatio: 0.85,
            ),
            itemCount: items.length,
            itemBuilder: (context, index) {
              final media = items[index];
              return _GridFileCard(
                media: media,
                onTap: () =>
                    context.go('/files/${media.mediaId}', extra: media),
              );
            },
          ),
        ),
      ],
    );
  }

  Widget _filterChip(ThemeData theme, MediaState? value, String label) {
    final isSelected = _stateFilter == value;
    return FilterChip(
      selected: isSelected,
      label: Text(label),
      selectedColor: theme.colorScheme.secondaryContainer,
      shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(8)),
      onSelected: (_) => setState(() => _stateFilter = value),
    );
  }

  void _refresh() {
    ref.invalidate(searchMediaProvider(_searchParams));
    ref.invalidate(getStorageStatsProvider);
  }
}

/// Grid card for file preview.
class _GridFileCard extends StatelessWidget {
  const _GridFileCard({required this.media, this.onTap});

  final MediaMetadata media;
  final VoidCallback? onTap;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final icon = FilePreviewCard.iconForContentType(media.contentType);
    final iconColor =
        FilePreviewCard.colorForContentType(media.contentType, theme.colorScheme);

    return Card(
      clipBehavior: Clip.antiAlias,
      child: InkWell(
        onTap: onTap,
        child: Column(
          mainAxisAlignment: MainAxisAlignment.center,
          children: [
            Container(
              width: 56,
              height: 56,
              decoration: BoxDecoration(
                color: iconColor.withAlpha(25),
                borderRadius: BorderRadius.circular(14),
              ),
              child: Icon(icon, color: iconColor, size: 28),
            ),
            const SizedBox(height: 12),
            Padding(
              padding: const EdgeInsets.symmetric(horizontal: 8),
              child: Text(
                media.filename,
                style: theme.textTheme.bodySmall?.copyWith(
                  fontWeight: FontWeight.w600,
                ),
                maxLines: 2,
                overflow: TextOverflow.ellipsis,
                textAlign: TextAlign.center,
              ),
            ),
            const SizedBox(height: 4),
            Text(
              FilePreviewCard.formatFileSize(media.fileSizeBytes.toInt()),
              style: theme.textTheme.labelSmall?.copyWith(
                color: theme.colorScheme.onSurfaceVariant,
              ),
            ),
          ],
        ),
      ),
    );
  }
}
