using System;
using Avalonia.Controls;
using Avalonia.Controls.Templates;
using Wox.ViewModels;

namespace Wox;

public class ViewLocator : IDataTemplate
{
    public Control Build(object? data)
    {
        if (data == null) return new TextBlock { Text = "Build data is null" };

        var name = data.GetType().FullName!.Replace("ViewModel", "View");
        var type = Type.GetType(name);

        if (type != null) return (Control)Activator.CreateInstance(type)!;

        return new TextBlock { Text = "Not Found: " + name };
    }

    public bool Match(object? data)
    {
        return data is ViewModelBase;
    }
}