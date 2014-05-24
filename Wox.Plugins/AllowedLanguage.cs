﻿namespace Wox.Plugins
{
    public static class AllowedLanguage
    {
        public static string Python
        {
            get { return "python"; }
        }

        public static string CSharp
        {
            get { return "csharp"; }
        }

        public static bool IsAllowed(string language)
        {
            return language.ToUpper() == Python.ToUpper() || language.ToUpper() == CSharp.ToUpper();
        }
    }
}