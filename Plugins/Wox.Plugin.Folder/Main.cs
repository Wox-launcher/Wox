﻿using System;
using System.Collections.Generic;
using System.Diagnostics;
using System.IO;
using System.Linq;
using System.Windows;
using System.Windows.Controls;
using Wox.Infrastructure.Storage;

namespace Wox.Plugin.Folder
{
    public class Main : IPlugin, ISettingProvider, IPluginI18n, ISavable
    {
        private static List<string> _driverNames;
        private PluginInitContext _context;

        private readonly Settings _settings;
        private readonly PluginJsonStorage<Settings> _storage;

        public Main()
        {
            _storage = new PluginJsonStorage<Settings>();
            _settings = _storage.Load();
            InitialDriverList();
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
            
        }

        public List<Result> Query(Query query)
        {
            var op = Operators.OperatorFactory.GetOperator(_context, query);

            List<FolderLink> userFolderLinks = _settings.FolderLinks.Where(
                x => x.Nickname.StartsWith(op.ActualSearch, StringComparison.OrdinalIgnoreCase)).ToList();
            List<Result> results =
                userFolderLinks.Select(op.GetResult).ToList();

            if (_driverNames != null && !_driverNames.Any(op.ActualSearch.StartsWith))
                return results;

            //if (!input.EndsWith("\\"))
            //{
            //    //"c:" means "the current directory on the C drive" whereas @"c:\" means "root of the C drive"
            //    input = input + "\\";
            //}
            results.AddRange(QueryInternal_Directory_Exists(query));

            // todo temp hack for scores
            foreach (var result in results)
            {
                result.Score += 10;
            }

            return results;
        }
        private void InitialDriverList()
        {
            if (_driverNames == null)
            {
                _driverNames = new List<string>();
                DriveInfo[] allDrives = DriveInfo.GetDrives();
                foreach (DriveInfo driver in allDrives)
                {
                    _driverNames.Add(driver.Name.ToLower().TrimEnd('\\'));
                }
            }
        }

        private List<Result> QueryInternal_Directory_Exists(Query query)
        {
            var op = Operators.OperatorFactory.GetOperator(_context, query);
            var search = op.ActualSearch;

            var results = new List<Result>();

            string incompleteName = "";
            if (!Directory.Exists(op.ActualSearch + "\\"))
            {
                //if the last component of the path is incomplete,
                //then make auto complete for it.
                int index = search.LastIndexOf('\\');
                if (index > 0 && index < (search.Length - 1))
                {
                    incompleteName = search.Substring(index + 1);
                    incompleteName = incompleteName.ToLower();
                    search = search.Substring(0, index + 1);
                    if (!Directory.Exists(search))
                        return results;
                }
                else
                    return results;
            }
            else
            {
                if (!search.EndsWith("\\"))
                    search += "\\";
            }

            string firstResult = "Open current directory";
            if (incompleteName.Length > 0)
            {
                firstResult = "Open " + search;

                results.Add(new Result
                {
                    Title = firstResult,
                    IcoPath = search,
                    Score = 10000,
                    Action = c =>
                    {
                        Process.Start(search);
                        return true;
                    }
                });
            }
            else
            {
                var result = op.GetResult(new DirectoryInfo(search));
                result.Score = 10000;
                results.Add(result);

            }
            

            //Add children directories
            DirectoryInfo[] dirs = new DirectoryInfo(search).GetDirectories();
            foreach (DirectoryInfo dir in dirs)
            {
                if ((dir.Attributes & FileAttributes.Hidden) == FileAttributes.Hidden) continue;

                if (incompleteName.Length != 0 && !dir.Name.ToLower().StartsWith(incompleteName))
                    continue;
                DirectoryInfo dirCopy = dir;
                results.Add(op.GetResult(dirCopy));
            }

            //Add children files
            FileInfo[] files = new DirectoryInfo(search).GetFiles();
            foreach (FileInfo file in files)
            {
                if ((file.Attributes & FileAttributes.Hidden) == FileAttributes.Hidden) continue;
                if (incompleteName.Length != 0 && !file.Name.ToLower().StartsWith(incompleteName))
                    continue;
                string filePath = file.FullName;
                var result = new Result
                {
                    Title = Path.GetFileName(filePath),
                    IcoPath = filePath,
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
                    }
                };

                results.Add(result);
            }

            return results;
        }

        public string GetTranslatedPluginTitle()
        {
            return _context.API.GetTranslation("wox_plugin_folder_plugin_name");
        }

        public string GetTranslatedPluginDescription()
        {
            return _context.API.GetTranslation("wox_plugin_folder_plugin_description");
        }
    }
}