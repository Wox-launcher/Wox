using System.Collections.Generic;
using System.Windows.Documents;

namespace Wox.Plugin.Everything.Everything
{
    public class SearchResult
    {
        public string FileName { get; set; }
        public List<int> FileNameHightData { get; set; }
        public string FullPath { get; set; }
        public List<int> FullPathHightData { get; set; }
        public ResultType Type { get; set; }
    }
}