using System;
using System.Collections.ObjectModel;
using System.Linq;
using System.Threading.Tasks;
using System.Windows;
using CommunityToolkit.Mvvm.ComponentModel;
using CommunityToolkit.Mvvm.Input;
using Wox.UI.Windows.Models;
using Wox.UI.Windows.Services;
using System.Collections.Generic;

namespace Wox.UI.Windows.ViewModels;

public partial class MainViewModel : ObservableObject
{
    private readonly WoxApiService _apiService;
    private string _currentQueryId = string.Empty;
    private int _maxResultCount = UIConstants.MAX_LIST_VIEW_ITEM_COUNT;

    [ObservableProperty]
    private string _queryText = string.Empty;

    [ObservableProperty]
    private ObservableCollection<ResultItem> _results = new();

    [ObservableProperty]
    private ResultItem? _selectedResult;

    [ObservableProperty]
    private int _selectedIndex = 0;

    [ObservableProperty]
    private string? _previewContent;

    [ObservableProperty]
    private string _previewType = "text";

    [ObservableProperty]
    private string? _previewImagePath;

    [ObservableProperty]
    private double _previewWidth = 300;

    [ObservableProperty]
    private string? _previewScrollPosition;

    [ObservableProperty]
    private bool _isPreviewVisible;

    [ObservableProperty]
    private bool _isToolbarVisible;

    [ObservableProperty]
    private double _windowHeight = 200;

    [ObservableProperty]
    private double _windowWidth = 800;

    // Action panel state
    [ObservableProperty]
    private bool _isActionPanelVisible;

    [ObservableProperty]
    private ObservableCollection<ActionItem> _currentActions = new();

    [ObservableProperty]
    private int _selectedActionIndex = 0;

    // Quick select mode
    [ObservableProperty]
    private bool _isQuickSelectMode;

    // Toolbar message
    [ObservableProperty]
    private string? _toolbarMessage;

    [ObservableProperty]
    private string? _toolbarIcon;

    private List<QueryHistory> _queryHistory = new();
    private int _historyIndex = -1;

    public event EventHandler<double>? WindowHeightChanged;
    public event EventHandler<string>? AutoCompleteRequested;

    public MainViewModel()
    {
        _apiService = WoxApiService.Instance;
        _apiService.ResultsReceived += OnResultsReceived;
        _apiService.QueryChanged += OnQueryChanged;
        _apiService.RefreshQueryRequested += OnRefreshQueryRequested;
        _apiService.ToolbarMsgReceived += OnToolbarMsgReceived;
        _apiService.UpdateResultReceived += OnUpdateResultReceived;
        _apiService.GetCurrentQueryRequested += OnGetCurrentQueryRequested;
    }

    public void OnShowHistory(List<QueryHistory> history)
    {
        _queryHistory = history;
        _historyIndex = -1;
    }

    public void ApplySetting(WoxSetting setting)
    {
        if (setting.AppWidth > 0)
        {
            WindowWidth = setting.AppWidth;
        }

        if (setting.MaxResultCount > 0)
        {
            _maxResultCount = setting.MaxResultCount;
        }

        if (setting.PreviewWidthRatio > 0)
        {
            PreviewWidth = WindowWidth * setting.PreviewWidthRatio;
        }
        else
        {
            PreviewWidth = WindowWidth * 0.4; // Default ratio
        }

        ResizeWindow();
    }

    partial void OnQueryTextChanged(string value)
    {
        // Debounce query sending
        _ = SendQueryAsync(value);
    }

    partial void OnSelectedResultChanged(ResultItem? value)
    {
        if (value?.Preview != null)
        {
            PreviewContent = value.Preview.PreviewData;
            PreviewType = value.Preview.PreviewType ?? "text";
            PreviewScrollPosition = value.Preview.ScrollPosition;
            IsPreviewVisible = true;

            // Handle image preview type
            if (PreviewType.Equals("image", StringComparison.OrdinalIgnoreCase) ||
                PreviewType.Equals("file", StringComparison.OrdinalIgnoreCase))
            {
                var data = value.Preview.PreviewData;
                // Check if it's an image file
                if (PreviewType.Equals("file", StringComparison.OrdinalIgnoreCase))
                {
                    var ext = System.IO.Path.GetExtension(data)?.ToLower();
                    if (ext == ".png" || ext == ".jpg" || ext == ".jpeg" || ext == ".gif" || ext == ".bmp" || ext == ".webp")
                    {
                        PreviewImagePath = data;
                        PreviewType = "image";
                    }
                    else if (ext == ".md")
                    {
                        // Try to read markdown file content
                        try
                        {
                            if (System.IO.File.Exists(data))
                            {
                                PreviewContent = System.IO.File.ReadAllText(data);
                                PreviewType = "markdown";
                            }
                        }
                        catch { }
                    }
                }
                else if (PreviewType.Equals("image", StringComparison.OrdinalIgnoreCase))
                {
                    PreviewImagePath = data;
                }
            }
        }
        else
        {
            IsPreviewVisible = false;
            PreviewType = "text";
            PreviewImagePath = null;
        }
    }

    private async Task SendQueryAsync(string query)
    {
        await Task.Delay(50); // Simple debounce
        await _apiService.SendQueryAsync(query);
    }

    private void OnResultsReceived(object? sender, QueryResult queryResult)
    {
        Application.Current.Dispatcher.Invoke(() =>
        {
            _currentQueryId = queryResult.QueryId;

            // Replace or append results based on query ID
            Results.Clear();
            foreach (var result in queryResult.Results)
            {
                Results.Add(result);
            }

            // Auto-select first result
            if (Results.Count > 0)
            {
                SelectedIndex = 0;
                SelectedResult = Results[0];
            }

            // Resize window based on result count
            ResizeWindow();
        });
    }

    private void OnQueryChanged(object? sender, string newQuery)
    {
        Application.Current.Dispatcher.Invoke(() =>
        {
            QueryText = newQuery;
        });
    }

    [RelayCommand]
    private async Task ExecuteSelectedAsync()
    {
        if (SelectedResult != null)
        {
            var defaultAction = SelectedResult.Actions?.FirstOrDefault(a => a.IsDefault)
                ?? SelectedResult.Actions?.FirstOrDefault();

            if (defaultAction != null)
            {
                await ExecuteActionInternalAsync(defaultAction, SelectedResult.Id);
            }
        }
    }

    [RelayCommand]
    private void MoveSelectionUp()
    {
        if (SelectedIndex > 0)
        {
            SelectedIndex--;
            SelectedResult = Results[SelectedIndex];
        }
    }

    [RelayCommand]
    private void MoveSelectionDown()
    {
        if (SelectedIndex < Results.Count - 1)
        {
            SelectedIndex++;
            SelectedResult = Results[SelectedIndex];
        }
    }

    [RelayCommand]
    private void ClearQuery()
    {
        QueryText = string.Empty;
        Results.Clear();
    }

    private void ResizeWindow()
    {
        // Use default padding values matching Flutter theme
        var appPaddingTop = 12.0;
        var appPaddingBottom = 12.0;
        var resultContainerPaddingTop = 8.0;
        var resultContainerPaddingBottom = 8.0;
        var resultItemPaddingTop = 8.0;
        var resultItemPaddingBottom = 8.0;

        // Calculate query box height
        var queryBoxHeight = UIConstants.QUERY_BOX_BASE_HEIGHT + appPaddingTop + appPaddingBottom;

        // Calculate result height
        var itemCount = Results.Count;
        var maxResultCount = _maxResultCount;
        var visibleCount = Math.Min(itemCount, maxResultCount);

        double resultHeight = 0;
        if (visibleCount > 0)
        {
            var resultItemHeight = UIConstants.RESULT_ITEM_BASE_HEIGHT + resultItemPaddingTop + resultItemPaddingBottom;
            resultHeight = visibleCount * resultItemHeight + resultContainerPaddingTop + resultContainerPaddingBottom;
        }

        // Add toolbar height if there are results with actions
        var hasActions = Results.Any(r => r.Actions != null && r.Actions.Count > 0);
        IsToolbarVisible = hasActions && visibleCount > 0;
        if (IsToolbarVisible)
        {
            resultHeight += UIConstants.TOOLBAR_HEIGHT;
        }

        WindowHeight = queryBoxHeight + resultHeight;

        Logger.Log($"ResizeWindow: queryBoxHeight={queryBoxHeight}, resultHeight={resultHeight}, total={WindowHeight}, itemCount={itemCount}");

        // Notify window to resize
        WindowHeightChanged?.Invoke(this, WindowHeight);
    }

    #region New Event Handlers

    private void OnRefreshQueryRequested(object? sender, bool preserveSelectedIndex)
    {
        Application.Current.Dispatcher.Invoke(async () =>
        {
            var currentIndex = preserveSelectedIndex ? SelectedIndex : 0;
            await _apiService.SendQueryAsync(QueryText);
            if (preserveSelectedIndex && Results.Count > currentIndex)
            {
                SelectedIndex = currentIndex;
                SelectedResult = Results[currentIndex];
            }
        });
    }

    private void OnToolbarMsgReceived(object? sender, string json)
    {
        Application.Current.Dispatcher.Invoke(() =>
        {
            try
            {
                using var doc = System.Text.Json.JsonDocument.Parse(json);
                var root = doc.RootElement;
                if (root.TryGetProperty("Text", out var textProp))
                {
                    ToolbarMessage = textProp.GetString();
                }
                if (root.TryGetProperty("Icon", out var iconProp))
                {
                    ToolbarIcon = iconProp.GetString();
                }
            }
            catch (Exception ex)
            {
                Logger.Error("Error parsing toolbar message", ex);
            }
        });
    }

    private void OnUpdateResultReceived(object? sender, string json)
    {
        Application.Current.Dispatcher.Invoke(() =>
        {
            try
            {
                using var doc = System.Text.Json.JsonDocument.Parse(json);
                var root = doc.RootElement;

                if (!root.TryGetProperty("ResultId", out var resultIdProp))
                    return;

                var resultId = resultIdProp.GetString();
                var result = Results.FirstOrDefault(r => r.Id == resultId);
                if (result == null)
                    return;

                // Update title if provided
                if (root.TryGetProperty("Title", out var titleProp))
                {
                    result.Title = titleProp.GetString() ?? result.Title;
                }

                // Update subtitle if provided
                if (root.TryGetProperty("SubTitle", out var subTitleProp))
                {
                    result.SubTitle = subTitleProp.GetString();
                }

                Logger.Log($"Updated result: {resultId}");
            }
            catch (Exception ex)
            {
                Logger.Error("Error updating result", ex);
            }
        });
    }

    private object? OnGetCurrentQueryRequested()
    {
        return new
        {
            QueryId = _currentQueryId,
            QueryType = "input",
            RawQuery = QueryText,
            Search = "",
            Command = "",
        };
    }

    #endregion

    #region Action Panel Commands

    [RelayCommand]
    private void ToggleActionPanel()
    {
        if (SelectedResult?.Actions == null || SelectedResult.Actions.Count == 0)
        {
            IsActionPanelVisible = false;
            return;
        }

        IsActionPanelVisible = !IsActionPanelVisible;

        if (IsActionPanelVisible)
        {
            CurrentActions.Clear();
            foreach (var action in SelectedResult.Actions)
            {
                CurrentActions.Add(action);
            }
            SelectedActionIndex = 0;
        }
    }

    [RelayCommand]
    private void HideActionPanel()
    {
        IsActionPanelVisible = false;
        CurrentActions.Clear();
    }

    [RelayCommand]
    private void MoveActionUp()
    {
        if (SelectedActionIndex > 0)
        {
            SelectedActionIndex--;
        }
    }

    [RelayCommand]
    private void MoveActionDown()
    {
        if (SelectedActionIndex < CurrentActions.Count - 1)
        {
            SelectedActionIndex++;
        }
    }

    [RelayCommand]
    private async Task ExecuteSelectedActionAsync()
    {
        if (SelectedResult == null || SelectedActionIndex < 0 || SelectedActionIndex >= CurrentActions.Count)
            return;

        var action = CurrentActions[SelectedActionIndex];
        HideActionPanel();
        await ExecuteActionInternalAsync(action, SelectedResult.Id);
    }

    #endregion

    #region Auto-complete

    [RelayCommand]
    private void AutoComplete()
    {
        if (SelectedResult == null)
            return;

        // Use AutoComplete property if available, otherwise use Title
        var autoCompleteText = SelectedResult.AutoComplete;
        if (!string.IsNullOrEmpty(autoCompleteText))
        {
            QueryText = autoCompleteText;
            AutoCompleteRequested?.Invoke(this, autoCompleteText);
        }
    }

    #endregion

    #region Quick Select Mode

    [RelayCommand]
    private void ActivateQuickSelectMode()
    {
        IsQuickSelectMode = true;
    }

    [RelayCommand]
    private void DeactivateQuickSelectMode()
    {
        IsQuickSelectMode = false;
    }

    [RelayCommand]
    private async Task QuickSelectAsync(int index)
    {
        if (index < 0 || index >= Results.Count)
            return;

        SelectedIndex = index;
        SelectedResult = Results[index];
        await ExecuteSelectedAsync();
    }

    #endregion

    #region Form Action

    [ObservableProperty]
    private FormViewModel? _formViewModel;

    [ObservableProperty]
    private bool _isFormVisible;

    private void ShowForm(ActionItem action, string resultId)
    {
        FormViewModel = new FormViewModel(action, _currentQueryId, resultId);
        FormViewModel.RequestClose += (s, e) => HideForm();
        IsFormVisible = true;
        IsActionPanelVisible = false; // Hide action panel if open
    }

    [RelayCommand]
    private void HideForm()
    {
        IsFormVisible = false;
        FormViewModel = null;
    }

    private async Task ExecuteActionInternalAsync(ActionItem action, string resultId)
    {
        if (action.Type == "form" || action.Form.Count > 0)
        {
            ShowForm(action, resultId);
        }
        else
        {
            await _apiService.SendActionAsync(_currentQueryId, resultId, action.Id);

            if (!action.PreventHideAfterAction)
            {
                Application.Current.MainWindow?.Hide();
            }
        }
    }

    #endregion

    #region Query History

    [RelayCommand]
    private void MoveHistoryUp()
    {
        if (_queryHistory.Count == 0) return;

        if (_historyIndex < _queryHistory.Count - 1)
        {
            _historyIndex++;
            QueryText = _queryHistory[_historyIndex].Query;
            Application.Current.Dispatcher.Invoke(() =>
            {
                // Caret move logic handled in view
            });
        }
    }

    [RelayCommand]
    private void MoveHistoryDown()
    {
        if (_historyIndex > -1)
        {
            _historyIndex--;
            if (_historyIndex == -1)
            {
                QueryText = string.Empty;
            }
            else
            {
                QueryText = _queryHistory[_historyIndex].Query;
            }
        }
    }

    #endregion
}
