import 'package:flutter_test/flutter_test.dart';
import 'package:wox/utils/wox_fuzzy_match_util.dart';

void main() {
  group('WoxFuzzyMatchUtil regressions', () {
    test('short texts keep the best scoring alignment', () {
      final result = WoxFuzzyMatchUtil.match(text: 'a_b_abc', pattern: 'abc', usePinYin: false);

      expect(result.isMatch, isTrue);
      expect(result.score, 70);
    });

    test('short contained ASCII substrings still match through the fallback path', () {
      final result = WoxFuzzyMatchUtil.match(text: 'zzabzz', pattern: 'ab', usePinYin: false);

      expect(result.isMatch, isTrue, reason: 'Keep the Dart matcher behavior covered until the Go ASCII fast path applies the same fallback contract.');
      expect(result.score, 2);
    });
  });
}
