using System;
using System.Collections.Generic;
using System.Linq;
using System.Text;
using System.Threading.Tasks;

namespace Wox.Plugin.Folder.Operators
{
    interface IOperator
    {
        string ActualSearch { get; }
        Result GetResult(FolderLink item);
    }
}
