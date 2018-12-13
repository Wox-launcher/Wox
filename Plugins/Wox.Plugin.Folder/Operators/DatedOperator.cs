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
    class DatedOperator : IOperator
    {
        private readonly PluginInitContext _context;
        private readonly Query _query;

        public DatedOperator(PluginInitContext context, Query query, string actualSearch)
        {
            _context = context;
            _query = query;
            ActualSearch = actualSearch;
        }

        public string ActualSearch { get; }

        public Result GetResult(FolderLink item)
        {
            var today = DateTime.Today;

            return new Result
            {
                Title = $"{item.Nickname}\\{today:yyyy-MM-dd}",
                IcoPath = item.Path,
                SubTitle = "Ctrl + Enter to open the directory",
                Action = c =>
                {
                    if (c.SpecialKeyState.CtrlPressed)
                    {
                        try
                        {
                            if (Directory.Exists(item.Path))
                            {
                                var datedFolderPath = Path.Combine(item.Path, today.ToString("yyyy-MM-dd"));
                                Directory.CreateDirectory(datedFolderPath);
                                Process.Start(datedFolderPath);
                            }

                            return true;
                        }
                        catch (Exception ex)
                        {
                            MessageBox.Show(ex.Message, "Could not start " + item.Path);
                            return false;
                        }
                    }

                    _context.API.ChangeQuery($"{_query.ActionKeyword} {item.Path}{(item.Path.EndsWith("\\") ? "" : "\\")}{today:yyyy-MM-dd}");
                    return false;
                },
                ContextData = item,
            };
        }

        public Result GetResult(DirectoryInfo dir)
        {
            return new Result
            {
                Title = dir.Name,
                IcoPath = dir.FullName,
                SubTitle = "Ctrl + Enter to open the directory",
                Action = c =>
                {
                    if (c.SpecialKeyState.CtrlPressed)
                    {
                        try
                        {
                            Process.Start(dir.FullName);
                            return true;
                        }
                        catch (Exception ex)
                        {
                            MessageBox.Show(ex.Message, "Could not start " + dir.FullName);
                            return false;
                        }
                    }

                    _context.API.ChangeQuery($"{_query.ActionKeyword} {dir.FullName}\\");
                    return false;
                }
            };
        }
    }
}
