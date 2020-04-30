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
        public string ID { get; }

        public Query Query { get; }
        public CancellationToken Token { get; }
        public CountdownEvent Countdown { get; }

        public ResultsForUpdate(List<Result> results, string resultID, CancellationToken token)
        {
            Results = results;
            ID = resultID;
            Token = token;
        }


        public ResultsForUpdate(List<Result> results, PluginMetadata metadata, Query query, CancellationToken token, CountdownEvent countdown)
        {
            Results = results;
            Metadata = metadata;
            Query = query;
            Token = token;
            Countdown = countdown;
            ID = metadata.ID;
        }

    }
}
