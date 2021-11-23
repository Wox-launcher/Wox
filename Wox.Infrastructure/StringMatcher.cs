using NLog;
using System;
using System.Collections.Generic;
using System.Collections.Specialized;
using System.Linq;
using System.Runtime.Caching;
using Wox.Infrastructure.Logger;
using static Wox.Infrastructure.StringMatcher;
using System.Threading;
using System.Globalization;

namespace Wox.Infrastructure
{
    public class StringMatcher
    {
        
        public SearchPrecisionScore UserSettingSearchPrecision { get; set; }

        private readonly Alphabet _alphabet;
        private MemoryCache _cache;

        private Nullable<CancellationToken> _token = null;

        private static readonly NLog.Logger Logger = LogManager.GetCurrentClassLogger();

        public StringMatcher(Nullable<CancellationToken> token = null)
        {
            _alphabet = new Alphabet();
            _alphabet.Initialize();

            NameValueCollection config = new NameValueCollection();
            config.Add("pollingInterval", "00:05:00");
            config.Add("physicalMemoryLimitPercentage", "1");
            config.Add("cacheMemoryLimitMegabytes", "30");
            _cache = new MemoryCache("StringMatcherCache", config);

            if (token != null)
            {
                _token = token;
            }
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
            var queryWithoutCase = query.ToLower(CultureInfo.InvariantCulture);
            string translated = _alphabet.Translate(stringToCompare.ToLower(CultureInfo.InvariantCulture));

            string key = $"{queryWithoutCase}|{translated}";
            MatchResult match = _cache[key] as MatchResult;
            if (match == null)
            {
                match = DoFuzzyMatch(
                    queryWithoutCase, translated, 0, 0, new List<int>()
                );
                CacheItemPolicy policy = new CacheItemPolicy();
                policy.SlidingExpiration = new TimeSpan(12, 0, 0);
                _cache.Set(key, match, policy);
            }

            return match;
        }

        public MatchResult DoFuzzyMatch(
            string query, string stringToCompare, int queryCurrentIndex, int stringCurrentIndex, List<int> sourceMatchData
        )
        {
            if (_token?.IsCancellationRequested ?? false) { return new MatchResult(false, UserSettingSearchPrecision); }
            if (queryCurrentIndex == query.Length || stringCurrentIndex == stringToCompare.Length)
            {
                return new MatchResult(false, UserSettingSearchPrecision);
            }

            var querySubstrings = query.Split(new[] { ' ' }, StringSplitOptions.RemoveEmptyEntries);
            int currentQuerySubstringIndex = 0;
            var currentQuerySubstring = querySubstrings[currentQuerySubstringIndex];
            var currentQuerySubstringCharacterIndex = 0;

            var firstMatchIndex = -1;
            var firstMatchIndexInWord = -1;
            var lastMatchIndex = 0;
            bool allQuerySubstringsMatched = false;
            bool matchFoundInPreviousLoop = false;
            bool allSubstringsContainedInCompareString = true;

            var indexList = new List<int>();
            List<int> spaceIndices = new List<int>();

            for (var compareStringIndex = 0; compareStringIndex < stringToCompare.Length; compareStringIndex++)
            {
                // To maintain a list of indices which correspond to spaces in the string to compare
                // To populate the list only for the first query substring
                if (stringToCompare[compareStringIndex].Equals(' ') && currentQuerySubstringIndex == 0)
                {
                    spaceIndices.Add(compareStringIndex);
                }

                bool compareResult;
                var fullStringToCompareChar = stringToCompare[compareStringIndex].ToString();
                var querySubstringChar = currentQuerySubstring[currentQuerySubstringCharacterIndex].ToString();
                compareResult = string.Compare(fullStringToCompareChar, querySubstringChar, CultureInfo.CurrentCulture, CompareOptions.IgnoreCase | CompareOptions.IgnoreNonSpace) != 0;

                if (compareResult)
                {
                    matchFoundInPreviousLoop = false;
                    continue;
                }

                if (firstMatchIndex < 0)
                {
                    // first matched char will become the start of the compared string
                    firstMatchIndex = compareStringIndex;
                }

                if (currentQuerySubstringCharacterIndex == 0)
                {
                    // first letter of current word
                    matchFoundInPreviousLoop = true;
                    firstMatchIndexInWord = compareStringIndex;
                }
                else if (!matchFoundInPreviousLoop)
                {
                    // we want to verify that there is not a better match if this is not a full word
                    // in order to do so we need to verify all previous chars are part of the pattern
                    var startIndexToVerify = compareStringIndex - currentQuerySubstringCharacterIndex;

                    if (AllPreviousCharsMatched(startIndexToVerify, currentQuerySubstringCharacterIndex, stringToCompare, currentQuerySubstring))
                    {
                        matchFoundInPreviousLoop = true;

                        // if it's the beginning character of the first query substring that is matched then we need to update start index
                        firstMatchIndex = currentQuerySubstringIndex == 0 ? startIndexToVerify : firstMatchIndex;

                        indexList = GetUpdatedIndexList(startIndexToVerify, currentQuerySubstringCharacterIndex, firstMatchIndexInWord, indexList);
                    }
                }

                lastMatchIndex = compareStringIndex + 1;
                indexList.Add(compareStringIndex);

                currentQuerySubstringCharacterIndex++;

                // if finished looping through every character in the current substring
                if (currentQuerySubstringCharacterIndex == currentQuerySubstring.Length)
                {
                    // if any of the substrings was not matched then consider as all are not matched
                    allSubstringsContainedInCompareString = matchFoundInPreviousLoop && allSubstringsContainedInCompareString;

                    currentQuerySubstringIndex++;

                    allQuerySubstringsMatched = AllQuerySubstringsMatched(currentQuerySubstringIndex, querySubstrings.Length);
                    if (allQuerySubstringsMatched)
                    {
                        break;
                    }

                    // otherwise move to the next query substring
                    currentQuerySubstring = querySubstrings[currentQuerySubstringIndex];
                    currentQuerySubstringCharacterIndex = 0;
                }
            }

            // proceed to calculate score if every char or substring without whitespaces matched
            if (allQuerySubstringsMatched)
            {
                var nearestSpaceIndex = CalculateClosestSpaceIndex(spaceIndices, firstMatchIndex);
                var score = CalculateSearchScore(query, stringToCompare, firstMatchIndex - nearestSpaceIndex - 1, lastMatchIndex - firstMatchIndex, allSubstringsContainedInCompareString);

                return new MatchResult(true, UserSettingSearchPrecision, indexList, score);
            }

            return new MatchResult(false, UserSettingSearchPrecision);
        }

        // To get the index of the closest space which precedes the first matching index
        private static int CalculateClosestSpaceIndex(List<int> spaceIndices, int firstMatchIndex)
        {
            if (spaceIndices.Count == 0)
            {
                return -1;
            }
            else
            {
                int? ind = spaceIndices.OrderBy(item => (firstMatchIndex - item)).Where(item => firstMatchIndex > item).FirstOrDefault();
                int closestSpaceIndex = ind ?? -1;
                return closestSpaceIndex;
            }
        }

        private static bool AllPreviousCharsMatched(int startIndexToVerify, int currentQuerySubstringCharacterIndex, string fullStringToCompareWithoutCase, string currentQuerySubstring)
        {
            var allMatch = true;
            for (int indexToCheck = 0; indexToCheck < currentQuerySubstringCharacterIndex; indexToCheck++)
            {
                if (fullStringToCompareWithoutCase[startIndexToVerify + indexToCheck] !=
                    currentQuerySubstring[indexToCheck])
                {
                    allMatch = false;
                }
            }

            return allMatch;
        }

        private static List<int> GetUpdatedIndexList(int startIndexToVerify, int currentQuerySubstringCharacterIndex, int firstMatchIndexInWord, List<int> indexList)
        {
            var updatedList = new List<int>();

            indexList.RemoveAll(x => x >= firstMatchIndexInWord);

            updatedList.AddRange(indexList);

            for (int indexToCheck = 0; indexToCheck < currentQuerySubstringCharacterIndex; indexToCheck++)
            {
                updatedList.Add(startIndexToVerify + indexToCheck);
            }

            return updatedList;
        }

        private static bool AllQuerySubstringsMatched(int currentQuerySubstringIndex, int querySubstringsLength)
        {
            return currentQuerySubstringIndex >= querySubstringsLength;
        }

        private static int CalculateSearchScore(string query, string stringToCompare, int firstIndex, int matchLen, bool allSubstringsContainedInCompareString)
        {
            // A match found near the beginning of a string is scored more than a match found near the end
            // A match is scored more if the characters in the patterns are closer to each other,
            // while the score is lower if they are more spread out
            var score = 100 * (query.Length + 1) / ((1 + firstIndex) + (matchLen + 1));

            // A match with less characters assigning more weights
            if (stringToCompare.Length - query.Length < 5)
            {
                score += 20;
            }
            else if (stringToCompare.Length - query.Length < 10)
            {
                score += 10;
            }

            if (allSubstringsContainedInCompareString)
            {
                int count = query.Count(c => !char.IsWhiteSpace(c));
                int threshold = 4;
                if (count <= threshold)
                {
                    score += count * 10;
                }
                else
                {
                    score += (threshold * 10) + ((count - threshold) * 5);
                }
            }

            // Using CurrentCultureIgnoreCase since this relates to queries input by user
            if (string.Equals(query, stringToCompare, StringComparison.CurrentCultureIgnoreCase))
            {
                var bonusForExactMatch = 10;
                score += bonusForExactMatch;
            }

            return score;
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
