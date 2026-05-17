import 'package:lpinyin/lpinyin.dart';

//////////////////////////////////////////////////////////////////////////////////
///
///   SHOULD KEEP THIS FILE IN SYNC WITH fuzz_match.go IN wox.core
///
/////////////////////////////////////////////////////////////////////////////////////

class WoxFuzzyMatchResult {
  final bool isMatch;
  final int score;

  const WoxFuzzyMatchResult({required this.isMatch, required this.score});
}

class WoxFuzzyMatchUtil {
  static const int _scoreMatch = 16;
  static const int _scoreGapStart = -3;
  static const int _scoreGapExtension = -1;
  static const int _bonusBoundary = _scoreMatch ~/ 2; // 8
  static const int _bonusNonWord = _scoreMatch ~/ 2; // 8
  static const int _bonusCamelCase = _bonusBoundary + 2; // 10
  static const int _bonusFirstCharMatch = _bonusBoundary + 4; // 12
  static const int _bonusConsecutive = 5;
  static const int _bonusPrefixMatch = 20;
  static const int _bonusExactMatch = 100;

  static final RegExp _alphabeticRegExp = RegExp(r'^[a-zA-Z]+$');
  static final RegExp _hasChineseRegExp = RegExp(r'[\u4e00-\u9fff]');
  static final RegExp _chineseRegExp = RegExp(r'[\u4e00-\u9fff]');

  static final RegExp _isLowerLetterRegExp = RegExp(r'^\p{Ll}$', unicode: true);
  static final RegExp _isUpperLetterRegExp = RegExp(r'^\p{Lu}$', unicode: true);
  static final RegExp _isLetterRegExp = RegExp(r'^\p{L}$', unicode: true);
  static final RegExp _isNumberRegExp = RegExp(r'^\p{N}$', unicode: true);

  static const Map<int, int> _diacriticsMap = {
    0x00C0: 0x0061, // À -> a
    0x00C1: 0x0061, // Á -> a
    0x00C2: 0x0061, // Â -> a
    0x00C3: 0x0061, // Ã -> a
    0x00C4: 0x0061, // Ä -> a
    0x00C5: 0x0061, // Å -> a
    0x00E0: 0x0061, // à -> a
    0x00E1: 0x0061, // á -> a
    0x00E2: 0x0061, // â -> a
    0x00E3: 0x0061, // ã -> a
    0x00E4: 0x0061, // ä -> a
    0x00E5: 0x0061, // å -> a
    0x0101: 0x0061, // ā -> a
    0x0103: 0x0061, // ă -> a
    0x0105: 0x0061, // ą -> a
    0x00C7: 0x0063, // Ç -> c
    0x00E7: 0x0063, // ç -> c
    0x00D0: 0x0064, // Ð -> d
    0x00F0: 0x0064, // ð -> d
    0x00C8: 0x0065, // È -> e
    0x00C9: 0x0065, // É -> e
    0x00CA: 0x0065, // Ê -> e
    0x00CB: 0x0065, // Ë -> e
    0x00E8: 0x0065, // è -> e
    0x00E9: 0x0065, // é -> e
    0x00EA: 0x0065, // ê -> e
    0x00EB: 0x0065, // ë -> e
    0x0113: 0x0065, // ē -> e
    0x0117: 0x0065, // ė -> e
    0x0119: 0x0065, // ę -> e
    0x00CC: 0x0069, // Ì -> i
    0x00CD: 0x0069, // Í -> i
    0x00CE: 0x0069, // Î -> i
    0x00CF: 0x0069, // Ï -> i
    0x00EC: 0x0069, // ì -> i
    0x00ED: 0x0069, // í -> i
    0x00EE: 0x0069, // î -> i
    0x00EF: 0x0069, // ï -> i
    0x00D1: 0x006E, // Ñ -> n
    0x00F1: 0x006E, // ñ -> n
    0x00D2: 0x006F, // Ò -> o
    0x00D3: 0x006F, // Ó -> o
    0x00D4: 0x006F, // Ô -> o
    0x00D5: 0x006F, // Õ -> o
    0x00D6: 0x006F, // Ö -> o
    0x00F2: 0x006F, // ò -> o
    0x00F3: 0x006F, // ó -> o
    0x00F4: 0x006F, // ô -> o
    0x00F5: 0x006F, // õ -> o
    0x00F6: 0x006F, // ö -> o
    0x00D9: 0x0075, // Ù -> u
    0x00DA: 0x0075, // Ú -> u
    0x00DB: 0x0075, // Û -> u
    0x00DC: 0x0075, // Ü -> u
    0x00F9: 0x0075, // ù -> u
    0x00FA: 0x0075, // ú -> u
    0x00FB: 0x0075, // û -> u
    0x00FC: 0x0075, // ü -> u
    0x00DD: 0x0079, // Ý -> y
    0x00FD: 0x0079, // ý -> y
    0x00FF: 0x0079, // ÿ -> y
    0x00DF: 0x0073, // ß -> s
  };

  static bool isFuzzyMatch({required String text, required String pattern, required bool usePinYin}) {
    return match(text: text, pattern: pattern, usePinYin: usePinYin).isMatch;
  }

  static WoxFuzzyMatchResult match({required String text, required String pattern, required bool usePinYin}) {
    if (pattern.isEmpty) {
      return const WoxFuzzyMatchResult(isMatch: true, score: 0);
    }
    if (text.isEmpty) {
      return const WoxFuzzyMatchResult(isMatch: false, score: 0);
    }

    final normalizedText = _normalizeString(text);
    final normalizedPattern = _normalizeString(pattern);

    if (normalizedText == normalizedPattern) {
      return WoxFuzzyMatchResult(isMatch: true, score: _bonusExactMatch + normalizedPattern.runes.length * _scoreMatch);
    }

    if (normalizedText.startsWith(normalizedPattern)) {
      return WoxFuzzyMatchResult(isMatch: true, score: _bonusPrefixMatch + normalizedPattern.runes.length * _scoreMatch + _bonusFirstCharMatch);
    }

    final coreResult = _fuzzyMatchCore(originalText: text, normalizedText: normalizedText, normalizedPattern: normalizedPattern);
    if (coreResult.isMatch) {
      return coreResult;
    }

    if (usePinYin && _hasChineseRegExp.hasMatch(text)) {
      final pinyinResult = _matchPinyinStrict(text, normalizedPattern);
      if (pinyinResult.isMatch) {
        return pinyinResult;
      }
    }

    if (normalizedText.contains(normalizedPattern)) {
      return WoxFuzzyMatchResult(isMatch: true, score: normalizedPattern.runes.length);
    }

    return const WoxFuzzyMatchResult(isMatch: false, score: 0);
  }

  static WoxFuzzyMatchResult _matchPinyinStrict(String text, String normalizedPattern) {
    final segments = _buildPinyinSegments(text);
    if (segments.isEmpty) {
      return const WoxFuzzyMatchResult(isMatch: false, score: 0);
    }

    final patternRunes = normalizedPattern.runes.toList(growable: false);
    var matched = false;
    var bestScore = 0;

    // Bug fix: keep the Dart matcher aligned with Go's segment graph. The old
    // Dart-only first-letter string check missed initials that start after
    // skipped syllables, so action filtering could not find Chinese phrases like
    // "移除" inside longer labels. The state search below mirrors Go's skip and
    // mode rules so initials can start later without making mixed pinyin modes
    // too permissive for short queries.
    if (patternRunes.length <= segments.length) {
      var firstLetterMatch = true;
      for (var i = 0; i < patternRunes.length; i++) {
        if (!segments[i].firstLetters.contains(patternRunes[i])) {
          firstLetterMatch = false;
          break;
        }
      }

      if (firstLetterMatch) {
        bestScore =
            patternRunes.length == segments.length
                ? _bonusExactMatch + patternRunes.length * _scoreMatch
                : _bonusPrefixMatch + patternRunes.length * _scoreMatch + _bonusFirstCharMatch;
        matched = true;
      }
    }

    var states = <_PinyinSearchState>[
      const _PinyinSearchState(patternIdx: 0, consecutiveSkipped: 0, matchedSyllables: 0, score: 0, lastMatchWasPartial: false, matchMode: _pinyinModeAny),
    ];

    for (final segment in segments) {
      final nextStates = <_PinyinSearchState>[];
      final bestStateByKey = <String, int>{};
      final bestScoreByKey = <String, int>{};

      void addState(_PinyinSearchState newState) {
        final key = '${newState.patternIdx}:${newState.matchedSyllables}:${newState.matchMode}:${newState.consecutiveSkipped}';
        final currentScore = bestScoreByKey[key];
        if (currentScore != null && currentScore >= newState.score) {
          return;
        }
        final existingIndex = bestStateByKey[key];
        if (existingIndex == null) {
          bestStateByKey[key] = nextStates.length;
          bestScoreByKey[key] = newState.score;
          nextStates.add(newState);
        } else {
          bestScoreByKey[key] = newState.score;
          nextStates[existingIndex] = newState;
        }
      }

      for (final state in states) {
        if (state.patternIdx == patternRunes.length && state.matchedSyllables > 0) {
          final finalScore = state.score + (state.matchedSyllables == segments.length && !state.lastMatchWasPartial ? _bonusExactMatch : 0);
          if (!matched || finalScore > bestScore) {
            bestScore = finalScore;
            matched = true;
          }
          continue;
        }

        if (state.matchedSyllables == 0 || state.consecutiveSkipped < _maxConsecutiveSkippedSyllables) {
          final newSkips = state.consecutiveSkipped + 1;
          addState(state.copyWith(consecutiveSkipped: newSkips > _maxConsecutiveSkippedSyllables ? _maxConsecutiveSkippedSyllables : newSkips, lastMatchWasPartial: false));
        }

        if (state.patternIdx >= patternRunes.length) {
          continue;
        }

        for (var syllableIdx = 0; syllableIdx < segment.syllables.length; syllableIdx++) {
          final syllable = segment.syllables[syllableIdx];
          final syllableRunes = syllable.runes.toList(growable: false);
          final remainingRunes = patternRunes.length - state.patternIdx;
          final syllableLen = syllableRunes.length;

          if (remainingRunes >= syllableLen && _matchASCIIPrefix(patternRunes, state.patternIdx, syllableRunes, syllableLen)) {
            if (syllableLen == 1 || state.matchMode != _pinyinModeFirstLetter) {
              addState(
                _PinyinSearchState(
                  patternIdx: state.patternIdx + syllableLen,
                  consecutiveSkipped: 0,
                  matchedSyllables: state.matchedSyllables + 1,
                  score: state.score + _scoreMatch * 2 + (state.matchedSyllables > 0 && state.consecutiveSkipped == 0 ? _bonusConsecutive : 0),
                  lastMatchWasPartial: false,
                  matchMode: syllableLen > 1 && segment.isChinese ? _pinyinModeFullPinyin : state.matchMode,
                ),
              );
            }
          }

          if (remainingRunes < syllableLen && _matchASCIIPrefix(patternRunes, state.patternIdx, syllableRunes, remainingRunes)) {
            if (remainingRunes == 1) {
              if (state.matchMode != _pinyinModeFullPinyin) {
                addState(
                  _PinyinSearchState(
                    patternIdx: state.patternIdx + 1,
                    consecutiveSkipped: 0,
                    matchedSyllables: state.matchedSyllables + 1,
                    score: state.score + _scoreMatch + 5 + (state.matchedSyllables > 0 && state.consecutiveSkipped == 0 ? _bonusConsecutive : 0),
                    lastMatchWasPartial: true,
                    matchMode: _pinyinModeFirstLetter,
                  ),
                );
              }
            } else if (state.matchMode != _pinyinModeFirstLetter && state.consecutiveSkipped == 0) {
              addState(
                _PinyinSearchState(
                  patternIdx: state.patternIdx + remainingRunes,
                  consecutiveSkipped: 0,
                  matchedSyllables: state.matchedSyllables + 1,
                  score: state.score + remainingRunes * _scoreMatch + (state.matchedSyllables > 0 ? _bonusConsecutive : 0),
                  lastMatchWasPartial: true,
                  matchMode: _pinyinModeFullPinyin,
                ),
              );
            }
          }

          if (remainingRunes > 1 && syllableLen > 1 && patternRunes[state.patternIdx] == segment.firstLetters[syllableIdx] && state.matchMode != _pinyinModeFullPinyin) {
            addState(
              _PinyinSearchState(
                patternIdx: state.patternIdx + 1,
                consecutiveSkipped: 0,
                matchedSyllables: state.matchedSyllables + 1,
                score: state.score + _scoreMatch + 5 + (state.matchedSyllables > 0 && state.consecutiveSkipped == 0 ? _bonusConsecutive : 0),
                lastMatchWasPartial: false,
                matchMode: _pinyinModeFirstLetter,
              ),
            );
          }
        }
      }

      states = nextStates;
      if (states.isEmpty) {
        break;
      }
    }

    for (final state in states) {
      if (state.patternIdx == patternRunes.length && state.matchedSyllables > 0) {
        final finalScore = state.score + (state.matchedSyllables == segments.length && !state.lastMatchWasPartial ? _bonusExactMatch : 0);
        if (!matched || finalScore > bestScore) {
          bestScore = finalScore;
          matched = true;
        }
      }
    }

    return WoxFuzzyMatchResult(isMatch: matched, score: bestScore);
  }

  static const int _maxConsecutiveSkippedSyllables = 3;
  static const int _pinyinModeAny = 0;
  static const int _pinyinModeFirstLetter = 1;
  static const int _pinyinModeFullPinyin = 2;

  static List<_PinyinSegment> _buildPinyinSegments(String text) {
    final segments = <_PinyinSegment>[];
    final asciiBuilder = StringBuffer();

    void flushAscii() {
      if (asciiBuilder.isEmpty) {
        return;
      }
      final raw = asciiBuilder.toString();
      final normalized = _normalizeString(raw);
      if (normalized.isNotEmpty) {
        segments.add(_PinyinSegment(syllables: [normalized], firstLetters: [normalized.runes.first], isChinese: false));
      }
      asciiBuilder.clear();
    }

    // Match the Go matcher shape by treating each Han character as a pinyin
    // segment and merging adjacent non-Chinese text into one segment. Keeping
    // this structure local avoids special cases in action/result filtering.
    for (final rune in text.runes) {
      final char = String.fromCharCode(rune);
      if (_chineseRegExp.hasMatch(char)) {
        flushAscii();
        final syllables = PinyinHelper.convertToPinyinArray(
          char,
          PinyinFormat.WITHOUT_TONE,
        ).map(_normalizeString).where((part) => part.isNotEmpty && _alphabeticRegExp.hasMatch(part)).toList(growable: false);
        if (syllables.isEmpty) {
          segments.add(_PinyinSegment(syllables: [char], firstLetters: [rune], isChinese: true));
        } else {
          segments.add(_PinyinSegment(syllables: syllables, firstLetters: syllables.map((part) => part.runes.first).toList(growable: false), isChinese: true));
        }
      } else {
        asciiBuilder.write(char);
      }
    }
    flushAscii();

    return segments;
  }

  static bool _matchASCIIPrefix(List<int> patternRunes, int patternOffset, List<int> syllableRunes, int length) {
    if (patternOffset + length > patternRunes.length || length > syllableRunes.length) {
      return false;
    }
    for (var i = 0; i < length; i++) {
      if (_toLowerASCII(patternRunes[patternOffset + i]) != _toLowerASCII(syllableRunes[i])) {
        return false;
      }
    }
    return true;
  }

  static WoxFuzzyMatchResult _fuzzyMatchCore({required String originalText, required String normalizedText, required String normalizedPattern}) {
    final textRunes = normalizedText.runes.toList(growable: false);
    final patternRunes = normalizedPattern.runes.toList(growable: false);
    final originalRunes = originalText.runes.toList(growable: false);

    final textLen = textRunes.length;
    final patternLen = patternRunes.length;

    if (patternLen == 0) {
      return const WoxFuzzyMatchResult(isMatch: true, score: 0);
    }
    if (textLen == 0 || patternLen > textLen) {
      return const WoxFuzzyMatchResult(isMatch: false, score: 0);
    }

    var patternIdx = 0;
    for (var textIdx = 0; textIdx < textLen && patternIdx < patternLen; textIdx++) {
      if (textRunes[textIdx] == patternRunes[patternIdx]) {
        patternIdx++;
      }
    }
    if (patternIdx != patternLen) {
      return const WoxFuzzyMatchResult(isMatch: false, score: 0);
    }

    final matchedPositions = _optimizeMatchPositions(originalRunes: originalRunes, textRunes: textRunes, patternRunes: patternRunes);

    final score = _calculateScore(originalRunes: originalRunes, textRunes: textRunes, matchedIndexes: matchedPositions, patternLen: patternLen);

    final minScore = _calculateMinScoreThreshold(patternLen: patternLen, textLen: textLen);
    if (score < minScore) {
      return const WoxFuzzyMatchResult(isMatch: false, score: 0);
    }

    return WoxFuzzyMatchResult(isMatch: true, score: score);
  }

  static List<int> _optimizeMatchPositions({required List<int> originalRunes, required List<int> textRunes, required List<int> patternRunes}) {
    final textLen = textRunes.length;
    final patternLen = patternRunes.length;
    final matchedIndexes = List<int>.filled(patternLen, 0);

    var patternIdx = 0;
    for (var textIdx = 0; textIdx < textLen && patternIdx < patternLen; textIdx++) {
      if (textRunes[textIdx] != patternRunes[patternIdx]) {
        continue;
      }

      final isBoundary = textIdx == 0 || _isBoundaryChar(originalRunes, textIdx);
      final isConsecutive = patternIdx > 0 && matchedIndexes[patternIdx - 1] == textIdx - 1;

      if (isBoundary || isConsecutive) {
        matchedIndexes[patternIdx] = textIdx;
        patternIdx++;
        continue;
      }

      var foundBetter = false;
      for (var lookAhead = textIdx + 1; lookAhead < textLen && lookAhead < textIdx + 10; lookAhead++) {
        if (textRunes[lookAhead] == patternRunes[patternIdx] && _isBoundaryChar(originalRunes, lookAhead)) {
          foundBetter = true;
          break;
        }
      }

      if (!foundBetter) {
        matchedIndexes[patternIdx] = textIdx;
        patternIdx++;
      }
    }

    if (patternIdx != patternLen) {
      patternIdx = 0;
      for (var textIdx = 0; textIdx < textLen && patternIdx < patternLen; textIdx++) {
        if (textRunes[textIdx] == patternRunes[patternIdx]) {
          matchedIndexes[patternIdx] = textIdx;
          patternIdx++;
        }
      }
    }

    return matchedIndexes;
  }

  static int _calculateScore({required List<int> originalRunes, required List<int> textRunes, required List<int> matchedIndexes, required int patternLen}) {
    if (matchedIndexes.isEmpty) {
      return 0;
    }

    var score = 0;
    var prevMatchIdx = -1;

    for (var i = 0; i < matchedIndexes.length; i++) {
      final matchIdx = matchedIndexes[i];

      score += _scoreMatch;

      if (matchIdx == 0) {
        score += _bonusFirstCharMatch;
      }

      if (matchIdx > 0) {
        final prevChar = originalRunes[matchIdx - 1];
        final currChar = originalRunes[matchIdx];

        if (_isLowerLetter(prevChar) && _isUpperLetter(currChar)) {
          score += _bonusCamelCase;
        }

        if (_isDelimiter(prevChar)) {
          score += _bonusBoundary;
        }

        if (!_isLetterOrNumber(prevChar) && _isLetterOrNumber(currChar)) {
          score += _bonusNonWord;
        }
      }

      if (i > 0 && matchIdx == prevMatchIdx + 1) {
        score += _bonusConsecutive;
      }

      if (prevMatchIdx >= 0) {
        final gap = matchIdx - prevMatchIdx - 1;
        if (gap > 0) {
          score += _scoreGapStart + (gap - 1) * _scoreGapExtension;
        }
      } else if (matchIdx > 0) {
        final leadingGap = matchIdx;
        var penalty = leadingGap * _scoreGapExtension;
        if (penalty < -15) {
          penalty = -15;
        }
        score += penalty;
      }

      prevMatchIdx = matchIdx;
    }

    final textLen = textRunes.length;
    if (prevMatchIdx >= 0 && prevMatchIdx < textLen - 1) {
      final trailingGap = textLen - prevMatchIdx - 1;
      var penalty = (trailingGap * _scoreGapExtension) ~/ 2;
      if (penalty < -10) {
        penalty = -10;
      }
      score += penalty;
    }

    final matchRatio = patternLen / textLen;
    if (matchRatio > 0.5) {
      score += (matchRatio * 10).toInt();
    }

    return score;
  }

  static int _calculateMinScoreThreshold({required int patternLen, required int textLen}) {
    if (patternLen == 1) {
      if (textLen <= 2) {
        return _scoreMatch;
      }
      return _scoreMatch + _bonusBoundary;
    }

    if (patternLen == 2) {
      if (textLen <= 4) {
        return _scoreMatch * 2;
      }
      return _scoreMatch * 2 + _bonusConsecutive;
    }

    if (patternLen == 3) {
      if (textLen <= 6) {
        return (patternLen * _scoreMatch * 2) ~/ 3;
      }
      return (patternLen * _scoreMatch * 2) ~/ 3 + _bonusConsecutive;
    }

    final ratio = patternLen / textLen;
    if (ratio < 0.15) {
      return patternLen * _scoreMatch;
    }
    if (ratio < 0.3) {
      return (patternLen * _scoreMatch * 3) ~/ 4;
    }
    if (ratio < 0.5) {
      return (patternLen * _scoreMatch * 2) ~/ 3;
    }
    return (patternLen * _scoreMatch) ~/ 2;
  }

  static bool _isBoundaryChar(List<int> runes, int idx) {
    if (idx == 0) {
      return true;
    }
    if (idx >= runes.length) {
      return false;
    }

    final prev = runes[idx - 1];
    final curr = runes[idx];

    if (_isLowerLetter(prev) && _isUpperLetter(curr)) {
      return true;
    }
    if (_isDelimiter(prev)) {
      return true;
    }
    if (!_isLetterOrNumber(prev) && _isLetterOrNumber(curr)) {
      return true;
    }
    return false;
  }

  static bool _isLowerLetter(int rune) {
    return _isLowerLetterRegExp.hasMatch(String.fromCharCode(rune));
  }

  static bool _isUpperLetter(int rune) {
    return _isUpperLetterRegExp.hasMatch(String.fromCharCode(rune));
  }

  static bool _isLetterOrNumber(int rune) {
    final s = String.fromCharCode(rune);
    return _isLetterRegExp.hasMatch(s) || _isNumberRegExp.hasMatch(s);
  }

  static bool _isDelimiter(int rune) {
    const delimiters = {
      0x20, // space
      0x2D, // -
      0x5F, // _
      0x2E, // .
      0x2F, // /
      0x5C, // \
      0x3A, // :
      0x2C, // ,
      0x3B, // ;
      0x28, // (
      0x29, // )
      0x5B, // [
      0x5D, // ]
      0x7B, // {
      0x7D, // }
    };
    return delimiters.contains(rune);
  }

  static String _normalizeString(String s) {
    final builder = StringBuffer();
    for (final rune in s.runes) {
      final lowered = String.fromCharCode(rune).toLowerCase();
      final loweredRunes = lowered.runes.toList(growable: false);
      if (loweredRunes.length != 1) {
        builder.write(lowered);
        continue;
      }

      final loweredRune = loweredRunes[0];
      final mapped = _diacriticsMap[loweredRune] ?? loweredRune;
      builder.writeCharCode(mapped);
    }
    return builder.toString();
  }

  static int _toLowerASCII(int rune) {
    if (rune >= 0x41 && rune <= 0x5A) {
      return rune + 0x20;
    }
    return rune;
  }
}

class _PinyinSegment {
  // Segment-level pinyin data mirrors Go's PinyinSegment so the Dart and Go
  // matchers make the same decisions for skipped syllables and initials.
  final List<String> syllables;
  final List<int> firstLetters;
  final bool isChinese;

  const _PinyinSegment({required this.syllables, required this.firstLetters, required this.isChinese});
}

class _PinyinSearchState {
  // Search state mirrors Go's pinyinSearchState. Keeping the mode in the state
  // preserves the existing restriction against noisy mixed first-letter/full-pinyin
  // matches while allowing later phrase initials such as "yc".
  final int patternIdx;
  final int consecutiveSkipped;
  final int matchedSyllables;
  final int score;
  final bool lastMatchWasPartial;
  final int matchMode;

  const _PinyinSearchState({
    required this.patternIdx,
    required this.consecutiveSkipped,
    required this.matchedSyllables,
    required this.score,
    required this.lastMatchWasPartial,
    required this.matchMode,
  });

  _PinyinSearchState copyWith({int? patternIdx, int? consecutiveSkipped, int? matchedSyllables, int? score, bool? lastMatchWasPartial, int? matchMode}) {
    return _PinyinSearchState(
      patternIdx: patternIdx ?? this.patternIdx,
      consecutiveSkipped: consecutiveSkipped ?? this.consecutiveSkipped,
      matchedSyllables: matchedSyllables ?? this.matchedSyllables,
      score: score ?? this.score,
      lastMatchWasPartial: lastMatchWasPartial ?? this.lastMatchWasPartial,
      matchMode: matchMode ?? this.matchMode,
    );
  }
}
