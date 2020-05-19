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

        public string Translate(string content)
        {
            string result;
            if (_settings.ShouldUsePinyin && WordsHelper.HasChinese(content))
            {
                // todo change first pinyin to full pinyin list, but current fuzzy match algorithm won't support first char match
                result = WordsHelper.GetFirstPinyin(content);
            }
            else
            {
                result = content;
            }
            return result;
        }
    }
}
