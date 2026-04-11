import 'package:antinvestor_api_files/antinvestor_api_files.dart';
import 'package:antinvestor_ui_core/antinvestor_ui_core.dart';
import 'package:flutter/foundation.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';

import '../providers/files_providers.dart';
import '../widgets/file_preview_card.dart';
import '../widgets/media_state_badge.dart';
import '../widgets/scan_status_badge.dart';
import '../widgets/storage_usage_bar.dart';

/// Main file browser screen with AdminEntityListPage DataTable, CSV export,
/// search, filter chips, and storage stats header.
class FilesBrowserScreen extends ConsumerStatefulWidget {
  const FilesBrowserScreen({super.key});

  @override
  ConsumerState<FilesBrowserScreen> createState() =>
      _FilesBrowserScreenState();
}

class _FilesBrowserScreenState extends ConsumerState<FilesBrowserScreen> {
  String _searchQuery = '';
  MediaState? _stateFilter;

  MediaSearchParams get _searchParams => MediaSearchParams(
        query: _searchQuery,
        state: _stateFilter,
      );

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final asyncMedia = ref.watch(searchMediaProvider(_searchParams));
    final asyncStats = ref.watch(getStorageStatsProvider);

    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        // Storage stats header
        asyncStats.whenOrNull(
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
        const SizedBox(height: 8),

        // Main content
        Expanded(
          child: asyncMedia.when(
            loading: () => const Center(child: CircularProgressIndicator()),
            error: (error, _) => Center(
              child: Column(
                mainAxisSize: MainAxisSize.min,
                children: [
                  Icon(Icons.error_outline,
                      size: 48, color: theme.colorScheme.error),
                  const SizedBox(height: 16),
                  Text('$error', style: theme.textTheme.bodyLarge),
                  const SizedBox(height: 16),
                  FilledButton.tonal(
                    onPressed: _refresh,
                    child: const Text('Retry'),
                  ),
                ],
              ),
            ),
            data: (media) => AdminEntityListPage<MediaMetadata>(
              title: 'Files',
              breadcrumbs: const ['Home', 'Files'],
              columns: const [
                DataColumn(label: Text('Filename')),
                DataColumn(label: Text('Type')),
                DataColumn(label: Text('Size'), numeric: true),
                DataColumn(label: Text('State')),
                DataColumn(label: Text('Scan Status')),
                DataColumn(label: Text('Created')),
                DataColumn(label: Text('Visibility')),
              ],
              items: media,
              onSearch: (query) {
                setState(() => _searchQuery = query.trim());
              },
              searchHint: 'Search files...',
              onAdd: () => context.go('/files/upload'),
              addLabel: 'Upload',
              onRowNavigate: (m) {
                context.go('/files/${m.mediaId}', extra: m);
              },
              rowBuilder: (m, selected, onSelect) {
                return DataRow(
                  selected: selected,
                  onSelectChanged: (_) => onSelect(),
                  cells: [
                    DataCell(Text(
                      m.filename,
                      overflow: TextOverflow.ellipsis,
                    )),
                    DataCell(Text(m.contentType)),
                    DataCell(Text(
                        FilePreviewCard.formatFileSize(m.fileSizeBytes.toInt()))),
                    DataCell(MediaStateBadge(state: m.state)),
                    DataCell(ScanStatusBadge(status: m.scanStatus)),
                    DataCell(Text(m.createdAt.toDateTime().toIso8601String())),
                    DataCell(Text(m.visibility.name)),
                  ],
                );
              },
              exportRow: (m) => [
                m.filename,
                m.contentType,
                FilePreviewCard.formatFileSize(m.fileSizeBytes.toInt()),
                m.state.name,
                m.scanStatus.name,
                m.createdAt.toDateTime().toIso8601String(),
                m.visibility.name,
              ],
              onExport: (format, count) {
                debugPrint('[AUDIT] Exported $count Files as $format');
              },
            ),
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
