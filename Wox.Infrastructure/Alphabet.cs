using System;
using System.Linq;
using NLog;
using ToolGood.Words;
using Wox.Infrastructure.UserSettings;

namespace Wox.Infrastructure
{
    public class Alphabet
    {
        private Settings _settings;

        public void Initialize()
        {
            _settings = Settings.Instance;
        }

        public string[] Translate(string content)
        {
            string[] result;
            if (_settings.ShouldUsePinyin && WordsHelper.HasChinese(content))
            {
                result = WordsHelper.GetPinyinList(content);
            }
            else
            {
                result = content.Select(c => c.ToString()).ToArray();
            }
            return result;
        }
    }
}
