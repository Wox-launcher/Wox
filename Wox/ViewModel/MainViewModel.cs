using System;
using System.Collections.Generic;
using System.Collections.Specialized;
using System.Diagnostics;
using System.Threading;
using System.Threading.Tasks;
using System.Windows;
using System.Windows.Input;
using Wox.Core.Plugin;
using Wox.Core.Resource;
using Wox.Core.UserSettings;
using Wox.Helper;
using Wox.Infrastructure;
using Wox.Infrastructure.Hotkey;
using Wox.Infrastructure.Image;
using Wox.Infrastructure.Logger;
using Wox.Infrastructure.Storage;
using Wox.Plugin;
using Wox.Storage;

namespace Wox.ViewModel
{
    public class MainViewModel : BaseModel, ISavable
    {
        #region Private Fields

        private Visibility _contextMenuVisibility;

        private bool _queryHasReturn;
        private Query _lastQuery;
        private bool _ignoreTextChange;
        private string _queryTextBeforeLoadContextMenu;
        private string _queryText;

        private readonly JsonStrorage<QueryHistory> _queryHistoryStorage;
        private readonly JsonStrorage<UserSelectedRecord> _userSelectedRecordStorage;
        private readonly JsonStrorage<TopMostRecord> _topMostRecordStorage;
        private readonly QueryHistory _queryHistory;
        private readonly UserSelectedRecord _userSelectedRecord;
        private readonly TopMostRecord _topMostRecord;
        private readonly Settings _settings;
        private readonly SettingWindowViewModel _settingVM;

        private CancellationTokenSource _updateSource;
        private CancellationToken _updateToken;
        private bool _saved;

        private readonly Internationalization _translater = InternationalizationManager.Instance;
        private readonly GlobalHotkey _globalHotkey = GlobalHotkey.Instance;

        #endregion

        #region Constructor

        public MainViewModel(SettingWindowViewModel settings)
        {
            _saved = false;
            _queryTextBeforeLoadContextMenu = "";
            _queryText = "";
            _lastQuery = new Query();

            _settingVM = settings;
            _settings = _settingVM.Settings;

            SetHotkeys();

            // happlebao todo temp fix for instance code logic
            HttpProxy.Instance.Settings = _settings;
            _translater.Settings = _settings;
            _translater.ChangeLanguage(_settings.Language);
            ThemeManager.Instance.Settings = _settings;

            _queryHistoryStorage = new JsonStrorage<QueryHistory>();
            _userSelectedRecordStorage = new JsonStrorage<UserSelectedRecord>();
            _topMostRecordStorage = new JsonStrorage<TopMostRecord>();
            _queryHistory = _queryHistoryStorage.Load();
            _userSelectedRecord = _userSelectedRecordStorage.Load();
            _topMostRecord = _topMostRecordStorage.Load();

            InitializeResultListBox();
            InitializeContextMenu();
            InitializeKeyCommands();
            RegisterResultsUpdatedEvent();
        }


        private void RegisterResultsUpdatedEvent()
        {
            foreach (var pair in PluginManager.GetPluginsForInterface<IResultUpdated>())
            {
                var plugin = (IResultUpdated)pair.Plugin;
                plugin.ResultsUpdated += (s, e) =>
                {
                    Task.Run(() =>
                    {
                        PluginManager.UpdatePluginMetadata(e.Results, pair.Metadata, e.Query);
                        UpdateResultView(e.Results, pair.Metadata, e.Query);
                    }, _updateToken);
                };
            }
        }


        private void InitializeKeyCommands()
        {
            EscCommand = new RelayCommand(_ =>
            {
                if (ContextMenuVisibility.IsVisible())
                {
                    ContextMenuVisibility = Visibility.Collapsed;
                }
                else
                {
                    MainWindowVisibility = Visibility.Collapsed;
                }
            });

            SelectNextItemCommand = new RelayCommand(_ =>
            {
                if (ContextMenuVisibility.IsVisible())
                {
                    ContextMenu.SelectNextResult();
                }
                else
                {
                    Results.SelectNextResult();
                }
            });

            SelectPrevItemCommand = new RelayCommand(_ =>
            {
                if (ContextMenuVisibility.IsVisible())
                {
                    ContextMenu.SelectPrevResult();
                }
                else
                {
                    Results.SelectPrevResult();
                }
            });


            /**
            DisplayNextQueryCommand = new RelayCommand(_ =>
            {
                var nextQuery = _queryHistory.Next();
                DisplayQueryHistory(nextQuery);
            });

            DisplayPrevQueryCommand = new RelayCommand(_ =>
            {
                var prev = _queryHistory.Previous();
                DisplayQueryHistory(prev);
            });
            **/

            SelectNextPageCommand = new RelayCommand(_ =>
            {
                Results.SelectNextPage();
            });

            SelectPrevPageCommand = new RelayCommand(_ =>
            {
                Results.SelectPrevPage();
            });

            StartHelpCommand = new RelayCommand(_ =>
            {
                Process.Start("http://doc.getwox.com");
            });

            OpenResultCommand = new RelayCommand(index =>
            {
                var results = ContextMenuVisibility.IsVisible() ? ContextMenu : Results;

                if (index != null)
                {
                    results.SelectedIndex = int.Parse(index.ToString());
                }

                var result = results.SelectedItem?.RawResult;
                if (result != null) // SelectedItem returns null if selection is empty.
                {
                    bool hideWindow = result.Action != null && result.Action(new ActionContext());

                    if (hideWindow)
                    {
                        MainWindowVisibility = Visibility.Collapsed;
                    }

                    if (!ContextMenuVisibility.IsVisible())
                    {
                        _userSelectedRecord.Add(result);
                        _queryHistory.Add(result.OriginQuery.RawQuery);
                    }
                }
            });

            LoadContextMenuCommand = new RelayCommand(_ =>
            {
                if (!ContextMenuVisibility.IsVisible())
                {
                    var result = Results.SelectedItem?.RawResult;

                    if (result != null) // SelectedItem returns null if selection is empty.
                    {
                        var id = result.PluginID;

                        var menus = PluginManager.GetContextMenusForPlugin(result);
                        menus.Add(ContextMenuTopMost(result));
                        menus.Add(ContextMenuPluginInfo(id));

                        ContextMenu.Clear();
                        Task.Run(() =>
                        {
                            ContextMenu.AddResults(menus, id);
                        }, _updateToken);
                        ContextMenuVisibility = Visibility.Visible;
                    }
                }
                else
                {
                    ContextMenuVisibility = Visibility.Collapsed;
                }
            });

        }

        private void InitializeResultListBox()
        {
            Results = new ResultsViewModel(_settings);
            ResultListBoxVisibility = Visibility.Collapsed;
        }


        private void InitializeContextMenu()
        {
            ContextMenu = new ResultsViewModel(_settings);
            ContextMenuVisibility = Visibility.Collapsed;
        }

        private void HandleQueryTextUpdated()
        {
            ProgressBarVisibility = Visibility.Hidden;
            _updateSource?.Cancel();
            _updateSource = new CancellationTokenSource();
            _updateToken = _updateSource.Token;

            if (ContextMenuVisibility.IsVisible())
            {
                QueryContextMenu();
            }
            else
            {
                string query = QueryText.Trim();
                if (!string.IsNullOrEmpty(query))
                {
                    Query(query);
                    //reset query history index after user start new query
                    ResetQueryHistoryIndex();
                }
                else
                {
                    Results.Clear();
                    ResultListBoxVisibility = Visibility.Collapsed;
                }
            }
        }

        #endregion

        #region ViewModel Properties

        public ResultsViewModel Results { get; private set; }

        public ResultsViewModel ContextMenu { get; private set; }

        public string QueryText
        {
            get { return _queryText; }
            set
            {
                _queryText = value;
                if (_ignoreTextChange)
                {
                    _ignoreTextChange = false;
                }
                else
                {
                    HandleQueryTextUpdated();
                }
            }
        }

        public bool QueryTextSelected { get; set; }

        public Visibility ContextMenuVisibility

        {
            get { return _contextMenuVisibility; }
            set
            {
                _contextMenuVisibility = value;

                _ignoreTextChange = true;
                if (!value.IsVisible())
                {
                    QueryText = _queryTextBeforeLoadContextMenu;
                    ResultListBoxVisibility = Visibility.Visible;
                }
                else
                {
                    _queryTextBeforeLoadContextMenu = QueryText;
                    QueryText = "";
                    ResultListBoxVisibility = Visibility.Collapsed;
                }
            }
        }

        public Visibility ProgressBarVisibility { get; set; }

        public Visibility ResultListBoxVisibility { get; set; }

        public Visibility MainWindowVisibility { get; set; }

        public ICommand EscCommand { get; set; }
        public ICommand SelectNextItemCommand { get; set; }
        public ICommand SelectPrevItemCommand { get; set; }
        //todo happlebao restore history command
        public ICommand DisplayNextQueryCommand { get; set; }
        public ICommand DisplayPrevQueryCommand { get; set; }
        public ICommand SelectNextPageCommand { get; set; }
        public ICommand SelectPrevPageCommand { get; set; }
        public ICommand StartHelpCommand { get; set; }
        public ICommand LoadContextMenuCommand { get; set; }
        public ICommand OpenResultCommand { get; set; }

        #endregion

        #region Private Methods

        private void QueryContextMenu()
        {
            var contextMenuId = "Context Menu Id";
            var query = QueryText.ToLower();
            if (!string.IsNullOrEmpty(query))
            {

                List<Result> filterResults = new List<Result>();
                foreach (var contextMenu in ContextMenu.Results)
                {
                    if (StringMatcher.IsMatch(contextMenu.Title, query)
                        || StringMatcher.IsMatch(contextMenu.SubTitle, query))
                    {
                        filterResults.Add(contextMenu.RawResult);
                    }
                }
                ContextMenu.Clear();
                Task.Run(() =>
                {
                    ContextMenu.AddResults(filterResults, contextMenuId);
                }, _updateToken);
            }
        }

        private void Query(string text)
        {
            _queryHasReturn = false;
            var query = PluginManager.QueryInit(text);
            if (query != null)
            {
                // handle the exclusiveness of plugin using action keyword
                string lastKeyword = _lastQuery.ActionKeyword;
                string keyword = query.ActionKeyword;
                if (string.IsNullOrEmpty(lastKeyword))
                {
                    if (!string.IsNullOrEmpty(keyword))
                    {
                        Results.RemoveResultsExcept(PluginManager.NonGlobalPlugins[keyword].Metadata);
                    }
                }
                else
                {
                    if (string.IsNullOrEmpty(keyword))
                    {
                        Results.RemoveResultsFor(PluginManager.NonGlobalPlugins[lastKeyword].Metadata);
                    }
                    else if (lastKeyword != keyword)
                    {
                        Results.RemoveResultsExcept(PluginManager.NonGlobalPlugins[keyword].Metadata);
                    }
                }

                _lastQuery = query;
                Task.Delay(200, _updateToken).ContinueWith(_ =>
                {
                    if (query.RawQuery == _lastQuery.RawQuery && !_queryHasReturn)
                    {
                        ProgressBarVisibility = Visibility.Visible;
                    }
                }, _updateToken);

                var plugins = PluginManager.ValidPluginsForQuery(query);
                Task.Run(() =>
                {
                    Parallel.ForEach(plugins, plugin =>
                    {
                        var config = _settings.PluginSettings.Plugins[plugin.Metadata.ID];
                        if (!config.Disabled)
                        {

                            var results = PluginManager.QueryForPlugin(plugin, query);
                            UpdateResultView(results, plugin.Metadata, query);
                        }
                    });
                }, _updateToken);



            }
        }

        private void ResetQueryHistoryIndex()
        {
            Results.RemoveResultsFor(QueryHistory.MetaData);
            _queryHistory.Reset();
        }
        /**
        private void DisplayQueryHistory(HistoryItem history)
        {
            if (history != null)
            {
                var historyMetadata = QueryHistory.MetaData;

                QueryText = history.Query;
                OnTextBoxSelected();

                var executeQueryHistoryTitle = _translater.GetTranslation("executeQuery");
                var lastExecuteTime = _translater.GetTranslation("lastExecuteTime");
                Results.RemoveResultsExcept(historyMetadata);
                var result = new Result
                {
                    Title = string.Format(executeQueryHistoryTitle, history.Query),
                    SubTitle = string.Format(lastExecuteTime, history.ExecutedDateTime),
                    IcoPath = "Images\\history.png",
                    PluginDirectory = Infrastructure.Constant.ProgramDirectory,
                    Action = _ =>
                    {
                        QueryText = history.Query;
                        OnTextBoxSelected();
                        return false;
                    }
                };
                Task.Run(() =>
                {
                    Results.AddResults(new List<Result> {result}, historyMetadata.ID);
                }, _updateToken);
            }
        }
        **/

        private Result ContextMenuTopMost(Result result)
        {
            Result menu;

            if (_topMostRecord.IsTopMost(result))
            {
                menu = new Result
                {
                    Title = _translater.GetTranslation("cancelTopMostInThisQuery"),
                    IcoPath = "Images\\down.png",
                    PluginDirectory = Constant.ProgramDirectory,
                    Action = _ =>
                    {
                        _topMostRecord.Remove(result);
                        App.API.ShowMsg("Succeed");
                        return false;
                    }
                };
            }
            else
            {
                menu = new Result
                {
                    Title = _translater.GetTranslation("setAsTopMostInThisQuery"),
                    IcoPath = "Images\\up.png",
                    PluginDirectory = Constant.ProgramDirectory,
                    Action = _ =>
                    {
                        _topMostRecord.AddOrUpdate(result);
                        App.API.ShowMsg("Succeed");
                        return false;
                    }
                };
            }
            return menu;
        }

        private Result ContextMenuPluginInfo(string id)
        {
            var metadata = PluginManager.GetPluginForId(id).Metadata;
            var translator = _translater;

            var author = translator.GetTranslation("author");
            var website = translator.GetTranslation("website");
            var version = translator.GetTranslation("version");
            var plugin = translator.GetTranslation("plugin");
            var title = $"{plugin}: {metadata.Name}";
            var icon = metadata.IcoPath;
            var subtitle = $"{author}: {metadata.Author}, {website}: {metadata.Website} {version}: {metadata.Version}";

            var menu = new Result
            {
                Title = title,
                IcoPath = icon,
                SubTitle = subtitle,
                PluginDirectory = metadata.PluginDirectory,
                Action = _ => false
            };
            return menu;
        }

        #endregion
        #region Hotkey

        private void SetHotkeys()
        {
            SetWoxHotkey();
            SetCustomHotkeys();
            _settingVM.HotkeyViewModel.PropertyChanged += (s, e) =>
            {
                if (e.PropertyName == nameof(HotkeyViewModel.Hotkey))
                {
                    _globalHotkey.SetHotkey(_settingVM.HotkeyViewModel.Hotkey, OnWoxHotkey);
                }
            };

            foreach (var vm in _settingVM.CustomHotkeyViewModels)
            {
                vm.PropertyChanged += (s, e) =>
                {
                    if (e.PropertyName == nameof(CustomHotkeyViewModel.CustomHotkey))
                    {
                        var k = vm.CustomHotkey;
                        _globalHotkey.SetHotkey(k.Hotkey, () => { OnCustomHotkey(k); });
                    }
                };
            }
            _settingVM.CustomHotkeyViewModels.CollectionChanged += (_, ce) =>
            {
                if (ce.Action == NotifyCollectionChangedAction.Add)
                {
                    var vm = (CustomHotkeyViewModel)ce.NewItems[0];
                    vm.PropertyChanged += (__, pe) =>
                    {
                        if (pe.PropertyName == nameof(CustomHotkeyViewModel.CustomHotkey))
                        {
                            var k = vm.CustomHotkey;
                            _globalHotkey.SetHotkey(k.Hotkey, () => { OnCustomHotkey(k); });
                        }
                    };
                }
                else if (ce.Action == NotifyCollectionChangedAction.Remove)
                {
                    var vm = (CustomHotkeyViewModel)ce.OldItems[0];
                    var k = vm.CustomHotkey;
                    _globalHotkey.RemoveHotkey(k.Hotkey);
                }
            };
        }

        private void SetWoxHotkey()
        {
            try
            {
                _globalHotkey.SetHotkey(_settingVM.HotkeyViewModel.Hotkey, OnWoxHotkey);
            }
            catch (ArgumentException e)
            {
                Log.Exception(e);
                var translation = _translater.GetTranslation("registerHotkeyFailed");
                string error = string.Format(translation, _settings.Hotkey);
                MessageBox.Show(error);
            }
        }

        private void OnWoxHotkey()
        {
            if (!IgnoreHotkey())
            {
                QueryTextSelected = true;
                ToggleWox();
            }
        }

        private void SetCustomHotkeys()
        {
            foreach (var c in _settingVM.CustomHotkeyViewModels)
            {
                try
                {
                    var k = c.CustomHotkey;
                    _globalHotkey.SetHotkey(k.Hotkey, () => { OnCustomHotkey(k); });
                }
                catch (ArgumentException e)
                {
                    Log.Exception(e);
                    var translation = _translater.GetTranslation("registerHotkeyFailed");
                    string error = string.Format(translation, _settings.Hotkey);
                    MessageBox.Show(error);
                }
            }
        }

        private void OnCustomHotkey(CustomHotkeyModel hotkey)
        {
            if (!IgnoreHotkey())
            {
                QueryText = hotkey.Query + Plugin.Query.TermSeperater;
                ToggleWox();
            }
        }

        private bool IgnoreHotkey()
        {
            var ignore = _settings.IgnoreHotkeysOnFullscreen && WindowIntelopHelper.IsWindowFullscreen();
            return ignore;
        }

        private void ToggleWox()
        {
            if (!MainWindowVisibility.IsVisible())
            {
                MainWindowVisibility = Visibility.Visible;
            }
            else
            {
                MainWindowVisibility = Visibility.Collapsed;
            }
        }

        #endregion

        #region Public Methods

        public void Save()
        {
            if (!_saved)
            {
                _queryHistoryStorage.Save();
                _userSelectedRecordStorage.Save();
                _topMostRecordStorage.Save();

                PluginManager.Save();
                ImageLoader.Save();

                _saved = true;
            }
        }

        /// <summary>
        /// To avoid deadlock, this method should not called from main thread
        /// </summary>
        public void UpdateResultView(List<Result> list, PluginMetadata metadata, Query originQuery)
        {
            _queryHasReturn = true;
            ProgressBarVisibility = Visibility.Hidden;

            foreach (var result in list)
            {
                if (_topMostRecord.IsTopMost(result))
                {
                    result.Score = int.MaxValue;
                }
                else
                {
                    result.Score += _userSelectedRecord.GetSelectedCount(result) * 5;
                }
            }

            if (originQuery.RawQuery == _lastQuery.RawQuery)
            {
                Results.AddResults(list, metadata.ID);
            }

            if (list.Count > 0 && !ResultListBoxVisibility.IsVisible())
            {
                ResultListBoxVisibility = Visibility.Visible;
            }
        }

        #endregion
    }
}