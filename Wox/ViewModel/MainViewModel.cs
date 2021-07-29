using System;
using System.Collections.Generic;
using System.ComponentModel;
using System.Diagnostics;
using System.Linq;
using System.Threading;
using System.Threading.Tasks;
using System.Windows;
using System.Windows.Input;
using System.Collections.Concurrent;

using NHotkey;
using NHotkey.Wpf;
using NLog;

using Wox.Core.Plugin;
using Wox.Core.Resource;
using Wox.Helper;
using Wox.Infrastructure;
using Wox.Infrastructure.Hotkey;
using Wox.Infrastructure.Logger;
using Wox.Infrastructure.Storage;
using Wox.Infrastructure.UserSettings;
using Wox.Plugin;
using Wox.Storage;

namespace Wox.ViewModel
{
    public class MainViewModel : BaseModel, ISavable
    {
        #region Private Fields

        private Query _lastQuery;
        private string _queryTextBeforeLeaveResults;

        private readonly WoxJsonStorage<History> _historyItemsStorage;
        private readonly WoxJsonStorage<UserSelectedRecord> _userSelectedRecordStorage;
        private readonly WoxJsonStorage<TopMostRecord> _topMostRecordStorage;
        private readonly Settings _settings;
        private readonly History _history;
        private readonly UserSelectedRecord _userSelectedRecord;
        private readonly TopMostRecord _topMostRecord;
        private BlockingCollection<ResultsForUpdate> _resultsQueue;

        private CancellationTokenSource _updateSource;
        private bool _saved;

        private readonly Internationalization _translator;

        private static readonly Logger Logger = LogManager.GetCurrentClassLogger();

        #endregion

        #region Constructor

        public MainViewModel(bool useUI = true)
        {
            _saved = false;
            _queryTextBeforeLeaveResults = "";
            _queryText = "";
            _lastQuery = new Query();

            _settings = Settings.Instance;

            _historyItemsStorage = new WoxJsonStorage<History>();
            _userSelectedRecordStorage = new WoxJsonStorage<UserSelectedRecord>();
            _topMostRecordStorage = new WoxJsonStorage<TopMostRecord>();
            _history = _historyItemsStorage.Load();
            _userSelectedRecord = _userSelectedRecordStorage.Load();
            _topMostRecord = _topMostRecordStorage.Load();

            ContextMenu = new ResultsViewModel(_settings);
            Results = new ResultsViewModel(_settings);
            History = new ResultsViewModel(_settings);
            _selectedResults = Results;

            if (useUI)
            {
                _translator = InternationalizationManager.Instance;
                InitializeKeyCommands();
                RegisterResultsUpdatedEvent();

                SetHotkey(_settings.Hotkey, OnHotkey);
                SetCustomPluginHotkey();
            }

            RegisterResultConsume();
        }

        private void RegisterResultConsume()
        {
            _resultsQueue = new BlockingCollection<ResultsForUpdate>();
            Task.Run(() =>
            {
                while (true)
                {
                    ResultsForUpdate first = _resultsQueue.Take();
                    List<ResultsForUpdate> updates = new List<ResultsForUpdate>() { first };

                    DateTime startTime = DateTime.Now;
                    int timeout = 50;
                    DateTime takeExpired = startTime.AddMilliseconds(timeout / 10);

                    ResultsForUpdate tempUpdate;
                    while (_resultsQueue.TryTake(out tempUpdate) && DateTime.Now < takeExpired)
                    {
                        updates.Add(tempUpdate);
                    }


                    UpdateResultView(updates);

                    DateTime currentTime = DateTime.Now;
                    Logger.WoxTrace($"start {startTime.Millisecond} end {currentTime.Millisecond}");
                    foreach (var update in updates)
                    {
                        Logger.WoxTrace($"update name:{update.Metadata.Name} count:{update.Results.Count} query:{update.Query} token:{update.Token.IsCancellationRequested}");
                        update.Countdown.Signal();
                    }
                    DateTime viewExpired = startTime.AddMilliseconds(timeout);
                    if (currentTime < viewExpired)
                    {
                        TimeSpan span = viewExpired - currentTime;
                        Logger.WoxTrace($"expired { viewExpired.Millisecond} span {span.TotalMilliseconds}");
                        Thread.Sleep(span);
                    }
                }
            }).ContinueWith(ErrorReporting.UnhandledExceptionHandleTask, TaskContinuationOptions.OnlyOnFaulted);
        }

        private void RegisterResultsUpdatedEvent()
        {
            foreach (var pair in PluginManager.GetPluginsForInterface<IResultUpdated>())
            {
                var plugin = (IResultUpdated)pair.Plugin;
                plugin.ResultsUpdated += (s, e) =>
                {
                    if (!_updateSource.IsCancellationRequested)
                    {
                        CancellationToken token = _updateSource.Token;
                        // todo async update don't need count down
                        // init with 1 since every ResultsForUpdate will be countdown.signal()
                        CountdownEvent countdown = new CountdownEvent(1);
                        Task.Run(() =>
                        {
                            if (token.IsCancellationRequested) { return; }
                            PluginManager.UpdatePluginMetadata(e.Results, pair.Metadata, e.Query);
                            _resultsQueue.Add(new ResultsForUpdate(e.Results, pair.Metadata, e.Query, token, countdown));
                        }, token);
                    }
                };
            }
        }


        private void InitializeKeyCommands()
        {
            EscCommand = new RelayCommand(_ =>
            {
                if (!SelectedIsFromQueryResults())
                {
                    SelectedResults = Results;
                }
                else
                {
                    MainWindowVisibility = Visibility.Collapsed;
                }
            });

            SelectNextItemCommand = new RelayCommand(_ =>
            {
                SelectedResults.SelectNextResult();
            });

            SelectPrevItemCommand = new RelayCommand(_ =>
            {
                SelectedResults.SelectPrevResult();
            });

            SelectNextPageCommand = new RelayCommand(_ =>
            {
                SelectedResults.SelectNextPage();
            });

            SelectPrevPageCommand = new RelayCommand(_ =>
            {
                SelectedResults.SelectPrevPage();
            });

            SelectFirstResultCommand = new RelayCommand(_ => SelectedResults.SelectFirstResult());

            StartHelpCommand = new RelayCommand(_ =>
            {
                Process.Start("http://doc.wox.one/");
            });

            RefreshCommand = new RelayCommand(_ => Refresh());

            OpenResultCommand = new RelayCommand(index =>
            {
                var results = SelectedResults;

                if (index != null)
                {
                    results.SelectedIndex = int.Parse(index.ToString());
                }

                var result = results.SelectedItem?.Result;
                if (result != null) // SelectedItem returns null if selection is empty.
                {
                    bool hideWindow = result.Action != null && result.Action(new ActionContext
                    {
                        SpecialKeyState = GlobalHotkey.Instance.CheckModifiers()
                    });

                    if (hideWindow)
                    {
                        MainWindowVisibility = Visibility.Collapsed;
                    }

                    if (SelectedIsFromQueryResults())
                    {
                        _userSelectedRecord.Add(result);
                        _history.Add(result.OriginQuery.RawQuery);
                    }
                    else
                    {
                        SelectedResults = Results;
                    }
                }
            });

            LoadContextMenuCommand = new RelayCommand(_ =>
            {
                if (SelectedIsFromQueryResults())
                {
                    SelectedResults = ContextMenu;
                }
                else
                {
                    SelectedResults = Results;
                }
            });

            LoadHistoryCommand = new RelayCommand(_ =>
            {
                if (SelectedIsFromQueryResults())
                {
                    SelectedResults = History;
                    History.SelectedIndex = _history.Items.Count - 1;
                }
                else
                {
                    SelectedResults = Results;
                }
            });
        }

        #endregion

        #region ViewModel Properties

        public ResultsViewModel Results { get; private set; }
        public ResultsViewModel ContextMenu { get; private set; }
        public ResultsViewModel History { get; private set; }

        private string _queryText;
        public string QueryText
        {
            get { return _queryText; }
            set
            {
                _queryText = value;
                Query();
            }
        }

        /// <summary>
        /// we need move cursor to end when we manually changed query
        /// but we don't want to move cursor to end when query is updated from TextBox
        /// </summary>
        /// <param name="queryText"></param>
        public void ChangeQueryText(string queryText)
        {
            QueryTextCursorMovedToEnd = true;
            QueryText = queryText;
        }
        public bool LastQuerySelected { get; set; }
        public bool QueryTextCursorMovedToEnd { get; set; }

        private ResultsViewModel _selectedResults;
        private ResultsViewModel SelectedResults
        {
            get { return _selectedResults; }
            set
            {
                _selectedResults = value;
                if (SelectedIsFromQueryResults())
                {
                    ContextMenu.Visbility = Visibility.Collapsed;
                    History.Visbility = Visibility.Collapsed;
                    ChangeQueryText(_queryTextBeforeLeaveResults);
                }
                else
                {
                    Results.Visbility = Visibility.Collapsed;
                    _queryTextBeforeLeaveResults = QueryText;


                    // Because of Fody's optimization
                    // setter won't be called when property value is not changed.
                    // so we need manually call Query()
                    // http://stackoverflow.com/posts/25895769/revisions
                    if (string.IsNullOrEmpty(QueryText))
                    {
                        Query();
                    }
                    else
                    {
                        QueryText = string.Empty;
                    }
                }
                _selectedResults.Visbility = Visibility.Visible;
            }
        }

        public Visibility ProgressBarVisibility { get; set; }

        public Visibility MainWindowVisibility { get; set; }

        public ICommand EscCommand { get; set; }
        public ICommand SelectNextItemCommand { get; set; }
        public ICommand SelectPrevItemCommand { get; set; }
        public ICommand SelectNextPageCommand { get; set; }
        public ICommand SelectPrevPageCommand { get; set; }
        public ICommand SelectFirstResultCommand { get; set; }
        public ICommand StartHelpCommand { get; set; }
        public ICommand RefreshCommand { get; set; }
        public ICommand LoadContextMenuCommand { get; set; }
        public ICommand LoadHistoryCommand { get; set; }
        public ICommand OpenResultCommand { get; set; }

        #endregion

        public void Query()
        {
            if (SelectedIsFromQueryResults())
            {
                QueryResults();
            }
            else if (ContextMenuSelected())
            {
                QueryContextMenu();
            }
            else if (HistorySelected())
            {
                QueryHistory();
            }
        }

        private void QueryContextMenu()
        {
            const string id = "Context Menu ID";
            var query = QueryText.ToLower().Trim();
            ContextMenu.Clear();

            var selected = Results.SelectedItem?.Result;

            if (selected != null) // SelectedItem returns null if selection is empty.
            {
                var results = PluginManager.GetContextMenusForPlugin(selected);
                results.Add(ContextMenuTopMost(selected));
                results.Add(ContextMenuPluginInfo(selected.PluginID));

                if (!string.IsNullOrEmpty(query))
                {
                    var filtered = results.Where
                    (
                        r => StringMatcher.FuzzySearch(query, r.Title).IsSearchPrecisionScoreMet()
                            || StringMatcher.FuzzySearch(query, r.SubTitle).IsSearchPrecisionScoreMet()
                    ).ToList();
                    ContextMenu.AddResults(filtered, id);
                }
                else
                {
                    ContextMenu.AddResults(results, id);
                }
            }
        }

        private void QueryHistory()
        {
            const string id = "Query History ID";
            var query = QueryText.ToLower().Trim();
            History.Clear();

            var results = new List<Result>();
            foreach (var h in _history.Items)
            {
                var title = _translator.GetTranslation("executeQuery");
                var time = _translator.GetTranslation("lastExecuteTime");
                var result = new Result
                {
                    Title = string.Format(title, h.Query),
                    SubTitle = string.Format(time, h.ExecutedDateTime),
                    IcoPath = "Images\\history.png",
                    OriginQuery = new Query { RawQuery = h.Query },
                    Action = _ =>
                    {
                        SelectedResults = Results;
                        ChangeQueryText(h.Query);
                        return false;
                    }
                };
                results.Add(result);
            }

            if (!string.IsNullOrEmpty(query))
            {
                var filtered = results.Where
                (
                    r => StringMatcher.FuzzySearch(query, r.Title).IsSearchPrecisionScoreMet() ||
                         StringMatcher.FuzzySearch(query, r.SubTitle).IsSearchPrecisionScoreMet()
                ).ToList();
                History.AddResults(filtered, id);
            }
            else
            {
                History.AddResults(results, id);
            }
        }

        private void QueryResults()
        {
            if (_updateSource != null && !_updateSource.IsCancellationRequested)
            {
                // first condition used for init run
                // second condition used when task has already been canceled in last turn
                _updateSource.Cancel();
                Logger.WoxDebug($"cancel init {_updateSource.Token.GetHashCode()} {Thread.CurrentThread.ManagedThreadId} {QueryText}");
                _updateSource.Dispose();
            }
            var source = new CancellationTokenSource();
            _updateSource = source;
            var token = source.Token;

            ProgressBarVisibility = Visibility.Hidden;

            var queryText = QueryText.Trim();
            Task.Run(() =>
            {
                if (!string.IsNullOrEmpty(queryText))
                {
                    if (token.IsCancellationRequested) { return; }
                    var query = QueryBuilder.Build(queryText, PluginManager.NonGlobalPlugins);
                    _lastQuery = query;
                    if (query != null)
                    {
                        // handle the exclusiveness of plugin using action keyword
                        if (token.IsCancellationRequested) { return; }

                        Task.Delay(200, token).ContinueWith(_ =>
                        {
                            Logger.WoxTrace($"progressbar visible 1 {token.GetHashCode()} {token.IsCancellationRequested}  {Thread.CurrentThread.ManagedThreadId}  {query} {ProgressBarVisibility}");
                            // start the progress bar if query takes more than 200 ms
                            if (!token.IsCancellationRequested)
                            {
                                ProgressBarVisibility = Visibility.Visible;
                            }
                        }, token);


                        if (token.IsCancellationRequested) { return; }
                        var plugins = PluginManager.AllPlugins;

                        var option = new ParallelOptions()
                        {
                            CancellationToken = token,
                        };
                        CountdownEvent countdown = new CountdownEvent(plugins.Count);

                        foreach (var plugin in plugins)
                        {
                            Task.Run(() =>
                            {
                                if (token.IsCancellationRequested)
                                {
                                    Logger.WoxTrace($"canceled {token.GetHashCode()} {Thread.CurrentThread.ManagedThreadId}  {queryText} {plugin.Metadata.Name}");
                                    countdown.Signal();
                                    return;
                                }
                                var results = PluginManager.QueryForPlugin(plugin, query);
                                if (token.IsCancellationRequested)
                                {
                                    Logger.WoxTrace($"canceled {token.GetHashCode()} {Thread.CurrentThread.ManagedThreadId}  {queryText} {plugin.Metadata.Name}");
                                    countdown.Signal();
                                    return;
                                }

                                _resultsQueue.Add(new ResultsForUpdate(results, plugin.Metadata, query, token, countdown));
                            }, token).ContinueWith(ErrorReporting.UnhandledExceptionHandleTask, TaskContinuationOptions.OnlyOnFaulted);
                        }

                        Task.Run(() =>
                        {
                            Logger.WoxTrace($"progressbar visible 2 {token.GetHashCode()} {token.IsCancellationRequested}  {Thread.CurrentThread.ManagedThreadId}  {query} {ProgressBarVisibility}");
                            // wait all plugins has been processed
                            try
                            {
                                countdown.Wait(token);
                            }
                            catch (OperationCanceledException)
                            {
                                // todo: why we need hidden here and why progress bar is not working
                                ProgressBarVisibility = Visibility.Hidden;
                                return;
                            }
                            if (!token.IsCancellationRequested)
                            {
                                // used to cancel previous progress bar visible task
                                source.Cancel();
                                source.Dispose();
                                // update to hidden if this is still the current query
                                ProgressBarVisibility = Visibility.Hidden;
                            }
                        });


                    }
                }
                else
                {
                    Results.Clear();
                    Results.Visbility = Visibility.Collapsed;
                }
            }, token).ContinueWith(ErrorReporting.UnhandledExceptionHandleTask, TaskContinuationOptions.OnlyOnFaulted);

        }

        private void Refresh()
        {
            PluginManager.ReloadData();
        }

        private Result ContextMenuTopMost(Result result)
        {
            Result menu;
            if (_topMostRecord.IsTopMost(result))
            {
                menu = new Result
                {
                    Title = InternationalizationManager.Instance.GetTranslation("cancelTopMostInThisQuery"),
                    IcoPath = "Images\\down.png",
                    PluginDirectory = Constant.ProgramDirectory,
                    Action = _ =>
                    {
                        _topMostRecord.Remove(result);
                        App.API.ShowMsg("Success");
                        return false;
                    }
                };
            }
            else
            {
                menu = new Result
                {
                    Title = InternationalizationManager.Instance.GetTranslation("setAsTopMostInThisQuery"),
                    IcoPath = "Images\\up.png",
                    PluginDirectory = Constant.ProgramDirectory,
                    Action = _ =>
                    {
                        _topMostRecord.AddOrUpdate(result);
                        App.API.ShowMsg("Success");
                        return false;
                    }
                };
            }
            return menu;
        }

        private Result ContextMenuPluginInfo(string id)
        {
            var metadata = PluginManager.GetPluginForId(id).Metadata;
            var translator = InternationalizationManager.Instance;

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

        private bool SelectedIsFromQueryResults()
        {
            var selected = SelectedResults == Results;
            return selected;
        }

        private bool ContextMenuSelected()
        {
            var selected = SelectedResults == ContextMenu;
            return selected;
        }


        private bool HistorySelected()
        {
            var selected = SelectedResults == History;
            return selected;
        }
        #region Hotkey

        private void SetHotkey(string hotkeyStr, EventHandler<HotkeyEventArgs> action)
        {
            var hotkey = new HotkeyModel(hotkeyStr);
            SetHotkey(hotkey, action);
        }

        private void SetHotkey(HotkeyModel hotkey, EventHandler<HotkeyEventArgs> action)
        {
            string hotkeyStr = hotkey.ToString();
            try
            {
                HotkeyManager.Current.AddOrReplace(hotkeyStr, hotkey.CharKey, hotkey.ModifierKeys, action);
            }
            catch (Exception)
            {
                string errorMsg =
                    string.Format(InternationalizationManager.Instance.GetTranslation("registerHotkeyFailed"), hotkeyStr);
                MessageBox.Show(errorMsg);
            }
        }

        public void RemoveHotkey(string hotkeyStr)
        {
            if (!string.IsNullOrEmpty(hotkeyStr))
            {
                HotkeyManager.Current.Remove(hotkeyStr);
            }
        }

        /// <summary>
        /// Checks if Wox should ignore any hotkeys
        /// </summary>
        /// <returns></returns>
        private bool ShouldIgnoreHotkeys()
        {
            //double if to omit calling win32 function
            if (_settings.IgnoreHotkeysOnFullscreen)
                if (WindowsInteropHelper.IsWindowFullscreen())
                    return true;

            return false;
        }

        private void SetCustomPluginHotkey()
        {
            if (_settings.CustomPluginHotkeys == null) return;
            foreach (CustomPluginHotkey hotkey in _settings.CustomPluginHotkeys)
            {
                SetHotkey(hotkey.Hotkey, (s, e) =>
                {
                    if (ShouldIgnoreHotkeys()) return;
                    MainWindowVisibility = Visibility.Visible;
                    ChangeQueryText(hotkey.ActionKeyword);
                });
            }
        }

        private void OnHotkey(object sender, HotkeyEventArgs e)
        {
            if (!ShouldIgnoreHotkeys())
            {

                if (_settings.LastQueryMode == LastQueryMode.Empty)
                {
                    ChangeQueryText(string.Empty);
                }
                else if (_settings.LastQueryMode == LastQueryMode.Preserved)
                {
                    LastQuerySelected = true;
                }
                else if (_settings.LastQueryMode == LastQueryMode.Selected)
                {
                    LastQuerySelected = false;
                }
                else
                {
                    throw new ArgumentException($"wrong LastQueryMode: <{_settings.LastQueryMode}>");
                }

                ToggleWox();
                e.Handled = true;
            }
        }

        private void ToggleWox()
        {
            if (MainWindowVisibility != Visibility.Visible)
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
                _historyItemsStorage.Save();
                _userSelectedRecordStorage.Save();
                _topMostRecordStorage.Save();

                _saved = true;
            }
        }

        public void UpdateResultView(List<Result> list, PluginMetadata metadata, Query originQuery, CancellationToken token)
        {
            CountdownEvent countdown = new CountdownEvent(1);
            List<ResultsForUpdate> updates = new List<ResultsForUpdate>()
            {
                new ResultsForUpdate(list, metadata, originQuery, token, countdown)
            };
            UpdateResultView(updates);
        }

        /// <summary>
        /// To avoid deadlock, this method should not called from main thread
        /// </summary>
        public void UpdateResultView(List<ResultsForUpdate> updates)
        {
            foreach (ResultsForUpdate update in updates)
            {
                Logger.WoxTrace($"{update.Metadata.Name}:{update.Query.RawQuery}");
                foreach (var result in update.Results)
                {
                    if (update.Token.IsCancellationRequested) { return; }
                    if (_topMostRecord.IsTopMost(result))
                    {
                        result.Score = int.MaxValue;
                    }
                    else if (!update.Metadata.KeepResultRawScore)
                    {
                        result.Score += _userSelectedRecord.GetSelectedCount(result) * 10;
                    }
                    else
                    {
                        result.Score = result.Score;
                    }
                }
            }

            Results.AddResults(updates);

            if (Results.Visbility != Visibility.Visible && Results.Count > 0)
            {
                Results.Visbility = Visibility.Visible;
            }
        }

        #endregion
    }
}