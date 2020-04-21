using System;

namespace Wox.Plugin.Program
{
    public class IgnoredEntry
    {
        public string EntryString { get; set; }
        public bool IsRegex { get; set; }

        public override string ToString()
        {
            return String.Format("{0} {1}", EntryString, IsRegex ? "(regex)" : "");
        }
    }
}