using System;
using Avalonia.Controls;
using Avalonia.Threading;

namespace Wox.Views;

public partial class CoreQueryView : UserControl
{
    public CoreQueryView()
    {
        InitializeComponent();
        QueryTextBox.Focus();
    }
}