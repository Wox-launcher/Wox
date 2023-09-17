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

    public ObservableCollection<PluginQueryResult> QueryResult => new(_results);

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
            this.RaisePropertyChanged(nameof(QueryResult));
        }
    }
}