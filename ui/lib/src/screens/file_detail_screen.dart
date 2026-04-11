import 'package:antinvestor_api_files/antinvestor_api_files.dart';
import 'package:antinvestor_ui_core/widgets/error_helpers.dart';
import 'package:antinvestor_ui_core/widgets/form_field_card.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';

import '../providers/access_providers.dart';
import '../providers/files_providers.dart';
import '../providers/version_providers.dart';
import '../widgets/access_role_badge.dart';
import '../widgets/file_access_chip.dart';
import '../widgets/file_preview_card.dart';
import '../widgets/media_state_badge.dart';
import '../widgets/scan_status_badge.dart';
import '../widgets/version_history_timeline.dart';

/// Detail screen for a single file with tabs: Preview, Metadata, Access, Versions.
class FileDetailScreen extends ConsumerStatefulWidget {
  const FileDetailScreen({
    super.key,
    required this.contentId,
    this.initialMedia,
  });

  final String contentId;
  final MediaMetadata? initialMedia;

  @override
  ConsumerState<FileDetailScreen> createState() => _FileDetailScreenState();
}

class _FileDetailScreenState extends ConsumerState<FileDetailScreen>
    with SingleTickerProviderStateMixin {
  late final TabController _tabController;

  // Metadata editing
  final _filenameController = TextEditingController();
  final _contentTypeController = TextEditingController();
  bool _isEditing = false;

  @override
  void initState() {
    super.initState();
    _tabController = TabController(length: 4, vsync: this);
    if (widget.initialMedia != null) {
      _filenameController.text = widget.initialMedia!.filename;
      _contentTypeController.text = widget.initialMedia!.contentType;
    }
  }

  @override
  void dispose() {
    _tabController.dispose();
    _filenameController.dispose();
    _contentTypeController.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final asyncContent = ref.watch(getContentProvider(widget.contentId));

    return asyncContent.when(
      loading: () => const Center(child: CircularProgressIndicator()),
      error: (error, _) => Center(
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            Icon(Icons.error_outline, size: 48, color: theme.colorScheme.error),
            const SizedBox(height: 16),
            Text(friendlyError(error), style: theme.textTheme.bodyLarge),
            const SizedBox(height: 16),
            FilledButton.tonal(
              onPressed: () =>
                  ref.invalidate(getContentProvider(widget.contentId)),
              child: const Text('Retry'),
            ),
          ],
        ),
      ),
      data: (media) {
        // Update editing controllers if not actively editing
        if (!_isEditing) {
          _filenameController.text = media.filename;
          _contentTypeController.text = media.contentType;
        }
        return _buildContent(theme, media);
      },
    );
  }

  Widget _buildContent(ThemeData theme, MediaMetadata media) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        // Header
        Padding(
          padding: const EdgeInsets.fromLTRB(24, 24, 24, 0),
          child: Row(
            children: [
              Container(
                width: 56,
                height: 56,
                decoration: BoxDecoration(
                  color: FilePreviewCard.colorForContentType(
                          media.contentType, theme.colorScheme)
                      .withAlpha(25),
                  borderRadius: BorderRadius.circular(14),
                ),
                child: Icon(
                  FilePreviewCard.iconForContentType(media.contentType),
                  color: FilePreviewCard.colorForContentType(
                      media.contentType, theme.colorScheme),
                  size: 28,
                ),
              ),
              const SizedBox(width: 16),
              Expanded(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Text(
                      media.filename,
                      style: theme.textTheme.headlineSmall?.copyWith(
                        fontWeight: FontWeight.w600,
                        letterSpacing: -0.3,
                      ),
                    ),
                    const SizedBox(height: 4),
                    Row(
                      children: [
                        MediaStateBadge(state: media.state),
                        const SizedBox(width: 8),
                        ScanStatusBadge(status: media.scanStatus),
                      ],
                    ),
                  ],
                ),
              ),
              PopupMenuButton<String>(
                onSelected: (action) => _handleAction(action, media),
                itemBuilder: (context) => [
                  const PopupMenuItem(
                    value: 'edit',
                    child: ListTile(
                      leading: Icon(Icons.edit),
                      title: Text('Edit metadata'),
                      dense: true,
                    ),
                  ),
                  const PopupMenuItem(
                    value: 'delete',
                    child: ListTile(
                      leading: Icon(Icons.delete, color: Colors.red),
                      title:
                          Text('Delete', style: TextStyle(color: Colors.red)),
                      dense: true,
                    ),
                  ),
                ],
              ),
            ],
          ),
        ),

        // Tab bar
        TabBar(
          controller: _tabController,
          tabs: const [
            Tab(text: 'Preview'),
            Tab(text: 'Metadata'),
            Tab(text: 'Access'),
            Tab(text: 'Versions'),
          ],
        ),

        // Tab content
        Expanded(
          child: TabBarView(
            controller: _tabController,
            children: [
              _buildPreviewTab(theme, media),
              _buildMetadataTab(theme, media),
              _buildAccessTab(theme),
              _buildVersionsTab(theme),
            ],
          ),
        ),
      ],
    );
  }

  Widget _buildPreviewTab(ThemeData theme, MediaMetadata media) {
    final icon = FilePreviewCard.iconForContentType(media.contentType);
    final iconColor =
        FilePreviewCard.colorForContentType(media.contentType, theme.colorScheme);

    return SingleChildScrollView(
      padding: const EdgeInsets.all(24),
      child: Column(
        children: [
          const SizedBox(height: 32),
          Container(
            width: 120,
            height: 120,
            decoration: BoxDecoration(
              color: iconColor.withAlpha(25),
              borderRadius: BorderRadius.circular(24),
            ),
            child: Icon(icon, color: iconColor, size: 64),
          ),
          const SizedBox(height: 24),
          Text(
            media.filename,
            style: theme.textTheme.headlineMedium?.copyWith(
              fontWeight: FontWeight.w600,
            ),
            textAlign: TextAlign.center,
          ),
          const SizedBox(height: 8),
          Text(
            media.contentType,
            style: theme.textTheme.bodyLarge?.copyWith(
              color: theme.colorScheme.onSurfaceVariant,
            ),
          ),
          const SizedBox(height: 4),
          Text(
            FilePreviewCard.formatFileSize(media.fileSizeBytes.toInt()),
            style: theme.textTheme.bodyLarge?.copyWith(
              color: theme.colorScheme.onSurfaceVariant,
            ),
          ),
          if (media.contentUri.isNotEmpty) ...[
            const SizedBox(height: 16),
            FilledButton.icon(
              onPressed: () {
                // Content URI could be used for downloading
              },
              icon: const Icon(Icons.download, size: 18),
              label: const Text('Download'),
            ),
          ],
        ],
      ),
    );
  }

  Widget _buildMetadataTab(ThemeData theme, MediaMetadata media) {
    if (_isEditing) {
      return SingleChildScrollView(
        padding: const EdgeInsets.all(24),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.stretch,
          children: [
            FormFieldCard(
              label: 'Filename',
              isRequired: true,
              child: TextField(
                controller: _filenameController,
                decoration: const InputDecoration(
                  hintText: 'Enter filename',
                ),
              ),
            ),
            FormFieldCard(
              label: 'Content Type',
              isRequired: true,
              child: TextField(
                controller: _contentTypeController,
                decoration: const InputDecoration(
                  hintText: 'e.g. application/pdf',
                ),
              ),
            ),
            const SizedBox(height: 16),
            Row(
              mainAxisAlignment: MainAxisAlignment.end,
              children: [
                TextButton(
                  onPressed: () => setState(() => _isEditing = false),
                  child: const Text('Cancel'),
                ),
                const SizedBox(width: 12),
                FilledButton(
                  onPressed: () => _saveMetadata(media),
                  child: const Text('Save'),
                ),
              ],
            ),
          ],
        ),
      );
    }

    final entries = <MapEntry<String, String>>[
      MapEntry('Media ID', media.mediaId),
      MapEntry('Filename', media.filename),
      MapEntry('Content Type', media.contentType),
      MapEntry('File Size',
          FilePreviewCard.formatFileSize(media.fileSizeBytes.toInt())),
      MapEntry('State', media.state.name),
      MapEntry('Scan Status', media.scanStatus.name),
      MapEntry('Visibility', media.visibility.name),
      MapEntry('Content URI', media.contentUri),
      ...media.labels.entries.map((e) => MapEntry('Label: ${e.key}', e.value)),
    ];

    return ListView.separated(
      padding: const EdgeInsets.all(24),
      itemCount: entries.length,
      separatorBuilder: (_, __) => const Divider(height: 1),
      itemBuilder: (context, index) {
        final entry = entries[index];
        return Padding(
          padding: const EdgeInsets.symmetric(vertical: 12),
          child: Row(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              SizedBox(
                width: 140,
                child: Text(
                  entry.key,
                  style: theme.textTheme.bodyMedium?.copyWith(
                    color: theme.colorScheme.onSurfaceVariant,
                    fontWeight: FontWeight.w500,
                  ),
                ),
              ),
              Expanded(
                child: Text(
                  entry.value.isEmpty ? '-' : entry.value,
                  style: theme.textTheme.bodyMedium,
                ),
              ),
            ],
          ),
        );
      },
    );
  }

  Widget _buildAccessTab(ThemeData theme) {
    final asyncAccess = ref.watch(listAccessProvider(widget.contentId));

    return asyncAccess.when(
      loading: () => const Center(child: CircularProgressIndicator()),
      error: (error, _) => Center(
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            Icon(Icons.error_outline, size: 48, color: theme.colorScheme.error),
            const SizedBox(height: 16),
            Text(friendlyError(error)),
            const SizedBox(height: 16),
            FilledButton.tonal(
              onPressed: () =>
                  ref.invalidate(listAccessProvider(widget.contentId)),
              child: const Text('Retry'),
            ),
          ],
        ),
      ),
      data: (grants) {
        if (grants.isEmpty) {
          return Center(
            child: Column(
              mainAxisSize: MainAxisSize.min,
              children: [
                Icon(Icons.lock_outline,
                    size: 48,
                    color: theme.colorScheme.onSurfaceVariant.withAlpha(120)),
                const SizedBox(height: 12),
                Text(
                  'No access grants',
                  style: theme.textTheme.bodyLarge?.copyWith(
                    color: theme.colorScheme.onSurfaceVariant,
                  ),
                ),
                const SizedBox(height: 12),
                FilledButton.icon(
                  onPressed: () =>
                      context.go('/files/${widget.contentId}/access'),
                  icon: const Icon(Icons.person_add, size: 18),
                  label: const Text('Manage Access'),
                ),
              ],
            ),
          );
        }
        return Column(
          crossAxisAlignment: CrossAxisAlignment.stretch,
          children: [
            Padding(
              padding: const EdgeInsets.fromLTRB(24, 16, 24, 8),
              child: Row(
                children: [
                  Expanded(
                    child: Text(
                      '${grants.length} access grants',
                      style: theme.textTheme.titleSmall,
                    ),
                  ),
                  FilledButton.icon(
                    onPressed: () =>
                        context.go('/files/${widget.contentId}/access'),
                    icon: const Icon(Icons.settings, size: 18),
                    label: const Text('Manage'),
                  ),
                ],
              ),
            ),
            Expanded(
              child: ListView.builder(
                padding: const EdgeInsets.symmetric(horizontal: 24),
                itemCount: grants.length,
                itemBuilder: (context, index) {
                  return Padding(
                    padding: const EdgeInsets.only(bottom: 8),
                    child: FileAccessChip(grant: grants[index]),
                  );
                },
              ),
            ),
          ],
        );
      },
    );
  }

  Widget _buildVersionsTab(ThemeData theme) {
    final asyncVersions = ref.watch(getVersionsProvider(widget.contentId));

    return asyncVersions.when(
      loading: () => const Center(child: CircularProgressIndicator()),
      error: (error, _) => Center(
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            Icon(Icons.error_outline, size: 48, color: theme.colorScheme.error),
            const SizedBox(height: 16),
            Text(friendlyError(error)),
            const SizedBox(height: 16),
            FilledButton.tonal(
              onPressed: () =>
                  ref.invalidate(getVersionsProvider(widget.contentId)),
              child: const Text('Retry'),
            ),
          ],
        ),
      ),
      data: (versions) {
        return VersionHistoryTimeline(
          versions: versions,
          currentVersion: versions.isNotEmpty ? versions.first.version.toInt() : null,
          onRestore: (versionNumber) async {
            final confirmed = await _confirmRestore(theme, versionNumber);
            if (confirmed && mounted) {
              try {
                await ref.read(versionNotifierProvider.notifier).restoreVersion(
                      mediaId: widget.contentId,
                      versionNumber: versionNumber,
                    );
                if (mounted) {
                  ScaffoldMessenger.of(context).showSnackBar(
                    SnackBar(
                        content:
                            Text('Restored to version $versionNumber')),
                  );
                  ref.invalidate(getContentProvider(widget.contentId));
                  ref.invalidate(getVersionsProvider(widget.contentId));
                }
              } catch (e) {
                if (mounted) {
                  ScaffoldMessenger.of(context).showSnackBar(
                    SnackBar(content: Text(friendlyError(e))),
                  );
                }
              }
            }
          },
        );
      },
    );
  }

  Future<bool> _confirmRestore(ThemeData theme, int versionNumber) async {
    return await showDialog<bool>(
          context: context,
          builder: (context) => AlertDialog(
            title: const Text('Restore Version'),
            content:
                Text('Restore to version $versionNumber? This will create a new version.'),
            actions: [
              TextButton(
                onPressed: () => Navigator.pop(context, false),
                child: const Text('Cancel'),
              ),
              FilledButton(
                onPressed: () => Navigator.pop(context, true),
                child: const Text('Restore'),
              ),
            ],
          ),
        ) ??
        false;
  }

  void _handleAction(String action, MediaMetadata media) {
    switch (action) {
      case 'edit':
        setState(() => _isEditing = true);
        _tabController.animateTo(1); // Switch to metadata tab
      case 'delete':
        _confirmDelete(media);
    }
  }

  Future<void> _confirmDelete(MediaMetadata media) async {
    final confirmed = await showDialog<bool>(
      context: context,
      builder: (context) => AlertDialog(
        title: const Text('Delete File'),
        content: Text('Are you sure you want to delete "${media.filename}"?'),
        actions: [
          TextButton(
            onPressed: () => Navigator.pop(context, false),
            child: const Text('Cancel'),
          ),
          FilledButton(
            style: FilledButton.styleFrom(backgroundColor: Colors.red),
            onPressed: () => Navigator.pop(context, true),
            child: const Text('Delete'),
          ),
        ],
      ),
    );

    if (confirmed == true && mounted) {
      try {
        await ref
            .read(contentNotifierProvider.notifier)
            .deleteContent(media.mediaId);
        if (mounted) {
          ScaffoldMessenger.of(context).showSnackBar(
            const SnackBar(content: Text('File deleted')),
          );
          context.go('/files');
        }
      } catch (e) {
        if (mounted) {
          ScaffoldMessenger.of(context).showSnackBar(
            SnackBar(content: Text(friendlyError(e))),
          );
        }
      }
    }
  }

  Future<void> _saveMetadata(MediaMetadata media) async {
    try {
      await ref.read(contentNotifierProvider.notifier).patchContent(
            mediaId: media.mediaId,
            filename: _filenameController.text.trim(),
            contentType: _contentTypeController.text.trim(),
          );
      if (mounted) {
        setState(() => _isEditing = false);
        ref.invalidate(getContentProvider(widget.contentId));
        ScaffoldMessenger.of(context).showSnackBar(
          const SnackBar(content: Text('Metadata updated')),
        );
      }
    } catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: Text(friendlyError(e))),
        );
      }
    }
  }
}
