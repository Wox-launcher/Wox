using NLog;
using System;
using System.Collections.Generic;
using System.Collections.Specialized;
using System.Linq;
using System.Runtime.Caching;
using Wox.Infrastructure.Logger;
using static Wox.Infrastructure.StringMatcher;

namespace Wox.Infrastructure
{
    public class StringMatcher
    {

        public SearchPrecisionScore UserSettingSearchPrecision { get; set; }

        private readonly Alphabet _alphabet;
        private MemoryCache _cache;

        private static readonly NLog.Logger Logger = LogManager.GetCurrentClassLogger();

        public StringMatcher()
        {
            _alphabet = new Alphabet();
            _alphabet.Initialize();

            NameValueCollection config = new NameValueCollection();
            config.Add("pollingInterval", "00:05:00");
            config.Add("physicalMemoryLimitPercentage", "1");
            config.Add("cacheMemoryLimitMegabytes", "30");
            _cache = new MemoryCache("StringMatcherCache", config);
        }

        public static StringMatcher Instance { get; internal set; }

        public static MatchResult FuzzySearch(string query, string stringToCompare)
        {
            return Instance.FuzzyMatch(query, stringToCompare);
        }

        public MatchResult FuzzyMatch(string query, string stringToCompare)
        {
            query = query.Trim();
            if (string.IsNullOrEmpty(stringToCompare) || string.IsNullOrEmpty(query)) return new MatchResult(false, UserSettingSearchPrecision);
            var queryWithoutCase = query.ToLower();
            string translated = _alphabet.Translate(stringToCompare);

            string key = $"{queryWithoutCase}|{translated}";
            MatchResult match = _cache[key] as MatchResult;
            if (match == null)
            {
                match = FuzzyMatchRecurrsive(
                    queryWithoutCase, translated, 0, 0, new List<int>()
                );
                CacheItemPolicy policy = new CacheItemPolicy();
                policy.SlidingExpiration = new TimeSpan(12, 0, 0);
                _cache.Set(key, match, policy);
            }

            return match;
        }

        public MatchResult FuzzyMatchRecurrsive(
            string query, string stringToCompare, int queryCurrentIndex, int stringCurrentIndex, List<int> sourceMatchData
        )
        {
            if (queryCurrentIndex == query.Length || stringCurrentIndex == stringToCompare.Length)
            {
                return new MatchResult(false, UserSettingSearchPrecision);
            }

            bool recursiveMatch = false;
            List<int> bestRecursiveMatchData = new List<int>();
            int bestRecursiveScore = 0;

            List<int> matchs = new List<int>();
            if (sourceMatchData.Count > 0)
            {
                foreach (var data in sourceMatchData)
                {
                    matchs.Add(data);
                }
            }

            while (queryCurrentIndex < query.Length && stringCurrentIndex < stringToCompare.Length)
            {
                char queryLower = char.ToLower(query[queryCurrentIndex]);
                char stringToCompareLower = char.ToLower(stringToCompare[stringCurrentIndex]);
                if (queryLower == stringToCompareLower)
                {
                    MatchResult match = FuzzyMatchRecurrsive(
                        query, stringToCompare, queryCurrentIndex, stringCurrentIndex + 1, matchs
                    );

                    if (match.Success)
                    {
                        if (!recursiveMatch || match.RawScore > bestRecursiveScore)
                        {
                            bestRecursiveMatchData = new List<int>();
                            foreach (int data in match.MatchData)
                            {
                                bestRecursiveMatchData.Add(data);
                            }
                            bestRecursiveScore = match.Score;
                        }
                        recursiveMatch = true;
                    }

                    matchs.Add(stringCurrentIndex);
                    queryCurrentIndex += 1;
                }
                stringCurrentIndex += 1;
            }

            bool matched = queryCurrentIndex == query.Length;
            int outScore;
            if (matched)
            {
                outScore = 100;
                int penality = 3 * matchs[0];
                outScore = outScore - penality;

                int unmatched = stringToCompare.Length - matchs.Count;
                outScore = outScore - (5 * unmatched);

                int consecutiveMatch = 0;
                for (int i = 0; i < matchs.Count; i++)
                {
                    int indexCurent = matchs[i];
                    if (i > 0)
                    {
                        int indexPrevious = matchs[i - 1];
                        if (indexCurent == indexPrevious + 1)
                        {
                            consecutiveMatch += 1;
                            outScore += 10 * consecutiveMatch;
                        } else
                        {
                            consecutiveMatch = 0;
                        }
                    }

                    char current = stringToCompare[indexCurent];
                    bool currentUpper = char.IsUpper(current);
                    if (indexCurent > 0)
                    {
                        char neighbor = stringToCompare[indexCurent - 1];
                        if (currentUpper && char.IsLower(neighbor))
                        {
                            outScore += 30;
                        }

                        bool isNeighbourSeparator = neighbor == '_' || neighbor == ' ';
                        if (isNeighbourSeparator)
                        {
                            outScore += 50;
                            if (currentUpper)
                            {
                                outScore += 50;
                            }
                        }
                    }
                    else
                    {
                        outScore += 50;
                        if (currentUpper)
                        {
                            outScore += 50;
                        }
                    }
                }
            }
            else
            {
                outScore = 0;
            }

            if (recursiveMatch && (!matched || bestRecursiveScore > outScore))
            {
                matchs = new List<int>();
                foreach (int data in bestRecursiveMatchData)
                {
                    matchs.Add(data);
                }
                outScore = bestRecursiveScore;
                return new MatchResult(true, UserSettingSearchPrecision, matchs, outScore);
            }
            else if (matched)
            {
                return new MatchResult(true, UserSettingSearchPrecision, matchs, outScore);
            }
            else
            {
                return new MatchResult(false, UserSettingSearchPrecision);
            }
        }

        public enum SearchPrecisionScore
        {
            Regular = 50,
            Low = 20,
            None = 0
        }
    }

    public class MatchResult
    {
        public MatchResult(bool success, SearchPrecisionScore searchPrecision)
        {
            Success = success;
            SearchPrecision = searchPrecision;
        }

        public MatchResult(bool success, SearchPrecisionScore searchPrecision, List<int> matchData, int rawScore)
        {
            Success = success;
            SearchPrecision = searchPrecision;
            MatchData = matchData;
            RawScore = rawScore;
        }

        public bool Success { get; set; }

        /// <summary>
        /// The final score of the match result with search precision filters applied.
        /// </summary>
        public int Score { get; private set; }

        /// <summary>
        /// The raw calculated search score without any search precision filtering applied.
        /// </summary>
        private int _rawScore;

        public int RawScore
        {
            get { return _rawScore; }
            set
            {
                _rawScore = value;
                Score = ScoreAfterSearchPrecisionFilter(_rawScore);
            }
        }

        /// <summary>
        /// Matched data to highlight.
        /// </summary>
        public List<int> MatchData { get; set; }

        public SearchPrecisionScore SearchPrecision { get; set; }

        public bool IsSearchPrecisionScoreMet()
        {
            return IsSearchPrecisionScoreMet(_rawScore);
        }

        private bool IsSearchPrecisionScoreMet(int rawScore)
        {
            return rawScore >= (int)SearchPrecision;
        }

        private int ScoreAfterSearchPrecisionFilter(int rawScore)
        {
            return IsSearchPrecisionScoreMet(rawScore) ? rawScore : 0;
        }
    }

}
