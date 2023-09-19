using System;
using System.Collections.Generic;
using System.Collections.ObjectModel;
using ReactiveUI;
using Wox.Core;
using Wox.Core.Plugin;

namespace Wox.ViewModels;

public class CoreQueryViewModel : ViewModelBase
{
    private readonly List<PluginQueryResult> _results = new();
    private string? _query;
    private int? _selectedIndex = 0;
    private PluginQueryResult? _selectedResult;

    public CoreQueryViewModel()
    {
        // We can listen to any property changes with "WhenAnyValue" and do whatever we want in "Subscribe".
        this.WhenAnyValue(o => o.Query)!
            .Subscribe(OnQuery);
    }

    public string? Query
    {
        get => _query;
        set =>
            // use "RaiseAndSetIfChanged" to check if the value changed and automatically notify the UI
            this.RaiseAndSetIfChanged(ref _query, value);
    }

    public PluginQueryResult? SelectedQueryResult
    {
        get => _selectedResult;
        set => this.RaiseAndSetIfChanged(ref _selectedResult, value);
    }

    public int? SelectedIndex
    {
        get => _selectedIndex;
        set => this.RaiseAndSetIfChanged(ref _selectedIndex, value);
    }

    public ObservableCollection<PluginQueryResult> QueryResult => new(_results);

    public void ResultListBoxKeyUp()
    {
        _selectedResult = _results[(int)(_selectedIndex > 0 ? _selectedIndex - 1 : 0)];
        this.RaisePropertyChanged(nameof(SelectedQueryResult));
    }

    public void ResultListBoxKeyDown()
    {
        _selectedResult = _results[(int)(_selectedIndex >= _results.Count - 1 ? _results.Count - 1 : _selectedIndex + 1)!];
        this.RaisePropertyChanged(nameof(SelectedQueryResult));
    }

    public void OpenResultCommand()
    {
        var action = _selectedResult!.Result.Action!();
        Console.Write(action);
    }

    private async void OnQuery(string? text)
    {
        if (text == null) return;
        var query = QueryBuilder.Build(text);
        if (query.IsEmpty) return;

        _results.Clear();

        foreach (var plugin in PluginManager.GetAllPlugins())
        {
            var result = await PluginManager.QueryForPlugin(plugin, query);
            foreach (var pluginQueryResult in result)
                if (pluginQueryResult.AssociatedQuery.RawQuery == Query)
                    _results.Add(pluginQueryResult);
            if (_results.Count > 0)
            {
                _selectedResult = _results[0];
                _selectedIndex = 0;
            }
        }

        this.RaisePropertyChanged(nameof(QueryResult));
        this.RaisePropertyChanged(nameof(SelectedQueryResult));
        this.RaisePropertyChanged(nameof(SelectedIndex));
    }
}