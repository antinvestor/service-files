import 'package:antinvestor_api_files/antinvestor_api_files.dart';
import 'package:flutter/material.dart';

import 'access_role_badge.dart';

/// Displays an access grant as a chip with principal type icon, name, and role badge.
class FileAccessChip extends StatelessWidget {
  const FileAccessChip({
    super.key,
    required this.grant,
    this.onRevoke,
  });

  final AccessGrant grant;
  final VoidCallback? onRevoke;

  IconData _principalTypeIcon(PrincipalType type) {
    return switch (type) {
      PrincipalType.PRINCIPAL_TYPE_USER => Icons.person,
      PrincipalType.PRINCIPAL_TYPE_ORGANIZATION => Icons.group,
      PrincipalType.PRINCIPAL_TYPE_SERVICE => Icons.settings,
      PrincipalType.PRINCIPAL_TYPE_CHAT_GROUP => Icons.people_alt,
      _ => Icons.account_circle,
    };
  }

  String _principalTypeLabel(PrincipalType type) {
    return switch (type) {
      PrincipalType.PRINCIPAL_TYPE_USER => 'User',
      PrincipalType.PRINCIPAL_TYPE_ORGANIZATION => 'Organization',
      PrincipalType.PRINCIPAL_TYPE_SERVICE => 'Service',
      PrincipalType.PRINCIPAL_TYPE_CHAT_GROUP => 'Chat Group',
      _ => type.name,
    };
  }

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);

    return Card(
      child: Padding(
        padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 10),
        child: Row(
          children: [
            Icon(
              _principalTypeIcon(grant.principalType),
              size: 20,
              color: theme.colorScheme.onSurfaceVariant,
            ),
            const SizedBox(width: 8),
            Expanded(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text(
                    grant.principalId,
                    style: theme.textTheme.bodyMedium?.copyWith(
                      fontWeight: FontWeight.w600,
                    ),
                  ),
                  Text(
                    _principalTypeLabel(grant.principalType),
                    style: theme.textTheme.bodySmall?.copyWith(
                      color: theme.colorScheme.onSurfaceVariant,
                    ),
                  ),
                ],
              ),
            ),
            AccessRoleBadge(role: grant.role),
            if (onRevoke != null) ...[
              const SizedBox(width: 8),
              IconButton(
                icon: Icon(
                  Icons.remove_circle_outline,
                  size: 20,
                  color: theme.colorScheme.error,
                ),
                onPressed: onRevoke,
                tooltip: 'Revoke access',
                visualDensity: VisualDensity.compact,
              ),
            ],
          ],
        ),
      ),
    );
  }
}
