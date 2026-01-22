using System;
using System.Collections.Generic;
using System.Collections.ObjectModel;
using System.Linq;
using System.Threading.Tasks;
using System.Windows;
using CommunityToolkit.Mvvm.ComponentModel;
using CommunityToolkit.Mvvm.Input;
using Wox.UI.Windows.Models;
using Wox.UI.Windows.Services;

namespace Wox.UI.Windows.ViewModels;

public partial class FormViewModel : ObservableObject
{
    private readonly WoxApiService _apiService;
    private readonly ActionItem _action;
    private readonly string _queryId;
    private readonly string _resultId;

    [ObservableProperty]
    private ObservableCollection<FormItemViewModelBase> _items = new();

    public event EventHandler? RequestClose;

    public FormViewModel(ActionItem action, string queryId, string resultId)
    {
        _apiService = WoxApiService.Instance;
        _action = action;
        _queryId = queryId;
        _resultId = resultId;

        InitializeItems();
    }

    private void InitializeItems()
    {
        foreach (var item in _action.Form)
        {
            FormItemViewModelBase? viewModel = null;

            switch (item.Type)
            {
                case "textbox":
                    if (item.Value is PluginSettingValueTextBox tb)
                        viewModel = new FormTextBoxItemViewModel(tb);
                    break;
                case "checkbox":
                    if (item.Value is PluginSettingValueCheckBox cb)
                        viewModel = new FormCheckboxItemViewModel(cb);
                    break;
                case "select":
                    if (item.Value is PluginSettingValueSelect sel)
                        viewModel = new FormSelectItemViewModel(sel);
                    break;
                case "label":
                    if (item.Value is PluginSettingValueLabel lbl)
                        viewModel = new FormLabelItemViewModel(lbl);
                    break;
                case "head":
                    if (item.Value is PluginSettingValueHead head)
                        viewModel = new FormHeadItemViewModel(head);
                    break;
                case "newline":
                    viewModel = new FormNewLineItemViewModel();
                    break;
            }

            if (viewModel != null)
            {
                Items.Add(viewModel);
            }
        }
    }

    [RelayCommand]
    private async Task SubmitAsync()
    {
        var values = new Dictionary<string, string>();

        foreach (var item in Items)
        {
            if (item is IFormInputItem inputItem)
            {
                values[inputItem.Key] = inputItem.Value;
            }
        }

        await _apiService.SendFormActionAsync(_queryId, _resultId, _action.Id, values);
        
        RequestClose?.Invoke(this, EventArgs.Empty);
    }

    [RelayCommand]
    private void Cancel()
    {
        RequestClose?.Invoke(this, EventArgs.Empty);
    }
}

public abstract class FormItemViewModelBase : ObservableObject
{
}

public interface IFormInputItem
{
    string Key { get; }
    string Value { get; }
}

public partial class FormTextBoxItemViewModel : FormItemViewModelBase, IFormInputItem
{
    private readonly PluginSettingValueTextBox _model;

    public FormTextBoxItemViewModel(PluginSettingValueTextBox model)
    {
        _model = model;
        Label = model.Label;
        Tooltip = model.Tooltip;
        Value = model.DefaultValue; // Logic to load initial values could be added here if passed
        Key = model.Key;
        IsMultiline = model.MaxLines > 1;
    }

    public string Key { get; }

    [ObservableProperty]
    private string _label = string.Empty;

    [ObservableProperty]
    private string _value = string.Empty;

    [ObservableProperty]
    private string _tooltip = string.Empty;
    
    [ObservableProperty]
    private bool _isMultiline;
}

public partial class FormCheckboxItemViewModel : FormItemViewModelBase, IFormInputItem
{
    private readonly PluginSettingValueCheckBox _model;

    public FormCheckboxItemViewModel(PluginSettingValueCheckBox model)
    {
        _model = model;
        Label = model.Label;
        Tooltip = model.Tooltip;
        Key = model.Key;
        IsChecked = model.DefaultValue;
    }

    public string Key { get; }

    public string Value => IsChecked.ToString().ToLower();

    [ObservableProperty]
    private string _label = string.Empty;

    [ObservableProperty]
    private bool _isChecked;

    [ObservableProperty]
    private string _tooltip = string.Empty;
}

public partial class FormSelectItemViewModel : FormItemViewModelBase, IFormInputItem
{
    private readonly PluginSettingValueSelect _model;

    public FormSelectItemViewModel(PluginSettingValueSelect model)
    {
        _model = model;
        Label = model.Label;
        Tooltip = model.Tooltip;
        Key = model.Key;
        Options = new ObservableCollection<PluginSettingValueSelectOption>(model.Options);
        SelectedValue = model.DefaultValue; // Should match one of the options values
        
        // Find matching option for selected item binding if needed, but simplistic Value binding works
    }

    public string Key { get; }

    // For ComboBox binding, usually bind SelectedItem or SelectedValue
    [ObservableProperty]
    private string _selectedValue = string.Empty; 

    public string Value => SelectedValue;

    [ObservableProperty]
    private string _label = string.Empty;

    [ObservableProperty]
    private string _tooltip = string.Empty;

    [ObservableProperty]
    private ObservableCollection<PluginSettingValueSelectOption> _options = new();
}

public partial class FormLabelItemViewModel : FormItemViewModelBase
{
    public FormLabelItemViewModel(PluginSettingValueLabel model)
    {
        Text = model.Content;
    }

    [ObservableProperty]
    private string _text = string.Empty;
}

public partial class FormHeadItemViewModel : FormItemViewModelBase
{
    public FormHeadItemViewModel(PluginSettingValueHead model)
    {
        Text = model.Content;
    }

    [ObservableProperty]
    private string _text = string.Empty;
}

public class FormNewLineItemViewModel : FormItemViewModelBase
{
}
