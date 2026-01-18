using System.Windows;
using Wox.UI.Windows.ViewModels;

namespace Wox.UI.Windows;

public partial class TestWindow : Window
{
    private MainWindow? _mainWindow;
    private MainViewModel? _viewModel;

    public TestWindow()
    {
        InitializeComponent();
        InitializeMainWindow();
    }

    private void InitializeMainWindow()
    {
        _mainWindow = new MainWindow();
        _viewModel = (MainViewModel)_mainWindow.DataContext;

        // Don't show the main window initially in test mode
        _mainWindow.Hide();

        // Load sample data by default
        LoadSampleResults();
    }

    private void LoadSampleResults()
    {
        if (_viewModel == null) return;

        _viewModel.Results.Clear();
        var sampleData = DesignTimeData.CreateSampleViewModel();
        foreach (var result in sampleData.Results)
        {
            _viewModel.Results.Add(result);
        }

        if (_viewModel.Results.Count > 0)
        {
            _viewModel.SelectedIndex = 0;
            _viewModel.SelectedResult = _viewModel.Results[0];
        }
    }

    private void LoadSampleResults_Click(object sender, RoutedEventArgs e)
    {
        LoadSampleResults();
    }

    private void LoadLongTextResults_Click(object sender, RoutedEventArgs e)
    {
        if (_viewModel == null) return;

        _viewModel.Results.Clear();
        var results = DesignTimeData.CreateLongTextResults();
        foreach (var result in results)
        {
            _viewModel.Results.Add(result);
        }
    }

    private void LoadIconResults_Click(object sender, RoutedEventArgs e)
    {
        if (_viewModel == null) return;

        _viewModel.Results.Clear();
        var results = DesignTimeData.CreateIconResults();
        foreach (var result in results)
        {
            _viewModel.Results.Add(result);
        }
    }

    private void LoadPreviewResults_Click(object sender, RoutedEventArgs e)
    {
        if (_viewModel == null) return;

        _viewModel.Results.Clear();
        var results = DesignTimeData.CreatePreviewResults();
        foreach (var result in results)
        {
            _viewModel.Results.Add(result);
        }

        if (_viewModel.Results.Count > 0)
        {
            _viewModel.SelectedIndex = 0;
            _viewModel.SelectedResult = _viewModel.Results[0];
        }
    }

    private void ClearResults_Click(object sender, RoutedEventArgs e)
    {
        if (_viewModel == null) return;
        _viewModel.Results.Clear();
    }

    private void DarkTheme_Click(object sender, RoutedEventArgs e)
    {
        // Apply dark theme colors
        Application.Current.Resources["ApplicationBackgroundBrush"] = new System.Windows.Media.SolidColorBrush(System.Windows.Media.Color.FromRgb(30, 30, 30));
        Application.Current.Resources["TextFillColorPrimaryBrush"] = new System.Windows.Media.SolidColorBrush(System.Windows.Media.Colors.White);
    }

    private void LightTheme_Click(object sender, RoutedEventArgs e)
    {
        // Apply light theme colors
        Application.Current.Resources["ApplicationBackgroundBrush"] = new System.Windows.Media.SolidColorBrush(System.Windows.Media.Colors.White);
        Application.Current.Resources["TextFillColorPrimaryBrush"] = new System.Windows.Media.SolidColorBrush(System.Windows.Media.Colors.Black);
    }

    private void ShowMainWindow_Click(object sender, RoutedEventArgs e)
    {
        _mainWindow?.Show();
        _mainWindow?.Activate();
    }

    private void HideMainWindow_Click(object sender, RoutedEventArgs e)
    {
        _mainWindow?.Hide();
    }
}
