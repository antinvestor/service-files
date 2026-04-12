import 'package:antinvestor_api_files/antinvestor_api_files.dart';
import 'package:antinvestor_ui_core/widgets/error_helpers.dart';
import 'package:antinvestor_ui_core/widgets/form_field_card.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../providers/retention_providers.dart';

/// Screen for listing and managing retention policies.
class FileRetentionScreen extends ConsumerStatefulWidget {
  const FileRetentionScreen({super.key});

  @override
  ConsumerState<FileRetentionScreen> createState() =>
      _FileRetentionScreenState();
}

class _FileRetentionScreenState extends ConsumerState<FileRetentionScreen> {
  bool _showForm = false;
  final _mediaIdController = TextEditingController();
  final _policyIdController = TextEditingController();

  @override
  void dispose() {
    _mediaIdController.dispose();
    _policyIdController.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final asyncPolicies = ref.watch(listRetentionPoliciesProvider);

    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        // Header
        Padding(
          padding: const EdgeInsets.fromLTRB(24, 24, 24, 0),
          child: Row(
            children: [
              Icon(Icons.schedule, size: 28, color: theme.colorScheme.primary),
              const SizedBox(width: 12),
              Expanded(
                child: Text(
                  'Retention Policies',
                  style: theme.textTheme.headlineSmall?.copyWith(
                    fontWeight: FontWeight.w600,
                    letterSpacing: -0.3,
                  ),
                ),
              ),
              FilledButton.icon(
                onPressed: () => setState(() => _showForm = !_showForm),
                icon: Icon(
                  _showForm ? Icons.close : Icons.add,
                  size: 18,
                ),
                label: Text(_showForm ? 'Cancel' : 'Apply Policy'),
              ),
            ],
          ),
        ),

        // Create/edit form
        if (_showForm)
          Padding(
            padding: const EdgeInsets.all(24),
            child: Card(
              child: Padding(
                padding: const EdgeInsets.all(16),
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.stretch,
                  children: [
                    Text(
                      'Apply Retention Policy',
                      style: theme.textTheme.titleMedium?.copyWith(
                        fontWeight: FontWeight.w700,
                        color: theme.colorScheme.primary,
                      ),
                    ),
                    const SizedBox(height: 16),
                    FormFieldCard(
                      label: 'Media ID',
                      isRequired: true,
                      description: 'The content item to apply the policy to.',
                      child: TextField(
                        controller: _mediaIdController,
                        decoration: const InputDecoration(
                          hintText: 'Enter media ID',
                        ),
                      ),
                    ),
                    FormFieldCard(
                      label: 'Policy ID',
                      isRequired: true,
                      description: 'The retention policy ID to apply.',
                      child: DropdownButtonFormField<String>(
                        initialValue: _policyIdController.text.isEmpty
                            ? null
                            : _policyIdController.text,
                        items: asyncPolicies.whenOrNull(
                          data: (policies) => policies
                              .map((p) => DropdownMenuItem(
                                    value: p.policyId,
                                    child: Text(
                                        '${p.name} (${p.retentionDays}d)'),
                                  ))
                              .toList(),
                        ),
                        onChanged: (v) {
                          if (v != null) {
                            _policyIdController.text = v;
                          }
                        },
                      ),
                    ),
                    Row(
                      mainAxisAlignment: MainAxisAlignment.end,
                      children: [
                        TextButton(
                          onPressed: () => setState(() => _showForm = false),
                          child: const Text('Cancel'),
                        ),
                        const SizedBox(width: 12),
                        FilledButton(
                          onPressed: _applyPolicy,
                          child: const Text('Apply'),
                        ),
                      ],
                    ),
                  ],
                ),
              ),
            ),
          ),

        // Policies list
        Expanded(
          child: asyncPolicies.when(
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
                    onPressed: () =>
                        ref.invalidate(listRetentionPoliciesProvider),
                    child: const Text('Retry'),
                  ),
                ],
              ),
            ),
            data: (policies) {
              if (policies.isEmpty) {
                return Center(
                  child: Column(
                    mainAxisSize: MainAxisSize.min,
                    children: [
                      Icon(Icons.schedule,
                          size: 48,
                          color: theme.colorScheme.onSurfaceVariant
                              .withAlpha(120)),
                      const SizedBox(height: 12),
                      Text(
                        'No retention policies configured',
                        style: theme.textTheme.bodyLarge?.copyWith(
                          color: theme.colorScheme.onSurfaceVariant,
                        ),
                      ),
                    ],
                  ),
                );
              }

              return ListView.separated(
                padding:
                    const EdgeInsets.symmetric(horizontal: 24, vertical: 8),
                itemCount: policies.length,
                separatorBuilder: (_, _) => const SizedBox(height: 8),
                itemBuilder: (context, index) {
                  final policy = policies[index];
                  return _RetentionPolicyCard(policy: policy);
                },
              );
            },
          ),
        ),
      ],
    );
  }

  Future<void> _applyPolicy() async {
    final mediaId = _mediaIdController.text.trim();
    final policyId = _policyIdController.text.trim();
    if (mediaId.isEmpty || policyId.isEmpty) {
      ScaffoldMessenger.of(context).showSnackBar(
        const SnackBar(content: Text('All fields are required')),
      );
      return;
    }

    try {
      await ref.read(retentionNotifierProvider.notifier).setRetentionPolicy(
            mediaId: mediaId,
            policyId: policyId,
          );
      if (mounted) {
        _mediaIdController.clear();
        _policyIdController.clear();
        setState(() => _showForm = false);
        ref.invalidate(listRetentionPoliciesProvider);
        ScaffoldMessenger.of(context).showSnackBar(
          const SnackBar(content: Text('Retention policy applied')),
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

class _RetentionPolicyCard extends StatelessWidget {
  const _RetentionPolicyCard({required this.policy});

  final RetentionPolicy policy;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final isDelete = policy.mode == RetentionPolicy_Mode.MODE_DELETE;

    return Card(
      child: Padding(
        padding: const EdgeInsets.all(16),
        child: Row(
          children: [
            Container(
              width: 40,
              height: 40,
              decoration: BoxDecoration(
                color: (isDelete ? Colors.red : Colors.blue).withAlpha(25),
                borderRadius: BorderRadius.circular(10),
              ),
              child: Icon(
                isDelete ? Icons.delete_outline : Icons.archive_outlined,
                color: isDelete ? Colors.red : Colors.blue,
                size: 20,
              ),
            ),
            const SizedBox(width: 16),
            Expanded(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text(
                    policy.name.isNotEmpty ? policy.name : policy.policyId,
                    style: theme.textTheme.titleSmall?.copyWith(
                      fontWeight: FontWeight.w600,
                    ),
                    maxLines: 1,
                    overflow: TextOverflow.ellipsis,
                  ),
                  const SizedBox(height: 4),
                  Row(
                    children: [
                      Container(
                        padding: const EdgeInsets.symmetric(
                            horizontal: 8, vertical: 2),
                        decoration: BoxDecoration(
                          color: (isDelete ? Colors.red : Colors.blue)
                              .withAlpha(25),
                          borderRadius: BorderRadius.circular(4),
                        ),
                        child: Text(
                          isDelete ? 'Delete' : 'Archive',
                          style: theme.textTheme.labelSmall?.copyWith(
                            color: isDelete ? Colors.red : Colors.blue,
                            fontWeight: FontWeight.w600,
                          ),
                        ),
                      ),
                      const SizedBox(width: 8),
                      Text(
                        'Retention: ${policy.retentionDays} days',
                        style: theme.textTheme.bodySmall?.copyWith(
                          color: theme.colorScheme.onSurfaceVariant,
                        ),
                      ),
                    ],
                  ),
                ],
              ),
            ),
          ],
        ),
      ),
    );
  }
}
