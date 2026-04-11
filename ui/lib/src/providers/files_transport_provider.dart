import 'package:antinvestor_api_files/antinvestor_api_files.dart';
import 'package:antinvestor_ui_core/api/api_base.dart';
import 'package:connectrpc/connect.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

const _filesUrl = String.fromEnvironment(
  'FILES_URL',
  defaultValue: 'https://api.antinvestor.com/files',
);

final filesTransportProvider = Provider<Transport>((ref) {
  final tokenProvider = ref.watch(authTokenProviderProvider);
  return createTransport(tokenProvider, baseUrl: _filesUrl);
});

final filesServiceClientProvider = Provider<FilesServiceClient>((ref) {
  final transport = ref.watch(filesTransportProvider);
  return FilesServiceClient(transport);
});
