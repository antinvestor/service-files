import 'dart:convert';

import 'package:antinvestor_api_files/antinvestor_api_files.dart';
import 'package:antinvestor_ui_core/antinvestor_ui_core.dart';
import 'package:antinvestor_ui_files/antinvestor_ui_files.dart';
import 'package:fixnum/fixnum.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;

import '../_helpers/fake_analytics_transport.dart';

void main() {
  late FakeAnalyticsTransport transport;

  setUp(() {
    transport = FakeAnalyticsTransport();
  });

  GetStorageStatsResponse storageStats() => GetStorageStatsResponse()
    ..totalFiles = Int64(321)
    ..totalBytes = Int64(5 * 1024 * 1024)
    ..totalUsers = Int64(12);

  Future<void> pumpDashboard(
    WidgetTester tester, {
    Future<GetStorageStatsResponse> Function()? stats,
  }) async {
    tester.view.physicalSize = const Size(1600, 2400);
    tester.view.devicePixelRatio = 1.0;
    addTearDown(tester.view.reset);
    await tester.pumpWidget(
      ProviderScope(
        // Disable Riverpod's automatic retry so failed gate queries settle
        // in their error state instead of flipping back to loading.
        retry: (retryCount, error) => null,
        overrides: [
          getStorageStatsProvider.overrideWith(
            (ref) => stats != null ? stats() : Future.value(storageStats()),
          ),
          analyticsDataSourceProvider.overrideWithValue(
            ThesaAnalyticsDataSource(
              transport.call,
              specs: const [filesAnalyticsSpec],
            ),
          ),
        ],
        child: const MaterialApp(
          home: Scaffold(body: StorageDashboardScreen()),
        ),
      ),
    );
    await tester.pumpAndSettle();
  }

  testWidgets('keeps inventory totals from the entity API', (tester) async {
    await pumpDashboard(tester);

    expect(find.text('Total Files'), findsOneWidget);
    expect(find.text('321'), findsOneWidget);
    expect(find.text('Total Users'), findsOneWidget);
    expect(find.text('12'), findsOneWidget);
  });

  testWidgets('renders gate-backed activity KPIs with derived cache ratio', (
    tester,
  ) async {
    transport.handler = (path, body) {
      if (!path.endsWith('/scalar')) {
        return http.Response(json.encode({'points': <Object>[]}), 200);
      }
      final value = switch (body['metric']) {
        'file_service_uploads_total' => 42,
        'file_service_downloads_total' => 87,
        'file_service_upload_bytes_total' => 2048,
        'file_service_download_bytes_total' => 4096,
        'file_service_cache_hits_total' => 75,
        'file_service_cache_misses_total' => 25,
        _ => 0,
      };
      return http.Response(json.encode({'value': value}), 200);
    };

    await pumpDashboard(tester);

    expect(find.text('Uploads'), findsAtLeastNWidgets(1));
    expect(find.text('42'), findsOneWidget);
    expect(find.text('Downloads'), findsAtLeastNWidgets(1));
    expect(find.text('87'), findsOneWidget);
    expect(find.text('Data uploaded'), findsAtLeastNWidgets(1));
    expect(find.text('2.0 KB'), findsOneWidget);
    expect(find.text('Data downloaded'), findsAtLeastNWidgets(1));
    expect(find.text('4.1 KB'), findsOneWidget);
    expect(find.text('Cache hit ratio'), findsOneWidget);
    expect(find.text('75.0%'), findsOneWidget);
    // Raw hit/miss scalars are folded into the ratio card.
    expect(find.text('Cache hits'), findsNothing);
    expect(find.text('Cache misses'), findsNothing);
  });

  testWidgets('omits the cache ratio card without cache traffic', (
    tester,
  ) async {
    await pumpDashboard(tester);

    expect(find.text('Cache hit ratio'), findsNothing);
  });

  testWidgets('shows empty chart states when the gate has no data', (
    tester,
  ) async {
    await pumpDashboard(tester);

    // Transfer activity and transfer volume both report no data.
    expect(find.text('No data'), findsNWidgets(2));
  });

  for (final (status, fragment) in [
    (400, 'not available from the analytics gate'),
    (403, 'not available for your current sign-in scope'),
    (503, 'temporarily unavailable'),
  ]) {
    testWidgets('renders friendly state for gate HTTP $status', (tester) async {
      transport.handler = (path, body) =>
          http.Response(json.encode({'error': 'gate says no'}), status);

      await pumpDashboard(tester);

      // KPI row plus both charts surface the same friendly message, while
      // entity-backed inventory still renders.
      expect(find.textContaining(fragment), findsNWidgets(3));
      expect(find.textContaining('gate says no'), findsNothing);
      expect(find.text('Total Files'), findsOneWidget);
    });
  }

  testWidgets('inventory errors stay independent of the gate', (tester) async {
    await pumpDashboard(
      tester,
      stats: () => Future.error(Exception('stats backend down')),
    );

    expect(find.text('Retry'), findsAtLeastNWidgets(1));
    // Gate-backed activity still renders its empty charts.
    expect(find.text('No data'), findsNWidgets(2));
  });
}
