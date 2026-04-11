import 'package:antinvestor_api_files/antinvestor_api_files.dart';
import 'package:antinvestor_ui_core/widgets/status_badge.dart';
import 'package:flutter/material.dart';

/// Displays a colored badge for AccessRole values.
class AccessRoleBadge extends StatelessWidget {
  const AccessRoleBadge({super.key, required this.role});

  final AccessRole role;

  @override
  Widget build(BuildContext context) {
    return StatusBadge.fromEnum(
      value: role,
      mapper: (r) => switch (r) {
        AccessRole.READER => ('Reader', Colors.blue, Icons.visibility),
        AccessRole.WRITER => ('Writer', Colors.amber, Icons.edit),
        AccessRole.OWNER => ('Owner', Colors.green, Icons.admin_panel_settings),
        _ => ('Unknown', Colors.grey, null),
      },
    );
  }
}
