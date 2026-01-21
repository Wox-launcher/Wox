using System.Collections.ObjectModel;
using System.Windows;
using CommunityToolkit.Mvvm.ComponentModel;
using CommunityToolkit.Mvvm.Input;
using Wox.UI.Windows.Models;
using Wox.UI.Windows.Services;

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
    private bool _isPreviewVisible;

    [ObservableProperty]
    private bool _isToolbarVisible;

    [ObservableProperty]
    private double _windowHeight = 200;

    [ObservableProperty]
    private double _windowWidth = 800;

    public event EventHandler<double>? WindowHeightChanged;

    public MainViewModel()
    {
        _apiService = WoxApiService.Instance;
        _apiService.ResultsReceived += OnResultsReceived;
        _apiService.QueryChanged += OnQueryChanged;
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
            IsPreviewVisible = true;
        }
        else
        {
            IsPreviewVisible = false;
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
                await _apiService.SendActionAsync(_currentQueryId, SelectedResult.Id, defaultAction.Id);

                if (!defaultAction.PreventHideAfterAction)
                {
                    // Hide window after action
                    Application.Current.MainWindow?.Hide();
                }
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
}
