import 'package:flutter_test/flutter_test.dart';
import 'package:wox/utils/wox_fuzzy_match_util.dart';

void main() {
  test('pinyin first-letters fuzzy: "bm" matches "编辑别名"', () {
    final result = WoxFuzzyMatchUtil.match(
      text: '编辑别名',
      pattern: 'bm',
      usePinYin: true,
    );
    expect(result.isMatch, isTrue);
  });

  test('pinyin mixed mode rejected: "nih" does not match "你好"', () {
    final result = WoxFuzzyMatchUtil.match(
      text: '你好',
      pattern: 'nih',
      usePinYin: true,
    );
    expect(result.isMatch, isFalse);
  });
}
