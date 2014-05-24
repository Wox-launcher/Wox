using System;
using System.Collections.Generic;
using System.Linq;
using System.Text;
using Wox.Core;
using Wox.Plugin;
using Wox.Plugins;

namespace Wox.Commands
{
    public abstract class BaseCommand
    {
        public abstract void Dispatch(Query query);

        protected void UpdateResultView(List<Result> results)
        {
            App.Window.OnUpdateResultView(results);
        }
    }
}
