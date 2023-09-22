using System.Globalization;

namespace Wox.Core.I18n;

public class Language
{
    public Language(string code, string display)
    {
        Code = code;
        Display = display;
    }

    /// <summary>
    ///     for example: en_US, zh_CN
    /// </summary>
    public string Code { get; set; }

    public string Display { get; set; }
}

public class Languages
{
    public static Language English = new("en_US", "English");
    public static Language Chinese = new("zh_CN", "简体中文");
    public static Language French = new("fr_FR", "Français");
    public static Language German = new("de_DE", "Deutsch");
    public static Language Japanese = new("ja_JP", "日本語");
    public static Language Korean = new("ko_KR", "한국어");
    public static Language Portuguese = new("pt_BR", "Português");
    public static Language Russian = new("ru_RU", "Русский");
    public static Language Spanish = new("es_ES", "Español");
    public static Language Ukrainian = new("uk_UA", "Українська");
    public static Language Chinese_TW = new("zh_TW", "繁體中文");
    public static Language Danish = new("da_DK", "Dansk");
    public static Language Dutch = new("nl_NL", "Nederlands");
}