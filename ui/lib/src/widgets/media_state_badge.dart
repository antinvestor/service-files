import 'package:antinvestor_api_files/antinvestor_api_files.dart';
import 'package:antinvestor_ui_core/widgets/status_badge.dart';
import 'package:flutter/material.dart';

/// Displays a colored badge for MediaState values.
class MediaStateBadge extends StatelessWidget {
  const MediaStateBadge({super.key, required this.state});

  final MediaState state;

  @override
  Widget build(BuildContext context) {
    return StatusBadge.fromEnum(
      value: state,
      mapper: (s) => switch (s) {
        MediaState.MEDIA_STATE_CREATING => ('Creating', Colors.blue, null),
        MediaState.MEDIA_STATE_AVAILABLE => ('Available', Colors.green, null),
        MediaState.MEDIA_STATE_ARCHIVED => ('Archived', Colors.grey, null),
        MediaState.MEDIA_STATE_DELETED => ('Deleted', Colors.red, null),
        MediaState.MEDIA_STATE_FAILED => ('Failed', Colors.orange, null),
        _ => ('Unknown', Colors.grey, null),
      },
    );
  }
}
