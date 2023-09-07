namespace Wox.Plugin;

/// <summary>
///     Allowed plugin languages
/// </summary>
public static class AllowedLanguage
{
    public static string Python => "PYTHON";

    public static string CSharp => "CSHARP";

    public static string Nodejs => "NODEJS";

    public static bool IsAllowed(string language)
    {
        return language.ToUpper() == Python.ToUpper()
               || language.ToUpper() == CSharp.ToUpper()
               || language.ToUpper() == Nodejs.ToUpper();
    }
}