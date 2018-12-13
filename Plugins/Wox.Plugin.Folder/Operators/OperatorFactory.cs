using System;
using System.Collections.Generic;
using System.Linq;
using System.Text;
using System.Threading.Tasks;

namespace Wox.Plugin.Folder.Operators
{
    class OperatorFactory
    {
        public static IOperator GetOperator(PluginInitContext context, Query query)
        {
            var search = query.Search.ToLower();
            if (!string.IsNullOrEmpty(search))
            {
                var strs = search.Split('|');
                var opSelector = strs.Length == 2 ? strs[1] : null;
                var actualSearch = strs[0];
                switch (opSelector)
                {
                    case "d":
                        return new DatedOperator(context, query, actualSearch);
                    case "ps":
                        return new PSOperator(context, query, actualSearch);
                    case "cmd":
                        return new CMDOperator(context, query, actualSearch);
                }
            }

            return new DefaultOperator(context, query, search);
        }
    }
}
