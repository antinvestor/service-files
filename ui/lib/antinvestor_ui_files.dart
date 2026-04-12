/// File management UI library for Antinvestor.
///
/// Provides embeddable screens, widgets, and Riverpod providers for uploading,
/// browsing, access control, versioning, and retention management.
library;

// Providers
export 'src/providers/files_transport_provider.dart';
export 'src/providers/files_providers.dart';
export 'src/providers/upload_providers.dart';
export 'src/providers/access_providers.dart';
export 'src/providers/version_providers.dart';
export 'src/providers/retention_providers.dart';

// Widgets
export 'src/widgets/file_preview_card.dart';
export 'src/widgets/media_state_badge.dart';
export 'src/widgets/scan_status_badge.dart';
export 'src/widgets/upload_progress_indicator.dart';
export 'src/widgets/file_access_chip.dart';
export 'src/widgets/storage_usage_bar.dart';
export 'src/widgets/version_history_timeline.dart';
export 'src/widgets/access_role_badge.dart';
export 'src/widgets/file_thumbnail_widget.dart';
export 'src/widgets/file_attachment_chip.dart';

// Screens
export 'src/screens/files_browser_screen.dart';
export 'src/screens/file_detail_screen.dart';
export 'src/screens/file_upload_screen.dart';
export 'src/screens/file_access_screen.dart';
export 'src/screens/file_retention_screen.dart';
export 'src/screens/storage_dashboard_screen.dart';

// Routing
export 'src/routing/files_route_module.dart';
