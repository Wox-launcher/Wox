using System;
using System.Collections.Generic;
using System.Diagnostics;
using System.IO;
using System.Linq;
using System.Text;
using System.Threading.Tasks;
using System.Windows;

namespace Wox.Plugin.Folder.Operators
{
    class DefaultOperator : IOperator
    {
        private readonly PluginInitContext _context;
        private readonly Query _query;

        public DefaultOperator(PluginInitContext context, Query query, string actualSearch)
        {
            _context = context;
            _query = query;
            ActualSearch = actualSearch;
        }

        public string ActualSearch { get; }
        public Result GetResult(FolderLink item)
        {
            return new Result
            {
                Title = item.Nickname,
                IcoPath = item.Path,
                SubTitle = "Ctrl + Enter to open the directory",
                Action = c =>
                {
                    if (c.SpecialKeyState.CtrlPressed)
                    {
                        try
                        {
                            Process.Start(item.Path);
                            return true;
                        }
                        catch (Exception ex)
                        {
                            MessageBox.Show(ex.Message, "Could not start " + item.Path);
                            return false;
                        }
                    }

                    _context.API.ChangeQuery($"{_query.ActionKeyword} {item.Path}{(item.Path.EndsWith("\\") ? "" : "\\")}");
                    return false;
                },
                ContextData = item,
            };
        }
    }
}
