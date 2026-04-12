import 'package:antinvestor_api_files/antinvestor_api_files.dart';
import 'package:antinvestor_ui_core/widgets/error_helpers.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../providers/files_providers.dart';
import '../widgets/file_preview_card.dart';
import '../widgets/storage_usage_bar.dart';

/// Dashboard screen showing storage usage statistics.
class StorageDashboardScreen extends ConsumerWidget {
  const StorageDashboardScreen({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final theme = Theme.of(context);
    final asyncStats = ref.watch(getStorageStatsProvider);

    return asyncStats.when(
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
              onPressed: () => ref.invalidate(getStorageStatsProvider),
              child: const Text('Retry'),
            ),
          ],
        ),
      ),
      data: (stats) => _buildDashboard(theme, stats),
    );
  }

  Widget _buildDashboard(ThemeData theme, GetStorageStatsResponse stats) {
    return SingleChildScrollView(
      padding: const EdgeInsets.all(24),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          // Header
          Row(
            children: [
              Icon(Icons.storage, size: 28, color: theme.colorScheme.primary),
              const SizedBox(width: 12),
              Text(
                'Storage Dashboard',
                style: theme.textTheme.headlineSmall?.copyWith(
                  fontWeight: FontWeight.w600,
                  letterSpacing: -0.3,
                ),
              ),
            ],
          ),
          const SizedBox(height: 24),

          // Overall usage bar
          Card(
            child: Padding(
              padding: const EdgeInsets.all(20),
              child: StorageUsageBar(
                usedBytes: stats.totalBytes.toInt(),
                totalBytes: stats.totalBytes.toInt(),
                label: 'Total Storage',
              ),
            ),
          ),
          const SizedBox(height: 16),

          // Summary stat cards
          Row(
            children: [
              Expanded(
                child: _StatCard(
                  theme: theme,
                  icon: Icons.folder,
                  iconColor: theme.colorScheme.primary,
                  label: 'Total Files',
                  value: '${stats.totalFiles}',
                ),
              ),
              const SizedBox(width: 12),
              Expanded(
                child: _StatCard(
                  theme: theme,
                  icon: Icons.data_usage,
                  iconColor: Colors.teal,
                  label: 'Total Size',
                  value: FilePreviewCard.formatFileSize(
                      stats.totalBytes.toInt()),
                ),
              ),
            ],
          ),
          const SizedBox(height: 12),
          Row(
            children: [
              Expanded(
                child: _StatCard(
                  theme: theme,
                  icon: Icons.people,
                  iconColor: Colors.green,
                  label: 'Total Users',
                  value: '${stats.totalUsers}',
                ),
              ),
              const SizedBox(width: 12),
              const Expanded(child: SizedBox.shrink()),
            ],
          ),
        ],
      ),
    );
  }
}

class _StatCard extends StatelessWidget {
  const _StatCard({
    required this.theme,
    required this.icon,
    required this.iconColor,
    required this.label,
    required this.value,
  });

  final ThemeData theme;
  final IconData icon;
  final Color iconColor;
  final String label;
  final String value;

  @override
  Widget build(BuildContext context) {
    return Card(
      child: Padding(
        padding: const EdgeInsets.all(16),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Row(
              children: [
                Icon(icon, size: 20, color: iconColor),
                const SizedBox(width: 8),
                Text(
                  label,
                  style: theme.textTheme.bodySmall?.copyWith(
                    color: theme.colorScheme.onSurfaceVariant,
                  ),
                ),
              ],
            ),
            const SizedBox(height: 8),
            Text(
              value,
              style: theme.textTheme.headlineMedium?.copyWith(
                fontWeight: FontWeight.w700,
              ),
            ),
          ],
        ),
      ),
    );
  }
}
