import 'package:antinvestor_api_files/antinvestor_api_files.dart';
import 'package:antinvestor_ui_core/antinvestor_ui_core.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../analytics/files_analytics.dart';
import '../providers/files_providers.dart';
import '../widgets/file_preview_card.dart';
import '../widgets/storage_usage_bar.dart';

/// Dashboard screen showing storage inventory and transfer activity.
///
/// Inventory totals (file count, stored bytes, users) stay on the entity
/// API via `GetStorageStats`. Activity KPIs and trends (uploads, downloads,
/// transferred bytes, cache hit ratio) come from the thesa analytics gate
/// ([analyticsDataSourceProvider]) using the `file_service_*` business
/// metrics. Tenant scoping is injected server-side; this screen never sends
/// tenant or partition filters.
class StorageDashboardScreen extends ConsumerStatefulWidget {
  const StorageDashboardScreen({super.key});

  @override
  ConsumerState<StorageDashboardScreen> createState() =>
      _StorageDashboardScreenState();
}

class _StorageDashboardScreenState
    extends ConsumerState<StorageDashboardScreen> {
  AnalyticsTimeRange _timeRange = AnalyticsTimeRange.last30Days();

  static const String _service = 'files';

  ServiceMetricsParams get _metricsParams =>
      ServiceMetricsParams(_service, timeRange: _timeRange);

  ServiceTimeSeriesParams _seriesParams(String metric) =>
      ServiceTimeSeriesParams(_service, metric, timeRange: _timeRange);

  void _refresh() {
    ref.invalidate(serviceMetricsProvider(_metricsParams));
    for (final metric in const [
      fileUploadsMetric,
      fileDownloadsMetric,
      fileUploadBytesMetric,
      fileDownloadBytesMetric,
    ]) {
      ref.invalidate(serviceTimeSeriesProvider(_seriesParams(metric)));
    }
  }

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final asyncStats = ref.watch(getStorageStatsProvider);
    final metricsAsync = ref.watch(serviceMetricsProvider(_metricsParams));
    final isDesktop = AppBreakpoints.isDesktop(
      MediaQuery.sizeOf(context).width,
    );

    final transferCountsCard = _ChartCard(
      title: 'Transfer activity',
      subtitle: 'Uploads and downloads over time',
      child: _buildDualSeries(fileUploadsMetric, fileDownloadsMetric),
    );

    final transferBytesCard = _ChartCard(
      title: 'Transfer volume',
      subtitle: 'Bytes uploaded and downloaded over time',
      child: _buildDualSeries(fileUploadBytesMetric, fileDownloadBytesMetric),
    );

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
              const Spacer(),
              IconButton(
                icon: const Icon(Icons.refresh),
                tooltip: 'Refresh',
                onPressed: () {
                  ref.invalidate(getStorageStatsProvider);
                  _refresh();
                },
              ),
            ],
          ),
          const SizedBox(height: 24),

          // Inventory (entity API)
          asyncStats.when(
            loading: () => const SizedBox(
              height: 160,
              child: Center(child: CircularProgressIndicator()),
            ),
            error: (error, _) => Center(
              child: Column(
                mainAxisSize: MainAxisSize.min,
                children: [
                  Icon(
                    Icons.error_outline,
                    size: 48,
                    color: theme.colorScheme.error,
                  ),
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
            data: (stats) => _InventorySection(theme: theme, stats: stats),
          ),
          const SizedBox(height: 24),

          // Activity (thesa analytics gate)
          Row(
            children: [
              Expanded(
                child: Text(
                  'Activity',
                  style: theme.textTheme.titleMedium?.copyWith(
                    fontWeight: FontWeight.w600,
                  ),
                ),
              ),
              SingleChildScrollView(
                scrollDirection: Axis.horizontal,
                reverse: true,
                child: TimeRangeSelector(
                  value: _timeRange,
                  onChanged: (range) => setState(() => _timeRange = range),
                ),
              ),
            ],
          ),
          const SizedBox(height: 16),
          metricsAsync.when(
            data: (metrics) => metrics.isEmpty
                ? const AnalyticsGateErrorCard(
                    message:
                        'Analytics are not configured for the files '
                        'service in this app.',
                  )
                : MetricsRow(metrics: _displayMetrics(metrics)),
            loading: () => const MetricsRow(metrics: [], isLoading: true),
            error: (e, _) => AnalyticsGateErrorCard(
              message: analyticsGateMessage(e),
              onRetry: _refresh,
            ),
          ),
          const SizedBox(height: 20),
          if (isDesktop)
            IntrinsicHeight(
              child: Row(
                crossAxisAlignment: CrossAxisAlignment.stretch,
                children: [
                  Expanded(child: transferCountsCard),
                  const SizedBox(width: 16),
                  Expanded(child: transferBytesCard),
                ],
              ),
            )
          else ...[
            transferCountsCard,
            const SizedBox(height: 16),
            transferBytesCard,
          ],
        ],
      ),
    );
  }

  /// KPI cards shown to the user: transfer counters and volumes plus the
  /// derived cache hit ratio (raw hit/miss scalars are folded into it).
  List<MetricValue> _displayMetrics(List<MetricValue> metrics) {
    final visible = [
      for (final m in metrics)
        if (m.key != fileCacheHitsKpiKey && m.key != fileCacheMissesKpiKey) m,
    ];
    final ratio = cacheHitRatioPercent(metrics);
    if (ratio != null) {
      visible.add(
        MetricValue(
          key: 'cache_hit_ratio',
          label: 'Cache hit ratio',
          value: ratio,
          unit: 'percent',
          icon: Icons.bolt_outlined,
        ),
      );
    }
    return visible;
  }

  /// Renders two gate time series (e.g. uploads vs downloads) on one chart.
  Widget _buildDualSeries(String metricA, String metricB) {
    final asyncA = ref.watch(serviceTimeSeriesProvider(_seriesParams(metricA)));
    final asyncB = ref.watch(serviceTimeSeriesProvider(_seriesParams(metricB)));

    final error = asyncA.error ?? asyncB.error;
    if (error != null) {
      return AnalyticsGateErrorCard(
        message: analyticsGateMessage(error),
        onRetry: _refresh,
      );
    }
    if (asyncA.isLoading || asyncB.isLoading) {
      return const SizedBox(
        height: 240,
        child: Center(child: CircularProgressIndicator()),
      );
    }
    return TimeSeriesChart(
      series: [...?asyncA.value, ...?asyncB.value],
      granularity: _timeRange.granularity,
    );
  }
}

/// Friendly inline error/empty card for analytics gate failures.
class AnalyticsGateErrorCard extends StatelessWidget {
  const AnalyticsGateErrorCard({
    super.key,
    required this.message,
    this.onRetry,
  });

  final String message;
  final VoidCallback? onRetry;

  @override
  Widget build(BuildContext context) {
    final cs = Theme.of(context).colorScheme;
    return Container(
      padding: const EdgeInsets.all(16),
      decoration: BoxDecoration(
        color: cs.errorContainer.withValues(alpha: 0.25),
        borderRadius: BorderRadius.circular(8),
      ),
      child: Row(
        children: [
          Icon(Icons.insights_outlined, color: cs.error, size: 20),
          const SizedBox(width: 8),
          Expanded(
            child: Text(
              message,
              style: TextStyle(color: cs.error, fontSize: 13),
            ),
          ),
          if (onRetry != null) ...[
            const SizedBox(width: 8),
            TextButton(onPressed: onRetry, child: const Text('Retry')),
          ],
        ],
      ),
    );
  }
}

/// Inventory cards backed by the `GetStorageStats` entity API.
class _InventorySection extends StatelessWidget {
  const _InventorySection({required this.theme, required this.stats});

  final ThemeData theme;
  final GetStorageStatsResponse stats;

  @override
  Widget build(BuildContext context) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
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
                value: FilePreviewCard.formatFileSize(stats.totalBytes.toInt()),
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
    );
  }
}

class _ChartCard extends StatelessWidget {
  const _ChartCard({
    required this.title,
    required this.subtitle,
    required this.child,
  });

  final String title;
  final String subtitle;
  final Widget child;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final cs = theme.colorScheme;
    return Container(
      padding: const EdgeInsets.all(20),
      decoration: BoxDecoration(
        color: cs.surface,
        borderRadius: BorderRadius.circular(12),
        border: Border.all(color: cs.outlineVariant),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(
            title,
            style: theme.textTheme.titleMedium?.copyWith(
              fontWeight: FontWeight.w600,
            ),
          ),
          const SizedBox(height: 4),
          Text(
            subtitle,
            style: theme.textTheme.bodySmall?.copyWith(
              color: cs.onSurfaceVariant,
            ),
          ),
          const SizedBox(height: 16),
          child,
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
