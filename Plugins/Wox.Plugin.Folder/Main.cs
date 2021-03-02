using System;
using System.Collections.Generic;
using System.Diagnostics;
using System.IO;
using System.Linq;
using System.Windows;
using System.Windows.Controls;
using Wox.Infrastructure;
using Wox.Infrastructure.Storage;

namespace Wox.Plugin.Folder
{
    public class Main : IPlugin, ISettingProvider, IPluginI18n, ISavable, IContextMenu
    {
        public const string FolderImagePath = "Images\\folder.png";
        public const string FileImagePath = "Images\\file.png";
        public const string DeleteFileFolderImagePath = "Images\\deletefilefolder.png";
        public const string CopyImagePath = "Images\\copy.png";

        private string DefaultFolderSubtitleString = "Ctrl + Enter to open the directory";

        private static List<string> _driverNames;
        private PluginInitContext _context;

        private readonly Settings _settings;
        private readonly PluginJsonStorage<Settings> _storage;
        private IContextMenu _contextMenuLoader;

        public Main()
        {
            _storage = new PluginJsonStorage<Settings>();
            _settings = _storage.Load();
        }

        public void Save()
        {
            _storage.Save();
        }

        public Control CreateSettingPanel()
        {
            return new FileSystemSettings(_context.API, _settings);
        }

        public void Init(PluginInitContext context)
        {
            _context = context;
            _contextMenuLoader = new ContextMenuLoader(context);
            InitialDriverList();
        }

        public List<Result> Query(Query query)
        {
            var inputs = query.MultilingualSearch;
            var results = new List<Result>();
            foreach (var input in inputs)
            {
                results.AddRange(GetUserFolderResults(query, input));

                string search = query.Search.ToLower();
                if (!IsDriveOrSharedFolder(input))
                    return results;

                results.AddRange(QueryInternal_Directory_Exists(query, input));
            }

            // todo why was this hack here?
            foreach (var result in results)
            {
                result.Score += 10;
            }

            return results;
        }

        private static bool IsDriveOrSharedFolder(string search)
        {
            if (search.StartsWith(@"\\"))
            { // share folder
                return true;
            }

            if (_driverNames != null && _driverNames.Any(search.StartsWith))
            { // normal drive letter
                return true;
            }

            if (_driverNames == null && search.Length > 2 && char.IsLetter(search[0]) && search[1] == ':')
            { // when we don't have the drive letters we can try...
                return true; // we don't know so let's give it the possibility
            }

            return false;
        }

        private Result CreateFolderResult(string title, string subtitle, string path, Query query, string input)
        {
            return new Result
            {
                Title = title,
                IcoPath = path,
                SubTitle = subtitle,
                TitleHighlightData = StringMatcher.FuzzySearch(input, title).MatchData,
                Action = c =>
                {
                    if (c.SpecialKeyState.CtrlPressed)
                    {
                        try
                        {
                            Process.Start(path);
                            return true;
                        }
                        catch (Exception ex)
                        {
                            MessageBox.Show(ex.Message, "Could not start " + path);
                            return false;
                        }
                    }

                    string changeTo = path.EndsWith("\\") ? path : path + "\\";
                    _context.API.ChangeQuery(string.IsNullOrEmpty(query.ActionKeyword) ?
                        changeTo :
                        query.ActionKeyword + " " + changeTo);
                    return false;
                },
                ContextData = new SearchResult { Type = ResultType.Folder, FullPath = path }
            };
        }

        private List<Result> GetUserFolderResults(Query query, string input)
        {
            var userFolderLinks = _settings.FolderLinks.Where(
                x => x.Nickname.StartsWith(input, StringComparison.OrdinalIgnoreCase));
            var results = userFolderLinks.Select(item =>
                CreateFolderResult(item.Nickname, DefaultFolderSubtitleString, item.Path, query, input)).ToList();
            return results;
        }

        private void InitialDriverList()
        {
            if (_driverNames == null)
            {
                _driverNames = new List<string>();
                var allDrives = DriveInfo.GetDrives();
                foreach (DriveInfo driver in allDrives)
                {
                    _driverNames.Add(driver.Name.ToLower().TrimEnd('\\'));
                }
            }
        }

        private static readonly char[] _specialSearchChars = new char[]
        {
            '?', '*', '>'
        };

        private List<Result> QueryInternal_Directory_Exists(Query query, string input)
        {
            var results = new List<Result>();
            var hasSpecial = input.IndexOfAny(_specialSearchChars) >= 0;
            string incompleteName = "";
            if (hasSpecial || !Directory.Exists(input + "\\"))
            {
                // if folder doesn't exist, we want to take the last part and use it afterwards to help the user 
                // find the right folder.
                int index = input.LastIndexOf('\\');
                if (index > 0 && index < (input.Length - 1))
                {
                    incompleteName = input.Substring(index + 1).ToLower();
                    input = input.Substring(0, index + 1);
                    if (!Directory.Exists(input))
                    {
                        return results;
                    }
                }
                else
                {
                    return results;
                }
            }
            else
            {
                // folder exist, add \ at the end of doesn't exist
                if (!input.EndsWith("\\"))
                {
                    input += "\\";
                }
            }

            results.Add(CreateOpenCurrentFolderResult(incompleteName, input));

            var searchOption = SearchOption.TopDirectoryOnly;
            incompleteName += "*";

            // give the ability to search all folder when starting with >
            if (incompleteName.StartsWith(">"))
            {
                searchOption = SearchOption.AllDirectories;

                // match everything before and after search term using supported wildcard '*', ie. *searchterm*
                incompleteName = "*" + incompleteName.Substring(1);
            }

            var folderList = new List<Result>();
            var fileList = new List<Result>();

            var folderSubtitleString = DefaultFolderSubtitleString;

            try
            {
                // search folder and add results
                var directoryInfo = new DirectoryInfo(input);
                var fileSystemInfos = directoryInfo.GetFileSystemInfos(incompleteName, searchOption);

                foreach (var fileSystemInfo in fileSystemInfos)
                {
                    if ((fileSystemInfo.Attributes & FileAttributes.Hidden) == FileAttributes.Hidden) continue;

                    if(fileSystemInfo is DirectoryInfo)
                    {
                        if (searchOption == SearchOption.AllDirectories)
                            folderSubtitleString = fileSystemInfo.FullName;

                        folderList.Add(CreateFolderResult(fileSystemInfo.Name, folderSubtitleString, fileSystemInfo.FullName, query, input));
                    }
                    else
                    {
                        fileList.Add(CreateFileResult(fileSystemInfo.FullName, input));
                    }
                }
            }
            catch (Exception e)
            {
                if (e is UnauthorizedAccessException || e is ArgumentException)
                {
                    results.Add(new Result { Title = e.Message, Score = 501 });

                    return results;
                }

                throw;
            }

            // Intial ordering, this order can be updated later by UpdateResultView.MainViewModel based on history of user selection.
            return results.Concat(folderList.OrderBy(x => x.Title)).Concat(fileList.OrderBy(x => x.Title)).ToList();
        }

        private static Result CreateFileResult(string filePath, string query)
        {
            var result = new Result
            {
                Title = Path.GetFileName(filePath),
                SubTitle = filePath,
                IcoPath = filePath,
                TitleHighlightData = StringMatcher.FuzzySearch(query, Path.GetFileName(filePath)).MatchData,
                Action = c =>
                {
                    try
                    {
                        Process.Start(filePath);
                    }
                    catch (Exception ex)
                    {
                        MessageBox.Show(ex.Message, "Could not start " + filePath);
                    }

                    return true;
                },
                ContextData = new SearchResult { Type = ResultType.File, FullPath = filePath}
            };
            return result;
        }

        private static Result CreateOpenCurrentFolderResult(string incompleteName, string search)
        {
            var firstResult = "Open current directory";
            if (incompleteName.Length > 0)
                firstResult = "Open " + search;

            var folderName = search.TrimEnd('\\').Split(new[] { Path.DirectorySeparatorChar }, StringSplitOptions.None).Last();

            return new Result
            {
                Title = firstResult,
                SubTitle = $"Use > to search files and subfolders within {folderName}, " +
                                $"* to search for file extensions in {folderName} or both >* to combine the search",
                IcoPath = search,
                Score = 500,
                Action = c =>
                {
                    Process.Start(search);
                    return true;
                }
            };
        }

        public string GetTranslatedPluginTitle()
        {
            return _context.API.GetTranslation("wox_plugin_folder_plugin_name");
        }

        public string GetTranslatedPluginDescription()
        {
            return _context.API.GetTranslation("wox_plugin_folder_plugin_description");
        }

        public List<Result> LoadContextMenus(Result selectedResult)
        {
            return _contextMenuLoader.LoadContextMenus(selectedResult);
        }
    }
}