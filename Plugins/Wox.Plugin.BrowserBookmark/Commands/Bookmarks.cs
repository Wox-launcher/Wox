using System.Collections.Generic;
using System.Linq;
using Wox.Infrastructure;

namespace Wox.Plugin.BrowserBookmark.Commands
{
    internal static class Bookmarks
    {
        internal static bool MatchProgram(Bookmark bookmark, string queryString)
        {
            if (StringMatcher.FuzzySearch(queryString, bookmark.Name).IsSearchPrecisionScoreMet()) return true;
            //if (StringMatcher.FuzzySearch(queryString, bookmark.PinyinName).IsSearchPrecisionScoreMet()) return true;
            if (StringMatcher.FuzzySearch(queryString, bookmark.Url).IsSearchPrecisionScoreMet()) return true;

            return false;
        }

    }
}
