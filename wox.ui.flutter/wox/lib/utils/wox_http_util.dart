import 'package:dio/dio.dart';
import 'package:wox/entity/wox_response.dart';
import 'package:wox/utils/entity_factory.dart';
import 'package:wox/utils/env.dart';
import 'package:wox/utils/log.dart';

class WoxHttpUtil {
  final Dio _dio = Dio();
  final String _baseUrl = 'http://127.0.0.1:${Env.serverPort}';

  WoxHttpUtil._privateConstructor();

  static final WoxHttpUtil _instance = WoxHttpUtil._privateConstructor();

  static WoxHttpUtil get instance => _instance;

  Future<T> getData<T>(String traceId, String url, {Map<String, dynamic>? params}) async {
    try {
      final response = await _dio.get(_baseUrl + url, queryParameters: params, options: Options(headers: {"TraceId": traceId, "SessionId": Env.sessionId}));
      WoxResponse woxResponse = WoxResponse.fromJson(response.data);
      if (woxResponse.success == false) throw Exception(woxResponse.message);
      return EntityFactory.generateOBJ<T>(woxResponse.data);
    } catch (e) {
      Logger.instance.error(traceId, 'Failed to fetch data: $e');
      rethrow;
    }
  }

  Future<T> postData<T>(String traceId, String url, dynamic data) async {
    try {
      Logger.instance.info(traceId, 'Posting data to $_baseUrl$url');
      final response = await _dio.post(_baseUrl + url, data: data, options: Options(headers: {"TraceId": traceId, "SessionId": Env.sessionId}));
      WoxResponse woxResponse = WoxResponse.fromJson(response.data);
      if (woxResponse.success == false) throw Exception(woxResponse.message);
      return EntityFactory.generateOBJ<T>(woxResponse.data);
    } catch (e) {
      Logger.instance.error(traceId, 'Failed to post data: $e');
      rethrow;
    }
  }
}
