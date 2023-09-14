using System;
using System.Collections.Generic;
using System.Collections.ObjectModel;
using System.Threading.Tasks;
using ReactiveUI;
using Wox.Core;
using Wox.Core.Plugin;
using Wox.Core.Utils;
using Wox.Plugin;

namespace Wox.ViewModels;

public class CoreQueryViewModel : ViewModelBase
{
    private string? _Query;

    public string? Query
    {
        get => _Query;
        set =>
            // use "RaiseAndSetIfChanged" to check if the value changed and automatically notify the UI
            this.RaiseAndSetIfChanged(ref _Query, value);
    }

    public ObservableCollection<PluginQueryResult> QueryResult
    {
        get
        {
            if (string.IsNullOrEmpty(_Query))
            {
                return new ObservableCollection<PluginQueryResult>();
            }
            else
            {
                ObservableCollection<PluginQueryResult> list = new ObservableCollection<PluginQueryResult>
                {
                    new PluginQueryResult()
                    {
                        Result = new Result() { Id = _Query, Title = "xxxx", Description = "xxxx", IcoPath = "xxxx", Score = 22 },
                        AssociatedQuery = null,
                        Plugin = null
                    },
                    new PluginQueryResult()
                    {
                        Result = new Result() { Id = "fdfdas", Title = "xxxx", Description = "xxxx", IcoPath = "xxxx", Score = 22 },
                        AssociatedQuery = null,
                        Plugin = null
                    }
                };
                return list;
            }
        }
    }
    
    public CoreQueryViewModel()
    {
        // We can listen to any property changes with "WhenAnyValue" and do whatever we want in "Subscribe".
        this.WhenAnyValue(o => o.Query)!
            .Subscribe(new Action<object>(o => this.RaisePropertyChanged(nameof(QueryResult))));
    }
}