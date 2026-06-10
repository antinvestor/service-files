import 'package:antinvestor_ui_core/antinvestor_ui_core.dart';
import 'package:flutter/material.dart';

/// Metric names emitted by service-files' business instrumentation
/// (`file_service_*` counters in apps/default/service/metrics/metrics.go).
const String fileUploadsMetric = 'file_service_uploads_total';
const String fileDownloadsMetric = 'file_service_downloads_total';
const String fileUploadBytesMetric = 'file_service_upload_bytes_total';
const String fileDownloadBytesMetric = 'file_service_download_bytes_total';
const String fileCacheHitsMetric = 'file_service_cache_hits_total';
const String fileCacheMissesMetric = 'file_service_cache_misses_total';

/// KPI keys used by [filesAnalyticsSpec].
const String fileCacheHitsKpiKey = 'cache_hits';
const String fileCacheMissesKpiKey = 'cache_misses';

/// Analytics catalog for the files service, consumed by the thesa-gated
/// metrics pipeline.
///
/// Host apps register this spec on their [ThesaAnalyticsDataSource]:
///
/// ```dart
/// analyticsDataSourceProvider.overrideWith(
///   (ref) => ThesaAnalyticsDataSource(transport, specs: [filesAnalyticsSpec]),
/// );
/// ```
///
/// Inventory totals (file counts, stored bytes, users) stay on the entity
/// API (`GetStorageStats`); only activity rates and trends are queried
/// through the gate. Tenant scoping is injected server-side from the
/// caller's JWT; no tenant/partition filters are declared (or ever sent).
const ServiceAnalyticsSpec filesAnalyticsSpec = ServiceAnalyticsSpec(
  service: 'files',
  kpis: [
    KpiSpec(
      'uploads',
      label: 'Uploads',
      metric: fileUploadsMetric,
      unit: 'count',
      icon: Icons.upload_outlined,
    ),
    KpiSpec(
      'downloads',
      label: 'Downloads',
      metric: fileDownloadsMetric,
      unit: 'count',
      icon: Icons.download_outlined,
    ),
    KpiSpec(
      'upload_bytes',
      label: 'Data uploaded',
      metric: fileUploadBytesMetric,
      unit: 'bytes',
      icon: Icons.cloud_upload_outlined,
    ),
    KpiSpec(
      'download_bytes',
      label: 'Data downloaded',
      metric: fileDownloadBytesMetric,
      unit: 'bytes',
      icon: Icons.cloud_download_outlined,
    ),
    KpiSpec(
      fileCacheHitsKpiKey,
      label: 'Cache hits',
      metric: fileCacheHitsMetric,
      unit: 'count',
      icon: Icons.bolt_outlined,
    ),
    KpiSpec(
      fileCacheMissesKpiKey,
      label: 'Cache misses',
      metric: fileCacheMissesMetric,
      unit: 'count',
      icon: Icons.bolt_outlined,
    ),
  ],
  charts: [
    ChartConfig.timeSeries(fileUploadsMetric, label: 'Uploads'),
    ChartConfig.timeSeries(fileDownloadsMetric, label: 'Downloads'),
    ChartConfig.timeSeries(fileUploadBytesMetric, label: 'Data uploaded'),
    ChartConfig.timeSeries(fileDownloadBytesMetric, label: 'Data downloaded'),
  ],
);

/// Derives the cache hit ratio (percent) from the gate-fetched KPI values.
///
/// The gate has no client-facing ratio query, so the ratio is computed from
/// the `file_service_cache_hits_total` / `file_service_cache_misses_total`
/// scalars. Returns null when either value is missing or there was no cache
/// traffic in the window.
double? cacheHitRatioPercent(List<MetricValue> metrics) {
  double? hits;
  double? misses;
  for (final m in metrics) {
    if (m.key == fileCacheHitsKpiKey) hits = m.value;
    if (m.key == fileCacheMissesKpiKey) misses = m.value;
  }
  if (hits == null || misses == null) return null;
  final total = hits + misses;
  if (total <= 0) return null;
  return (hits / total) * 100;
}

/// Maps analytics gate failures to short, user-facing messages.
///
/// The gate's error contract: 400 -> metric rejected by the server-side
/// allowlist, 403 -> caller's JWT carries no tenant scope, 5xx -> metrics
/// backend unreachable.
String analyticsGateMessage(Object error) {
  if (error is AnalyticsQueryException) {
    return switch (error.statusCode) {
      400 => 'This metric is not available from the analytics gate.',
      403 => 'Analytics are not available for your current sign-in scope.',
      >= 500 =>
        'The analytics backend is temporarily unavailable. '
            'Please try again shortly.',
      _ => 'Could not load analytics (HTTP ${error.statusCode}).',
    };
  }
  return 'Could not load analytics.';
}
