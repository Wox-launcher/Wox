using System.Windows;
using System.Windows.Input;
using Wox.UI.Windows.Models;
using Wox.UI.Windows.Services;
using Wox.UI.Windows.ViewModels;

namespace Wox.UI.Windows;

public partial class MainWindow : Window
{
    private readonly MainViewModel _viewModel;
    private readonly WoxApiService _apiService;

    public MainWindow()
    {
        InitializeComponent();

        _viewModel = (MainViewModel)DataContext;
        _apiService = WoxApiService.Instance;

        // Subscribe to API service events
        _apiService.ShowRequested += OnShowRequested;
        _apiService.HideRequested += OnHideRequested;
        _apiService.SettingLoaded += OnSettingLoaded;
        ThemeService.Instance.ThemeModeChanged += OnThemeModeChanged;

        // Subscribe to window height changes
        _viewModel.WindowHeightChanged += OnWindowHeightChanged;

        SourceInitialized += (s, e) =>
        {
            WindowBackdropService.ApplyMica(this, ThemeService.Instance.IsDarkTheme);
        };

        // Focus on query box when window is loaded
        Loaded += (s, e) =>
        {
            QueryTextBox.Focus();
        };
    }

    private void OnWindowHeightChanged(object? sender, double newHeight)
    {
        Dispatcher.Invoke(() =>
        {
            Height = newHeight;
            Services.Logger.Log($"MainWindow height changed to: {newHeight}");
        });
    }

    private void OnSettingLoaded(object? sender, WoxSetting setting)
    {
        Dispatcher.Invoke(() =>
        {
            _viewModel.ApplySetting(setting);
            Width = _viewModel.WindowWidth;
        });
    }

    private void OnThemeModeChanged(object? sender, bool isDarkTheme)
    {
        Dispatcher.Invoke(() =>
        {
            WindowBackdropService.ApplyMica(this, isDarkTheme);
        });
    }

    private void OnShowRequested(object? sender, EventArgs e)
    {
        Dispatcher.Invoke(() =>
        {
            Show();
            Activate();
            QueryTextBox.Focus();
            QueryTextBox.SelectAll();
        });
    }

    private void OnHideRequested(object? sender, EventArgs e)
    {
        Dispatcher.Invoke(() =>
        {
            Hide();
        });
    }

    private void Window_Deactivated(object sender, EventArgs e)
    {
        // Hide window when it loses focus
        _ = _apiService.NotifyFocusLostAsync();
        Hide();
    }

    private void Window_MouseLeftButtonDown(object sender, MouseButtonEventArgs e)
    {
        // Allow dragging window by clicking anywhere
        DragMove();
    }

    private void Window_PreviewKeyDown(object sender, KeyEventArgs e)
    {
        switch (e.Key)
        {
            case Key.Escape:
                Hide();
                e.Handled = true;
                break;
        }
    }

    private void QueryTextBox_PreviewKeyDown(object sender, KeyEventArgs e)
    {
        switch (e.Key)
        {
            case Key.Down:
                _viewModel.MoveSelectionDownCommand.Execute(null);
                e.Handled = true;
                break;

            case Key.Up:
                _viewModel.MoveSelectionUpCommand.Execute(null);
                e.Handled = true;
                break;

            case Key.Enter:
                _ = _viewModel.ExecuteSelectedCommand.ExecuteAsync(null);
                e.Handled = true;
                break;
        }
    }

    private void ResultsListView_PreviewMouseLeftButtonUp(object sender, MouseButtonEventArgs e)
    {
        if (_viewModel.SelectedResult != null)
        {
            _ = _viewModel.ExecuteSelectedCommand.ExecuteAsync(null);
        }
    }
}
