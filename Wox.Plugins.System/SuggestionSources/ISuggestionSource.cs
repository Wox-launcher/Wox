using System.Collections.Generic;

namespace Wox.Plugins.System.SuggestionSources
{
    public interface ISuggestionSource
    {
        List<string> GetSuggestions(string query);
    }

    public abstract class AbstractSuggestionSource : ISuggestionSource
    {
        public abstract List<string> GetSuggestions(string query);
    }
}
