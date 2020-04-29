using System;
using System.Collections.Generic;
using System.Linq;
using System.Text;
using System.Threading;
using System.Threading.Tasks;
using Wox.Plugin;

namespace Wox.ViewModel
{
    public class ResultsForUpdate
    {
        public List<Result> Results { get; }

        public PluginMetadata Metadata { get; }

        public Query Query { get; }
        public CancellationToken Token { get; }

        public ResultsForUpdate(List<Result> results, PluginMetadata metadata, Query query, CancellationToken token)
        {
            Results = results;
            Metadata = metadata;
            Query = query;
            Token = token;
        }

    }
}
