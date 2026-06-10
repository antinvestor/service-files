## 0.3.0

- `StorageDashboardScreen` keeps inventory totals on the `GetStorageStats`
  entity API and now adds a gate-backed Activity section: upload/download
  KPIs and trends, transferred bytes, and a derived cache hit ratio sourced
  from the thesa analytics gate via `antinvestor_ui_core`'s
  `analyticsDataSourceProvider` (`file_service_*` business metrics).
- Added `filesAnalyticsSpec` (a `ServiceAnalyticsSpec`) for host apps to
  register on their `ThesaAnalyticsDataSource`, `cacheHitRatioPercent` for
  the hits/misses ratio, and `analyticsGateMessage` for friendly gate error
  states (400 allowlist, 403 unscoped, 5xx backend down).
- Requires `antinvestor_ui_core` >= 0.5.0 (unpublished; use a local path
  override during development).

## 0.1.1

- Migrate providers to Riverpod 3.x Notifier, fix lint warnings and deprecations

## 0.1.0

- Initial release
- File management UI with browser, upload, access control, versioning, retention
