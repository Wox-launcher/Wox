namespace Wox.Plugin
{
    public static class AllowedLanguage
    {
        public static string Python
        {
            get { return "PYTHON"; }
        }

        public static string CSharp
        {
            get { return "CSHARP"; }
        }

        public static string Executable
        {
            get { return "EXECUTABLE"; }
        }

        public static bool IsAllowed(string language)
        {
            var upper = language.ToUpper();
            return upper == Python.ToUpper()
                || upper == CSharp.ToUpper()
                || upper == Executable.ToUpper();
        }
    }
}