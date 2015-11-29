using System.Collections.Generic;

namespace Wox.Core.i18n
{
    internal static class AvailableLanguages
    {
        public static Language English = new Language("en", "English");
        public static Language Chinese = new Language("zh-cn", "中文");
        public static Language Chinese_TW = new Language("zh-tw", "中文（繁体）");
        public static Language Russian = new Language("ru", "Русский");
        public static Language French = new Language("fr", "Français");

        public static List<Language> GetAvailableLanguages()
        {
            List<Language> languages = new List<Language>
            {
                English, 
                Chinese, 
                Chinese_TW,
                Russian,
                French,
            };
            return languages;
        }
    }
}