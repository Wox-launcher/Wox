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
    class PSOperator : IOperator
    {
        private readonly PluginInitContext _context;
        private readonly Query _query;

        public PSOperator(PluginInitContext context, Query query, string actualSearch)
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
                SubTitle = "Ctrl + Enter to open the directory in Power Shell",
                Action = c =>
                {
                    if (c.SpecialKeyState.CtrlPressed)
                    {
                        try
                        {
                            var arg = $"-NoExit -Command \"Set-Location '{item.Path.Replace("\\", "\\\\")}'\"";
                            Process.Start("powershell.exe", arg);
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

        public Result GetResult(DirectoryInfo dir, bool openByEnter)
        {
            return new Result
            {
                Title = dir.Name,
                IcoPath = dir.FullName,
                SubTitle = "Ctrl + Enter to open the directory",
                Action = c =>
                {
                    if (c.SpecialKeyState.CtrlPressed || openByEnter)
                    {
                        try
                        {
                            var arg = $"-NoExit -Command \"Set-Location '{dir.FullName.Replace("\\", "\\\\")}'\"";
                            Process.Start("powershell.exe", arg);
                            return true;
                        }
                        catch (Exception ex)
                        {
                            MessageBox.Show(ex.Message, "Could not start " + dir.FullName);
                            return false;
                        }
                    }

                    _context.API.ChangeQuery($"{_query.ActionKeyword} {dir.FullName}");
                    return false;
                }
            };
        }
    }
}
