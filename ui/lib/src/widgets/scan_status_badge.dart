import 'package:antinvestor_api_files/antinvestor_api_files.dart';
import 'package:antinvestor_ui_core/widgets/status_badge.dart';
import 'package:flutter/material.dart';

/// Displays a colored badge for ScanStatus values.
class ScanStatusBadge extends StatelessWidget {
  const ScanStatusBadge({super.key, required this.status});

  final ScanStatus status;

  @override
  Widget build(BuildContext context) {
    return StatusBadge.fromEnum(
      value: status,
      mapper: (s) => switch (s) {
        ScanStatus.SCAN_STATUS_PENDING => ('Pending', Colors.amber, Icons.hourglass_empty),
        ScanStatus.SCAN_STATUS_CLEAN => ('Clean', Colors.green, Icons.check_circle),
        ScanStatus.SCAN_STATUS_INFECTED =>
          ('Infected', Colors.red, Icons.warning_amber_rounded),
        ScanStatus.SCAN_STATUS_FAILED => ('Failed', Colors.orange, Icons.error_outline),
        _ => ('Unknown', Colors.grey, null),
      },
    );
  }
}
