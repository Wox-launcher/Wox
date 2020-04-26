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

        internal static List<Bookmark> LoadAllBookmarks()
        {
            var chromeBookmarks = new ChromeBookmarks();
            var mozBookmarks = new FirefoxBookmarks();

            //TODO: Let the user select which browser's bookmarks are displayed
            var b1 = mozBookmarks.GetBookmarks();
            var b2 = chromeBookmarks.GetBookmarks();
            b1.AddRange(b2);
            return b1.Distinct().ToList();
        }
    }
}
