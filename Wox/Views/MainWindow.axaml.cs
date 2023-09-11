using System;
using Avalonia.Controls;
using Avalonia.Input;
using Wox.ViewModels;

namespace Wox.Views;

public partial class MainWindow : Window
{
    public MainWindow()
    {
        InitializeComponent();
        PointerPressed += MainWindow_PointerPressed;
    }

    private void MainWindow_PointerPressed(object? sender, PointerPressedEventArgs e)
    {
        if (e.Pointer.Type == PointerType.Mouse)
        {
            BeginMoveDrag(e);
        }
    }

    private void WoxMainWindow_OnDeactivated(object? sender, EventArgs e)
    {
        ((MainWindowViewModel)DataContext!).OnDeactivated();
    }
}