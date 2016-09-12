using System;
using System.Collections.Generic;
using System.Linq;
using Wox.Infrastructure;
using Wox.Infrastructure.Logger;

namespace Wox.Infrastructure
{
    public static class StringMatcher
    {
        public static MatchResult Match(string source, string target, bool pinyin = false)
        {
            if (string.IsNullOrEmpty(source) || string.IsNullOrEmpty(target))
                return new MatchResult {Score = 0};

            var matcher = FuzzyMatcher.Create(target);
            var result = matcher.Evaluate(source);

            if (pinyin)
            {
                // does pinyin score better?
                var pinyinScore = ScoreForPinyin(source, target);
                if (pinyinScore > result.Score)
                    result = new MatchResult() {Score = pinyinScore};
            }

            return result;
        }

        public static int Score(string source, string target, bool pinyin = false)
        {
            return Match(source, target, pinyin).Score;
        }

        // TODO: pinyin match should return match data for highlighting
        private static int ScoreForPinyin(string source, string target)
        {
            if (string.IsNullOrEmpty(source) || string.IsNullOrEmpty(target) || !Alphabet.ContainsChinese(source))
                return 0;

            FuzzyMatcher matcher = FuzzyMatcher.Create(target);

            //todo happlebao currently generate pinyin on every query, should be generate on startup/index
            var combination = Alphabet.PinyinComination(source);
            var pinyinScore = combination.Select(pinyin => matcher.Evaluate(string.Join("", pinyin)).Score)
                .Max();
            var acronymScore = combination.Select(Alphabet.Acronym)
                .Select(pinyin => matcher.Evaluate(pinyin).Score)
                .Max();
            var score = Math.Max(pinyinScore, acronymScore);
            return score;
        }

        public static bool IsMatch(string source, string target)
        {
            return Score(source, target) > 0;
        }
    }
}
