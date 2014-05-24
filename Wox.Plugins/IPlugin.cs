using System.Collections.Generic;
using Wox.Core;

namespace Wox.Plugins
{
    public interface IPlugin
    {
        bool IsAvailable(Query query);
        List<Result> Query(Query query);

        // todo: inject context via .ctor
        void Init(PluginInitContext context); 
    }
}