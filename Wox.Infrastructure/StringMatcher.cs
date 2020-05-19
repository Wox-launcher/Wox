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

        [Obsolete("This method is obsolete and should not be used. Please use the static function StringMatcher.FuzzySearch")]
        public static int Score(string source, string target)
        {
            return FuzzySearch(target, source).Score;
        }

        [Obsolete("This method is obsolete and should not be used. Please use the static function StringMatcher.FuzzySearch")]
        public static bool IsMatch(string source, string target)
        {
            return Score(source, target) > 0;
        }

        public static MatchResult FuzzySearch(string query, string stringToCompare)
        {
            return Instance.FuzzyMatch(query, stringToCompare);
        }

        public MatchResult FuzzyMatch(string query, string stringToCompare)
        {
            query = query.Trim();
            string[] translated;
            var queryWithoutCase = query.ToLower();
            string key = $"{queryWithoutCase}|{stringToCompare}";
            MatchResult match = _cache[key] as MatchResult;
            if (match == null)
            {
                translated = _alphabet.Translate(stringToCompare);
                if (string.IsNullOrEmpty(stringToCompare) || string.IsNullOrEmpty(query)) return new MatchResult(false, UserSettingSearchPrecision);
                var fullStringToCompareWithoutCase = string.Join("", translated).ToLower();

                match = FuzzyMatchInternal(queryWithoutCase, fullStringToCompareWithoutCase, translated);
                CacheItemPolicy policy = new CacheItemPolicy();
                policy.SlidingExpiration = new TimeSpan(12, 0, 0);
                _cache.Set(key, match, policy);
            }

            return match;
        }

        private int OriginIndexFromTranslated(int index, string[] translatedList)
        {
            int lengthOutter = translatedList.Length;
            int count = 0;
            for (int i = 0; i < lengthOutter; i++)
            {
                string part = translatedList[i];
                int lengthInner = part.Length;
                for (int j = 0; j < lengthInner; j++)
                {
                    if (index == count)
                    {
                        return i;
                    }
                    else
                    {
                        count = count + 1;
                    }
                }
            }
            throw new ArgumentException($"{nameof(OriginIndexFromTranslated)} cannot get index {index} {string.Join(",", translatedList)}");
        }
        /// <summary>
        /// Current method:
        /// Character matching + substring matching;
        /// 1. Query search string is split into substrings, separator is whitespace.
        /// 2. Check each query substring's characters against full compare string,
        /// 3. if a character in the substring is matched, loop back to verify the previous character.
        /// 4. If previous character also matches, and is the start of the substring, update list.
        /// 5. Once the previous character is verified, move on to the next character in the query substring.
        /// 6. Move onto the next substring's characters until all substrings are checked.
        /// 7. Consider success and move onto scoring if every char or substring without whitespaces matched
        /// </summary>
        public MatchResult FuzzyMatchInternal(string query, string translated, string[] translatedList)
        {
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

            for (var compareStringIndex = 0; compareStringIndex < translated.Length; compareStringIndex++)
            {
                if (translated[compareStringIndex] != currentQuerySubstring[currentQuerySubstringCharacterIndex])
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

                    if (AllPreviousCharsMatched(startIndexToVerify, currentQuerySubstringCharacterIndex, translated, currentQuerySubstring))
                    {
                        matchFoundInPreviousLoop = true;

                        // if it's the beginning character of the first query substring that is matched then we need to update start index
                        firstMatchIndex = currentQuerySubstringIndex == 0 ? startIndexToVerify : firstMatchIndex;

                        indexList = GetUpdatedIndexList(startIndexToVerify, currentQuerySubstringCharacterIndex, firstMatchIndexInWord, indexList);
                    }
                }

                lastMatchIndex = compareStringIndex + 1;

                indexList.Add(OriginIndexFromTranslated(compareStringIndex, translatedList));

                currentQuerySubstringCharacterIndex++;

                // if finished looping through every character in the current substring
                if (currentQuerySubstringCharacterIndex == currentQuerySubstring.Length)
                {
                    // if any of the substrings was not matched then consider as all are not matched
                    allSubstringsContainedInCompareString = matchFoundInPreviousLoop && allSubstringsContainedInCompareString;

                    currentQuerySubstringIndex++;

                    allQuerySubstringsMatched = AllQuerySubstringsMatched(currentQuerySubstringIndex, querySubstrings.Length);
                    if (allQuerySubstringsMatched)
                        break;

                    // otherwise move to the next query substring
                    currentQuerySubstring = querySubstrings[currentQuerySubstringIndex];
                    currentQuerySubstringCharacterIndex = 0;
                }
            }

            // proceed to calculate score if every char or substring without whitespaces matched
            if (allQuerySubstringsMatched)
            {
                var score = CalculateSearchScore(query, translated, firstMatchIndex, lastMatchIndex - firstMatchIndex, allSubstringsContainedInCompareString);

                return new MatchResult(true, UserSettingSearchPrecision, indexList, score);
            }

            return new MatchResult(false, UserSettingSearchPrecision);
        }

        private static bool AllPreviousCharsMatched(int startIndexToVerify, int currentQuerySubstringCharacterIndex,
                                                        string fullStringToCompareWithoutCase, string currentQuerySubstring)
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
                int factor = count < 4 ? 10 : 5;
                score += factor * count;
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
