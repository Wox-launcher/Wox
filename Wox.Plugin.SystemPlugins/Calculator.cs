using System;
using System.Collections.Generic;
using System.Windows.Forms;
using YAMP;

namespace Wox.Plugin.SystemPlugins
{
    public class Calculator : BaseSystemPlugin
    {
        private static ParseContext yampContext = null;
        private QueryContext queryContext;
        private PluginInitContext context { get; set; }

        static Calculator()
        {
            yampContext = Parser.PrimaryContext;
            Parser.InteractiveMode = false;
            Parser.UseScripting = false;
        }

        public override bool IsAvailable(Query query)
        {
            try
            {
                queryContext = yampContext.Run(query.Raw);
                return queryContext.Output != null &&
                       !string.IsNullOrEmpty(queryContext.Result);
            }
            catch (Exception exception)
            {
                // todo: log
                return false;
            }
        }

        public override List<Result> Query(Query query)
        {
            return new List<Result>
            {
                new Result
                {
                    Title = queryContext.Result,
                    IcoPath = "Images/calculator.png",
                    Score = 300,
                    SubTitle = "Copy this number to the clipboard",
                    Action = _ =>
                    {
                        Clipboard.SetText(queryContext.Result);
                        return true;
                    }
                }
            };
        }

        public override void Init(PluginInitContext context)
        {
            this.context = context;
        }

        public override string Name
        {
            get { return "Calculator"; }
        }

        public override string IcoPath
        {
            get { return @"Images\calculator.png"; }
        }
    }
}
