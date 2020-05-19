using System;
using System.Collections.Specialized;
using System.Diagnostics;
using System.Linq;
using System.Runtime.Caching;
using NLog;
using ToolGood.Words;
using Wox.Infrastructure.Logger;
using Wox.Infrastructure.UserSettings;

namespace Wox.Infrastructure
{
    public class Alphabet
    {
        private Settings _settings;
        private MemoryCache _cache;
        
        private static readonly NLog.Logger Logger = LogManager.GetCurrentClassLogger();
        private static int count = 0;

        public void Initialize()
        {
            _settings = Settings.Instance;
            NameValueCollection config = new NameValueCollection();
            config.Add("pollingInterval", "00:05:00");
            config.Add("physicalMemoryLimitPercentage", "1");
            config.Add("cacheMemoryLimitMegabytes", "30");
            _cache = new MemoryCache("AlphabetCache", config);
        }

        public string Translate(string content)
        {
            if (_settings.ShouldUsePinyin)
            {
                string result = _cache[content] as string;
                if (result == null)
                {
                    if (count == 50)
                    {
                        // https://github.com/toolgood/ToolGood.Words/issues/53
                        GC.Collect();
                    }

                    if (WordsHelper.HasChinese(content))
                    {
                        // todo change first pinyin to full pinyin list, but current fuzzy match algorithm won't support first char match
                        result = WordsHelper.GetFirstPinyin(content);
                        if (count < 50)
                        {
                            count += 1;
                        }
                    }
                    else
                    {
                        result = content;
                    }
                    CacheItemPolicy policy = new CacheItemPolicy();
                    policy.SlidingExpiration = new TimeSpan(12, 0, 0);
                    _cache.Set(content, result, policy);
                }
                return result;
            }
            else
            {
                return content;
            }
        }
    }
}
