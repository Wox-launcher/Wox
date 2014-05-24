using System;
using System.Collections.Generic;
using System.Diagnostics;

namespace Wox.Plugin.SystemPlugins
{
    public class UrlPlugin : BaseSystemPlugin
    {
        public override bool IsAvailable(Query query)
        {
            Uri uri;
            return Uri.TryCreate(query.Raw, UriKind.Absolute, out uri);
        }

        public override List<Result> Query(Query query)
        {
            var raw = query.Raw;
            return new List<Result>
            {
                new Result
                {
                    Title = raw,
                    SubTitle = "Open the typed URL...",
                    IcoPath = "Images/url1.png",
                    Score = 8,
                    Action = _ =>
                    {
                        Process.Start(raw);
                        return true;
                    }
                }
            };
        }

        public override string Name { get { return "URL handler"; } }
        public override string Description { get { return "Open the typed URL..."; } }
        public override string IcoPath { get { return "Images/url2.png"; } }

        public override void Init(PluginInitContext context)
        {
        }
    }
}