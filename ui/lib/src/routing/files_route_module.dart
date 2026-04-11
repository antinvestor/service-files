import 'package:antinvestor_api_files/antinvestor_api_files.dart';
import 'package:antinvestor_ui_core/navigation/nav_items.dart';
import 'package:antinvestor_ui_core/permissions/permission_manifest.dart';
import 'package:antinvestor_ui_core/routing/route_module.dart';
import 'package:flutter/material.dart';
import 'package:go_router/go_router.dart';

import '../screens/file_access_screen.dart';
import '../screens/file_detail_screen.dart';
import '../screens/file_retention_screen.dart';
import '../screens/file_upload_screen.dart';
import '../screens/files_browser_screen.dart';
import '../screens/storage_dashboard_screen.dart';

/// Route module for file management.
///
/// Registers the following routes:
/// - `/files` - file browser
/// - `/files/upload` - upload new file
/// - `/files/storage` - storage dashboard
/// - `/files/retention` - retention policies
/// - `/files/:contentId` - file detail view
/// - `/files/:contentId/access` - access management
class FilesRouteModule extends RouteModule {
  @override
  String get moduleId => 'files';

  @override
  List<RouteBase> buildRoutes() {
    return [
      GoRoute(
        path: '/files',
        builder: (context, state) => const FilesBrowserScreen(),
        routes: [
          GoRoute(
            path: 'upload',
            builder: (context, state) => const FileUploadScreen(),
          ),
          GoRoute(
            path: 'storage',
            builder: (context, state) => const StorageDashboardScreen(),
          ),
          GoRoute(
            path: 'retention',
            builder: (context, state) => const FileRetentionScreen(),
          ),
          GoRoute(
            path: ':contentId',
            builder: (context, state) {
              final id = state.pathParameters['contentId'] ?? '';
              final extra = state.extra;
              final media = extra is MediaMetadata ? extra : null;
              return FileDetailScreen(
                contentId: id,
                initialMedia: media,
              );
            },
            routes: [
              GoRoute(
                path: 'access',
                builder: (context, state) {
                  final id = state.pathParameters['contentId'] ?? '';
                  return FileAccessScreen(contentId: id);
                },
              ),
            ],
          ),
        ],
      ),
    ];
  }

  @override
  List<NavItem> buildNavItems() {
    return [
      const NavItem(
        id: 'files',
        label: 'Files',
        icon: Icons.folder_outlined,
        activeIcon: Icons.folder,
        route: '/files',
        requiredPermissions: {'file_view'},
        children: [
          NavItem(
            id: 'files-browser',
            label: 'Browse',
            icon: Icons.grid_view,
            route: '/files',
            requiredPermissions: {'file_view'},
          ),
          NavItem(
            id: 'files-upload',
            label: 'Upload',
            icon: Icons.upload_file,
            route: '/files/upload',
            requiredPermissions: {'file_upload'},
          ),
          NavItem(
            id: 'files-storage',
            label: 'Storage',
            icon: Icons.storage,
            route: '/files/storage',
            requiredPermissions: {'file_view'},
          ),
          NavItem(
            id: 'files-retention',
            label: 'Retention',
            icon: Icons.schedule,
            route: '/files/retention',
            requiredPermissions: {'file_retention_manage'},
          ),
        ],
      ),
    ];
  }

  @override
  Map<String, Set<String>> get routePermissions => {
        '/files': {'file_view'},
        '/files/upload': {'file_upload'},
        '/files/storage': {'file_view'},
        '/files/retention': {'file_retention_manage'},
        '/files/:contentId': {'file_view'},
        '/files/:contentId/access': {'file_access_manage'},
      };

  @override
  PermissionManifest get permissionManifest => const PermissionManifest(
        namespace: 'service_files',
        permissions: [
          PermissionEntry(
            key: 'file_view',
            label: 'View Files',
            scope: PermissionScope.service,
          ),
          PermissionEntry(
            key: 'file_upload',
            label: 'Upload Files',
            scope: PermissionScope.action,
          ),
          PermissionEntry(
            key: 'file_delete',
            label: 'Delete Files',
            scope: PermissionScope.action,
          ),
          PermissionEntry(
            key: 'file_access_manage',
            label: 'Manage File Access',
            scope: PermissionScope.feature,
          ),
          PermissionEntry(
            key: 'file_version_manage',
            label: 'Manage File Versions',
            scope: PermissionScope.feature,
          ),
          PermissionEntry(
            key: 'file_retention_manage',
            label: 'Manage Retention Policies',
            scope: PermissionScope.feature,
          ),
        ],
      );
}
