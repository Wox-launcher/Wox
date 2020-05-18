using System;

namespace Wox.Infrastructure
{
    [Obsolete("This class is obsolete and should not be used. Please use the static function StringMatcher.FuzzySearch")]
    public class FuzzyMatcher
    {
        private string query;

        private FuzzyMatcher(string query)
        {
            this.query = query.Trim();
        }

        public static FuzzyMatcher Create(string query)
        {
            return new FuzzyMatcher(query);
        }

        public MatchResult Evaluate(string str)
        {
            return StringMatcher.Instance.FuzzyMatch(query, str);
        }
    }
}
