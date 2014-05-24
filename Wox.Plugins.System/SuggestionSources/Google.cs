﻿using System;
using System.Collections.Generic;
using System.IO;
using System.Linq;
using Newtonsoft.Json;
using Newtonsoft.Json.Linq;
using Wox.Core;

namespace Wox.Plugins.System.SuggestionSources
{
    public class Google : AbstractSuggestionSource
    {
        public override List<string> GetSuggestions(string query)
        {
            try
            {
                var response =
                    HttpRequest.CreateGetHttpResponse(
                        "https://www.google.com/complete/search?output=chrome&q=" + Uri.EscapeUriString(query), null,
                        null, null);
                var stream = response.GetResponseStream();

                if (stream != null)
                {
                    var body = new StreamReader(stream).ReadToEnd();
                    var json = JsonConvert.DeserializeObject(body) as JContainer;
                    if (json != null)
                    {
                        var results = json[1] as JContainer;
                        if (results != null)
                        {
                            return results.OfType<JValue>().Select(o => o.Value).OfType<string>().ToList();
                        }
                    }
                }
            }
            catch
            { }
            
            return null;
        }
    }
}
