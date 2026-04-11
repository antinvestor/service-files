import 'package:antinvestor_api_files/antinvestor_api_files.dart';
import 'package:antinvestor_ui_core/widgets/error_helpers.dart';
import 'package:antinvestor_ui_core/widgets/form_field_card.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../providers/access_providers.dart';
import '../widgets/file_access_chip.dart';

/// Screen for managing access grants on a specific content item.
class FileAccessScreen extends ConsumerStatefulWidget {
  const FileAccessScreen({
    super.key,
    required this.contentId,
  });

  final String contentId;

  @override
  ConsumerState<FileAccessScreen> createState() => _FileAccessScreenState();
}

class _FileAccessScreenState extends ConsumerState<FileAccessScreen> {
  final _principalIdController = TextEditingController();
  PrincipalType _principalType = PrincipalType.PRINCIPAL_TYPE_USER;
  AccessRole _selectedRole = AccessRole.ACCESS_ROLE_READER;
  bool _showGrantForm = false;

  @override
  void dispose() {
    _principalIdController.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final asyncAccess = ref.watch(listAccessProvider(widget.contentId));

    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        // Header
        Padding(
          padding: const EdgeInsets.fromLTRB(24, 24, 24, 0),
          child: Row(
            children: [
              Icon(Icons.lock_outline,
                  size: 28, color: theme.colorScheme.primary),
              const SizedBox(width: 12),
              Expanded(
                child: Text(
                  'Access Control',
                  style: theme.textTheme.headlineSmall?.copyWith(
                    fontWeight: FontWeight.w600,
                    letterSpacing: -0.3,
                  ),
                ),
              ),
              FilledButton.icon(
                onPressed: () => setState(() => _showGrantForm = !_showGrantForm),
                icon: Icon(
                  _showGrantForm ? Icons.close : Icons.person_add,
                  size: 18,
                ),
                label: Text(_showGrantForm ? 'Cancel' : 'Grant Access'),
              ),
            ],
          ),
        ),

        // Grant form
        if (_showGrantForm)
          Padding(
            padding: const EdgeInsets.all(24),
            child: Card(
              child: Padding(
                padding: const EdgeInsets.all(16),
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.stretch,
                  children: [
                    Text(
                      'Grant New Access',
                      style: theme.textTheme.titleMedium?.copyWith(
                        fontWeight: FontWeight.w700,
                        color: theme.colorScheme.primary,
                      ),
                    ),
                    const SizedBox(height: 16),
                    FormFieldCard(
                      label: 'Principal ID',
                      isRequired: true,
                      description: 'User, group, or service identifier.',
                      child: TextField(
                        controller: _principalIdController,
                        decoration: const InputDecoration(
                          hintText: 'Enter principal ID',
                        ),
                      ),
                    ),
                    FormFieldCard(
                      label: 'Principal Type',
                      isRequired: true,
                      child: DropdownButtonFormField<PrincipalType>(
                        value: _principalType,
                        items: const [
                          DropdownMenuItem(
                              value: PrincipalType.PRINCIPAL_TYPE_USER,
                              child: Text('User')),
                          DropdownMenuItem(
                              value: PrincipalType.PRINCIPAL_TYPE_ORGANIZATION,
                              child: Text('Organization')),
                          DropdownMenuItem(
                              value: PrincipalType.PRINCIPAL_TYPE_SERVICE,
                              child: Text('Service')),
                          DropdownMenuItem(
                              value: PrincipalType.PRINCIPAL_TYPE_CHAT_GROUP,
                              child: Text('Chat Group')),
                        ],
                        onChanged: (v) {
                          if (v != null) setState(() => _principalType = v);
                        },
                      ),
                    ),
                    FormFieldCard(
                      label: 'Role',
                      isRequired: true,
                      child: DropdownButtonFormField<AccessRole>(
                        value: _selectedRole,
                        items: const [
                          DropdownMenuItem(
                              value: AccessRole.ACCESS_ROLE_READER,
                              child: Text('Reader')),
                          DropdownMenuItem(
                              value: AccessRole.ACCESS_ROLE_WRITER,
                              child: Text('Writer')),
                          DropdownMenuItem(
                              value: AccessRole.ACCESS_ROLE_OWNER,
                              child: Text('Owner')),
                        ],
                        onChanged: (v) {
                          if (v != null) setState(() => _selectedRole = v);
                        },
                      ),
                    ),
                    Row(
                      mainAxisAlignment: MainAxisAlignment.end,
                      children: [
                        TextButton(
                          onPressed: () =>
                              setState(() => _showGrantForm = false),
                          child: const Text('Cancel'),
                        ),
                        const SizedBox(width: 12),
                        FilledButton(
                          onPressed: _grantAccess,
                          child: const Text('Grant'),
                        ),
                      ],
                    ),
                  ],
                ),
              ),
            ),
          ),

        // Access list
        Expanded(
          child: asyncAccess.when(
            loading: () => const Center(child: CircularProgressIndicator()),
            error: (error, _) => Center(
              child: Column(
                mainAxisSize: MainAxisSize.min,
                children: [
                  Icon(Icons.error_outline,
                      size: 48, color: theme.colorScheme.error),
                  const SizedBox(height: 16),
                  Text(friendlyError(error)),
                  const SizedBox(height: 16),
                  FilledButton.tonal(
                    onPressed: () => ref
                        .invalidate(listAccessProvider(widget.contentId)),
                    child: const Text('Retry'),
                  ),
                ],
              ),
            ),
            data: (grants) {
              if (grants.isEmpty) {
                return Center(
                  child: Column(
                    mainAxisSize: MainAxisSize.min,
                    children: [
                      Icon(Icons.people_outline,
                          size: 48,
                          color: theme.colorScheme.onSurfaceVariant
                              .withAlpha(120)),
                      const SizedBox(height: 12),
                      Text(
                        'No access grants yet',
                        style: theme.textTheme.bodyLarge?.copyWith(
                          color: theme.colorScheme.onSurfaceVariant,
                        ),
                      ),
                    ],
                  ),
                );
              }

              return ListView.builder(
                padding: const EdgeInsets.symmetric(horizontal: 24, vertical: 8),
                itemCount: grants.length,
                itemBuilder: (context, index) {
                  final grant = grants[index];
                  return Padding(
                    padding: const EdgeInsets.only(bottom: 8),
                    child: FileAccessChip(
                      grant: grant,
                      onRevoke: () => _revokeAccess(grant),
                    ),
                  );
                },
              );
            },
          ),
        ),
      ],
    );
  }

  Future<void> _grantAccess() async {
    final principalId = _principalIdController.text.trim();
    if (principalId.isEmpty) {
      ScaffoldMessenger.of(context).showSnackBar(
        const SnackBar(content: Text('Principal ID is required')),
      );
      return;
    }

    try {
      await ref.read(accessNotifierProvider.notifier).grantAccess(
            mediaId: widget.contentId,
            principalId: principalId,
            principalType: _principalType,
            role: _selectedRole,
          );
      if (mounted) {
        _principalIdController.clear();
        setState(() => _showGrantForm = false);
        ref.invalidate(listAccessProvider(widget.contentId));
        ScaffoldMessenger.of(context).showSnackBar(
          const SnackBar(content: Text('Access granted')),
        );
      }
    } catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: Text(friendlyError(e))),
        );
      }
    }
  }

  Future<void> _revokeAccess(AccessGrant grant) async {
    final confirmed = await showDialog<bool>(
      context: context,
      builder: (context) => AlertDialog(
        title: const Text('Revoke Access'),
        content: Text(
            'Revoke access for ${grant.principalId} (${grant.principalType.name})?'),
        actions: [
          TextButton(
            onPressed: () => Navigator.pop(context, false),
            child: const Text('Cancel'),
          ),
          FilledButton(
            style: FilledButton.styleFrom(backgroundColor: Colors.red),
            onPressed: () => Navigator.pop(context, true),
            child: const Text('Revoke'),
          ),
        ],
      ),
    );

    if (confirmed == true && mounted) {
      try {
        await ref.read(accessNotifierProvider.notifier).revokeAccess(
              mediaId: widget.contentId,
              principalId: grant.principalId,
            );
        if (mounted) {
          ref.invalidate(listAccessProvider(widget.contentId));
          ScaffoldMessenger.of(context).showSnackBar(
            const SnackBar(content: Text('Access revoked')),
          );
        }
      } catch (e) {
        if (mounted) {
          ScaffoldMessenger.of(context).showSnackBar(
            SnackBar(content: Text(friendlyError(e))),
          );
        }
      }
    }
  }
}
