using System.Collections.Generic;
using System.ComponentModel;
using Newtonsoft.Json;
using Wox.Infrastructure.Storage;

namespace Wox.Plugin.Everything
{
    public class Settings
    {
        public const int DefaultMaxSearchCount = 30;

        public string EditorPath { get; set; } = "";

        public List<ContextMenu> ContextMenus = new List<ContextMenu>();

        public List<IncludedFolder> IncludedFolders = new List<IncludedFolder>();

        public int MaxSearchCount { get; set; } = DefaultMaxSearchCount;

        public bool UseLocationAsWorkingDir { get; set; } = false;
    }

    public class ContextMenu
    {
        public string Name { get; set; }
        public string Command { get; set; }
        public string Argument { get; set; }
        public string ImagePath { get; set; }
    }

    public class IncludedFolder
    {
        public string Path { get; set; }
    }
}