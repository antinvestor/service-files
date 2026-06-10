import 'dart:convert';

import 'package:antinvestor_ui_core/antinvestor_ui_core.dart';
import 'package:antinvestor_ui_files/antinvestor_ui_files.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;

import '../_helpers/fake_analytics_transport.dart';

void main() {
  final timeRange = AnalyticsTimeRange(
    start: DateTime.utc(2026, 1, 1),
    end: DateTime.utc(2026, 1, 31),
    granularity: TimeGranularity.day,
  );
  const wireTimeRange = {
    'start': '2026-01-01T00:00:00.000Z',
    'end': '2026-01-31T00:00:00.000Z',
  };

  late FakeAnalyticsTransport transport;
  late ThesaAnalyticsDataSource dataSource;

  setUp(() {
    transport = FakeAnalyticsTransport();
    dataSource = ThesaAnalyticsDataSource(
      transport.call,
      specs: const [filesAnalyticsSpec],
    );
  });

  test('KPI fetch posts one exact scalar body per declared metric', () async {
    await dataSource.getMetrics('files', timeRange: timeRange);

    expect(
      transport.calls.map((c) => c.path),
      everyElement('/api/analytics/query/scalar'),
    );
    expect(transport.calls.map((c) => c.body), [
      for (final metric in [
        'file_service_uploads_total',
        'file_service_downloads_total',
        'file_service_upload_bytes_total',
        'file_service_download_bytes_total',
        'file_service_cache_hits_total',
        'file_service_cache_misses_total',
      ])
        {'metric': metric, 'aggregation': 'sum', 'time_range': wireTimeRange},
    ]);
  });

  test('activity trends post exact timeseries bodies with step', () async {
    for (final metric in [
      fileUploadsMetric,
      fileDownloadsMetric,
      fileUploadBytesMetric,
      fileDownloadBytesMetric,
    ]) {
      transport.calls.clear();
      await dataSource.getTimeSeries('files', metric, timeRange: timeRange);

      expect(transport.calls, hasLength(1));
      expect(transport.calls.single.path, '/api/analytics/query/timeseries');
      expect(transport.calls.single.body, {
        'metric': metric,
        'aggregation': 'sum',
        'time_range': wireTimeRange,
        'step': 'day',
      });
    }
  });

  test('no request ever carries tenant or partition filters', () async {
    await dataSource.getMetrics('files', timeRange: timeRange);
    await dataSource.getTimeSeries(
      'files',
      fileUploadsMetric,
      timeRange: timeRange,
    );

    for (final call in transport.calls) {
      final filters =
          (call.body['filters'] as Map<String, dynamic>?) ?? const {};
      expect(filters.keys, isNot(contains('tenant_id')));
      expect(filters.keys, isNot(contains('partition_id')));
    }
  });

  test('gate errors surface status code and server message', () async {
    transport.handler = (path, body) =>
        http.Response(json.encode({'error': 'no tenant scope'}), 403);

    await expectLater(
      dataSource.getMetrics('files', timeRange: timeRange),
      throwsA(
        isA<AnalyticsQueryException>()
            .having((e) => e.statusCode, 'statusCode', 403)
            .having((e) => e.message, 'message', 'no tenant scope'),
      ),
    );
  });

  group('cacheHitRatioPercent', () {
    MetricValue metric(String key, double value) =>
        MetricValue(key: key, label: key, value: value);

    test('derives the ratio from hits and misses', () {
      expect(
        cacheHitRatioPercent([
          metric('cache_hits', 75),
          metric('cache_misses', 25),
        ]),
        75.0,
      );
    });

    test('returns null without cache traffic', () {
      expect(
        cacheHitRatioPercent([
          metric('cache_hits', 0),
          metric('cache_misses', 0),
        ]),
        isNull,
      );
    });

    test('returns null when either scalar is missing', () {
      expect(cacheHitRatioPercent([metric('cache_hits', 10)]), isNull);
      expect(cacheHitRatioPercent(const []), isNull);
    });
  });

  test('analyticsGateMessage maps gate statuses to friendly text', () {
    const path = '/api/analytics/query/scalar';
    expect(
      analyticsGateMessage(
        const AnalyticsQueryException(
          statusCode: 400,
          message: 'metric not allowed',
          path: path,
        ),
      ),
      'This metric is not available from the analytics gate.',
    );
    expect(
      analyticsGateMessage(
        const AnalyticsQueryException(
          statusCode: 403,
          message: 'no tenant scope',
          path: path,
        ),
      ),
      'Analytics are not available for your current sign-in scope.',
    );
    expect(
      analyticsGateMessage(
        const AnalyticsQueryException(
          statusCode: 503,
          message: 'backend down',
          path: path,
        ),
      ),
      contains('temporarily unavailable'),
    );
    expect(
      analyticsGateMessage(StateError('boom')),
      'Could not load analytics.',
    );
  });
}
