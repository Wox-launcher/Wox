using Avalonia.Controls;
using Avalonia.Input;
using Wox.ViewModels;

namespace Wox.Views;

public partial class CoreQueryView : UserControl
{
    public CoreQueryView()
    {
        InitializeComponent();
    }

    public void ListBoxKeyUp(object? sender, KeyEventArgs keyEventArgs)
    {
        ((CoreQueryViewModel)DataContext!).ResultListBoxKeyUp();
    }

    public void ListBoxKeyDown(object? sender, KeyEventArgs keyEventArgs)
    {
        ((CoreQueryViewModel)DataContext!).ResultListBoxKeyDown();
    }
}