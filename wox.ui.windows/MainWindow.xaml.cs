using System;
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

        _viewModel = DataContext as MainViewModel;
        if (_viewModel != null)
        {
            _viewModel.PropertyChanged += ViewModel_PropertyChanged;
        }
        _apiService = WoxApiService.Instance;

        // Subscribe to API service events
        _apiService.ShowRequested += OnShowRequested;
        _apiService.HideRequested += OnHideRequested;
        _apiService.ToggleRequested += OnToggleRequested;
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

    private void ViewModel_PropertyChanged(object? sender, System.ComponentModel.PropertyChangedEventArgs e)
    {
        if (e.PropertyName == nameof(MainViewModel.PreviewScrollPosition))
        {
            if (_viewModel?.PreviewScrollPosition == "bottom")
            {
                Dispatcher.InvokeAsync(() => PreviewScrollViewer.ScrollToBottom(), System.Windows.Threading.DispatcherPriority.Loaded);
            }
            else
            {
                Dispatcher.InvokeAsync(() => PreviewScrollViewer.ScrollToTop(), System.Windows.Threading.DispatcherPriority.Loaded);
            }
        }
    }

    private void OnWindowHeightChanged(double newHeight)
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

    private void OnShowRequested(object? sender, System.Collections.Generic.List<QueryHistory> history)
    {
        Dispatcher.Invoke(() =>
        {
            _viewModel.OnShowHistory(history);
            ShowAndFocus();
        });
    }

    private void OnHideRequested(object? sender, EventArgs e)
    {
        Dispatcher.Invoke(() =>
        {
            Hide();
        });
    }

    private void OnToggleRequested(object? sender, EventArgs e)
    {
        Dispatcher.Invoke(() =>
        {
            if (IsVisible)
            {
                Hide();
                return;
            }

            ShowAndFocus();
        });
    }

    private void ShowAndFocus()
    {
        Show();
        if (WindowState == WindowState.Minimized)
        {
            WindowState = WindowState.Normal;
        }

        Activate();
        QueryTextBox.Focus();
        QueryTextBox.SelectAll();
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
        // Handle action panel navigation when visible
        if (_viewModel.IsActionPanelVisible)
        {
            switch (e.Key)
            {
                case Key.Escape:
                    _viewModel.HideActionPanelCommand.Execute(null);
                    e.Handled = true;
                    return;
                case Key.Up:
                    _viewModel.MoveActionUpCommand.Execute(null);
                    e.Handled = true;
                    return;
                case Key.Down:
                    _viewModel.MoveActionDownCommand.Execute(null);
                    e.Handled = true;
                    return;
                case Key.Enter:
                    _ = _viewModel.ExecuteSelectedActionCommand.ExecuteAsync(null);
                    e.Handled = true;
                    return;
            }
        }

        // Handle Alt key for quick select mode
        if (e.Key == Key.LeftAlt || e.Key == Key.RightAlt)
        {
            _viewModel.ActivateQuickSelectModeCommand.Execute(null);
        }

        // Alt+number for quick select
        if (Keyboard.Modifiers == ModifierKeys.Alt)
        {
            var number = GetNumberFromKey(e.Key);
            if (number >= 0 && number <= 9)
            {
                var index = number == 0 ? 9 : number - 1; // 1-9 maps to 0-8, 0 maps to 9
                _ = _viewModel.QuickSelectCommand.ExecuteAsync(index);
                e.Handled = true;
                return;
            }
        }

        switch (e.Key)
        {
            case Key.Escape:
                Hide();
                e.Handled = true;
                break;
        }
    }

    private void Window_PreviewKeyUp(object sender, KeyEventArgs e)
    {
        // Deactivate quick select mode when Alt is released
        if (e.Key == Key.LeftAlt || e.Key == Key.RightAlt)
        {
            _viewModel.DeactivateQuickSelectModeCommand.Execute(null);
        }
    }

    private static int GetNumberFromKey(Key key)
    {
        return key switch
        {
            Key.D1 or Key.NumPad1 => 1,
            Key.D2 or Key.NumPad2 => 2,
            Key.D3 or Key.NumPad3 => 3,
            Key.D4 or Key.NumPad4 => 4,
            Key.D5 or Key.NumPad5 => 5,
            Key.D6 or Key.NumPad6 => 6,
            Key.D7 or Key.NumPad7 => 7,
            Key.D8 or Key.NumPad8 => 8,
            Key.D9 or Key.NumPad9 => 9,
            Key.D0 or Key.NumPad0 => 0,
            _ => -1
        };
    }

    private void QueryTextBox_PreviewKeyDown(object sender, KeyEventArgs e)
    {
        switch (e.Key)
        {
            case Key.Down:
                if (_viewModel.Results.Count > 0)
                {
                     _viewModel.MoveSelectionDownCommand.Execute(null);
                }
                else
                {
                    _viewModel.MoveHistoryDownCommand.Execute(null);
                }
                e.Handled = true;
                break;

            case Key.Up:
                if (_viewModel.Results.Count > 0 && _viewModel.SelectedIndex > 0)
                {
                    _viewModel.MoveSelectionUpCommand.Execute(null);
                }
                else
                {
                    _viewModel.MoveHistoryUpCommand.Execute(null);
                }
                e.Handled = true;
                break;

            case Key.Enter:
                // Shift+Enter inserts new line (handled by TextBox with AcceptsReturn)
                if (Keyboard.Modifiers == ModifierKeys.Shift)
                {
                    // Let the TextBox handle Shift+Enter for new line
                    return;
                }
                // Alt+Enter toggles action panel
                if (Keyboard.Modifiers == ModifierKeys.Alt)
                {
                    _viewModel.ToggleActionPanelCommand.Execute(null);
                }
                else
                {
                    _ = _viewModel.ExecuteSelectedCommand.ExecuteAsync(null);
                }
                e.Handled = true;
                break;

            case Key.Tab:
                // Tab for auto-complete
                _viewModel.AutoCompleteCommand.Execute(null);
                // Move cursor to end after auto-complete
                QueryTextBox.CaretIndex = QueryTextBox.Text.Length;
                e.Handled = true;
                break;

            case Key.Left:
                if (_viewModel.IsGridLayout && _viewModel.Results.Count > 0)
                {
                    _viewModel.MoveSelectionLeftCommand.Execute(null);
                    e.Handled = true;
                }
                break;

            case Key.Right:
                if (_viewModel.IsGridLayout && _viewModel.Results.Count > 0)
                {
                    _viewModel.MoveSelectionRightCommand.Execute(null);
                    e.Handled = true;
                }
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

    private void QueryIcon_MouseLeftButtonUp(object sender, MouseButtonEventArgs e)
    {
        // Query icon click action - focus back to query box
        QueryTextBox.Focus();
    }
}
