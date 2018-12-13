using System;
using System.Collections.Generic;
using System.IO;
using System.Linq;
using System.Text;
using System.Threading.Tasks;

namespace Wox.Plugin.Folder.Operators
{
    interface IOperator
    {
        string ActualSearch { get; }
        Result GetResult(FolderLink item);
        Result GetResult(DirectoryInfo dir, bool openByEnter);
    }
}
