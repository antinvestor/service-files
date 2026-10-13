/// Dart client library for Ant Investor Files Service.
///
/// Provides Files service functionality using Connect RPC protocol.
library;

// Files service
export 'src/files/v1/files.pb.dart';
export 'src/files/v1/files.pbenum.dart';
export 'src/files/v1/files.pbjson.dart';
export 'src/files/v1/files.connect.client.dart';
export 'src/files/v1/files.connect.spec.dart';

// Common types
export 'src/common/v1/common.pb.dart';
export 'src/common/v1/common.pbenum.dart';
export 'package:protobuf/well_known_types/google/protobuf/struct.pb.dart';
export 'package:protobuf/well_known_types/google/protobuf/timestamp.pb.dart';
