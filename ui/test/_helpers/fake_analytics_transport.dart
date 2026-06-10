import 'dart:convert';

import 'package:http/http.dart' as http;

/// Recording fake for the [AnalyticsTransport] used by
/// `ThesaAnalyticsDataSource` in tests.
///
/// Captures every POSTed path + decoded JSON body and replies either via
/// [handler] or with an empty success payload appropriate for the endpoint.
class FakeAnalyticsTransport {
  /// Every request made through this transport, in order.
  final List<({String path, Map<String, dynamic> body})> calls = [];

  /// Optional per-request response factory. When null, the transport
  /// returns zero/empty payloads with HTTP 200.
  http.Response Function(String path, Map<String, dynamic> body)? handler;

  Future<http.Response> call(String path, {Object? body}) async {
    final decoded = json.decode(body! as String) as Map<String, dynamic>;
    calls.add((path: path, body: decoded));
    final h = handler;
    if (h != null) return h(path, decoded);
    return http.Response(json.encode(_emptyPayloadFor(path)), 200);
  }

  static Map<String, dynamic> _emptyPayloadFor(String path) {
    if (path.endsWith('/scalar')) return {'value': 0};
    if (path.endsWith('/timeseries')) return {'points': <Object>[]};
    if (path.endsWith('/grouped')) return {'segments': <Object>[]};
    return {'items': <Object>[]};
  }
}
