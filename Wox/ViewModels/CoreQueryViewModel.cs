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

    public ObservableCollection<PluginQueryResult> QueryResult => new(_results);

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
                    _results.AddRange(result);
            this.RaisePropertyChanged(nameof(QueryResult));
        }
    }
}