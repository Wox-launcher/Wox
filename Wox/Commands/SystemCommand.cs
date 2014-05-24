using System.Linq;
using System.Threading;
using Wox.Core;
using Wox.Plugins;

namespace Wox.Commands
{
    public class SystemCommand : BaseCommand
    {
        public override void Dispatch(Query query)
        {
            // todo: workaround
            if (query.IsEmpty()) return;

            foreach (PluginPair pair in PluginLoader.Plugins.AllPlugins.Where(o => o.Metadata.PluginType == PluginType.System))
            {
                PluginPair pair1 = pair;
                ThreadPool.QueueUserWorkItem(state =>
                {
                    pair1.InitContext.PushResults = (q, r) =>
                    {
                        if (r == null || r.Count == 0) return;
                        foreach (Result result in r)
                        {
                            result.PluginDirectory = pair1.Metadata.PluginDirecotry;
                            result.OriginQuery = q;
                            result.AutoAjustScore = true;
                        }
                        UpdateResultView(r);
                    };

                    if(pair1.Plugin.IsAvailable(query))
                    {
                        var results = pair1.Plugin.Query(query);
                        pair1.InitContext.PushResults(query, results);
                    }
                });
            }
        }
    }
}
