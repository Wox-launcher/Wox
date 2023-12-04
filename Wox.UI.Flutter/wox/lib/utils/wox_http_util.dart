import 'package:dio/dio.dart';
import 'package:wox/entity/wox_response.dart';
import 'package:wox/utils/entity_factory.dart';
import 'package:wox/utils/env.dart';

class WoxHttpUtil {
  final Dio _dio = Dio();
  final String _baseUrl = 'http://localhost:${Env.serverPort}';

  WoxHttpUtil._privateConstructor();

  static final WoxHttpUtil _instance = WoxHttpUtil._privateConstructor();

  static WoxHttpUtil get instance => _instance;

  Future<T> getData<T>(String url, {Map<String, dynamic>? params}) async {
    try {
      final response = await _dio.get(_baseUrl + url, queryParameters: params);
      WoxResponse woxResponse = WoxResponse.fromJson(response.data);
      if (woxResponse.success == false) throw Exception(woxResponse.message);
      return EntityFactory.generateOBJ<T>(woxResponse.data);
    } catch (e) {
      throw Exception('Failed to fetch data: $e');
    }
  }

  Future<T> postData<T>(String url, dynamic data) async {
    try {
      final response = await _dio.post(_baseUrl + url, data: data);
      WoxResponse woxResponse = WoxResponse.fromJson(response.data);
      if (woxResponse.success == false) throw Exception(woxResponse.message);
      return EntityFactory.generateOBJ<T>(woxResponse.data);
    } catch (e) {
      throw Exception('Failed to post data: $e');
    }
  }
}
