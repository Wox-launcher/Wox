using System;
using System.Collections.Generic;
using System.Collections.ObjectModel;
using Avalonia.Threading;
using ReactiveUI;
using Wox.Core;
using Wox.Core.Plugin;
using Wox.Core.Utils;
using Wox.Utils;

namespace Wox.ViewModels;

public class CoreQueryViewModel : ViewModelBase
{
    private readonly List<PluginQueryResult> _results = new();
    private int _caretIndex;
    private string? _query;
    private int? _selectedIndex = 0;

    public CoreQueryViewModel()
    {
        this.WhenAnyValue(o => o.Query)!
            .Subscribe(OnQuery);
        PluginPublicAPI.ChangeQueryEvent += OnChangeQuery;
    }

    public string? Query
    {
        get => _query;
        set =>
            this.RaiseAndSetIfChanged(ref _query, value);
    }

    public int CaretIndex
    {
        get => _caretIndex;
        set =>
            this.RaiseAndSetIfChanged(ref _caretIndex, value);
    }

    public int? SelectedIndex
    {
        get => _selectedIndex;
        set => this.RaiseAndSetIfChanged(ref _selectedIndex, value);
    }

    public ObservableCollection<PluginQueryResult> QueryResult => new(_results);

    private void OnChangeQuery(string query)
    {
        Dispatcher.UIThread.InvokeAsync(() => { Query = query; });
        MoveCaretToEnd(query.Length);
    }

    private void MoveCaretToEnd(int index)
    {
        //here we need to first set to zero, then set to the correct index
        //this is a bug of avalonia, see: https://github.com/AvaloniaUI/Avalonia/issues/12238
        Dispatcher.UIThread.InvokeAsync(() =>
        {
            CaretIndex = 0;
            CaretIndex = index;
        });
    }

    private async void OnQuery(string? text)
    {
        Logger.Info($"On Query invoke: {text}");
        if (text == null) return;
        var query = QueryBuilder.Build(text);
        if (query.IsEmpty)
        {
            _results.Clear();
            this.RaisePropertyChanged(nameof(QueryResult));
            return;
        }

        _results.Clear();

        foreach (var plugin in PluginManager.GetAllPlugins())
        {
            var result = await PluginManager.QueryForPlugin(plugin, query);
            foreach (var pluginQueryResult in result)
                if (pluginQueryResult.AssociatedQuery.RawQuery == Query)
                    _results.Add(pluginQueryResult);
        }

        this.RaisePropertyChanged(nameof(QueryResult));
        _selectedIndex = 0;
        this.RaisePropertyChanged(nameof(SelectedIndex));
    }

    public void MoveUpListBoxSelectedIndex()
    {
        _selectedIndex = _selectedIndex > 0 ? _selectedIndex - 1 : 0;
        this.RaisePropertyChanged(nameof(SelectedIndex));
    }

    public void MoveDownListBoxSelectedIndex()
    {
        _selectedIndex = _selectedIndex >= _results.Count - 1 ? _results.Count - 1 : _selectedIndex + 1;
        this.RaisePropertyChanged(nameof(SelectedIndex));
    }

    public async void AsyncOpenResultAction()
    {
        var hideApp = await _results[(int)_selectedIndex!].Result.Action!();
        if (hideApp)
            UiHelper.HideWindow();
    }
}