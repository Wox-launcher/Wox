using System.Collections.Generic;
using Wox.Core;

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
