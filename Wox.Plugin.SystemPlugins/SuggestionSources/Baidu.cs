﻿using System;
using System.Collections.Generic;
using System.IO;
using System.Linq;
using System.Text;
using System.Text.RegularExpressions;
using System.Xml;
using Newtonsoft.Json;
using Newtonsoft.Json.Linq;
using Newtonsoft.Json.Serialization;
using Wox.Infrastructure;
using Wox.Infrastructure.Http;
using YAMP.Numerics;

namespace Wox.Plugin.SystemPlugins.SuggestionSources
{
    public class Baidu : AbstractSuggestionSource
    {
        Regex reg = new Regex("window.baidu.sug\\((.*)\\)");

        public override List<string> GetSuggestions(string query)
        {
            var result = HttpRequest.Get("http://suggestion.baidu.com/su?json=1&wd=" + Uri.EscapeUriString(query), "GB2312");
            if (string.IsNullOrEmpty(result)) return new List<string>();

            Match match = reg.Match(result);
            if (match.Success)
            {
                JContainer json = null;
                try
                {
                    json =JsonConvert.DeserializeObject(match.Groups[1].Value) as JContainer;
                }
                catch{}

                if (json != null)
                {
                    var results = json["s"] as JArray;
                    if (results != null)
                    {
                        return results.OfType<JValue>().Select(o => o.Value).OfType<string>().ToList();
                    }
                }
            }

            return new List<string>();
        }
    }
}
