using System;
using System.Collections.Generic;
using System.Text.Json;
using System.Text.Json.Serialization;

namespace Wox.UI.Windows.Models;

[JsonConverter(typeof(PluginSettingDefinitionItemConverter))]
public class PluginSettingDefinitionItem
{
    public string Type { get; set; } = string.Empty;
    public object? Value { get; set; }
    public List<string> DisabledInPlatforms { get; set; } = new();
    public bool IsPlatformSpecific { get; set; }
}

public class PluginSettingDefinitionItemConverter : JsonConverter<PluginSettingDefinitionItem>
{
    public override PluginSettingDefinitionItem? Read(ref Utf8JsonReader reader, Type typeToConvert, JsonSerializerOptions options)
    {
        using var doc = JsonDocument.ParseValue(ref reader);
        var root = doc.RootElement;
        
        var item = new PluginSettingDefinitionItem();
        
        if (root.TryGetProperty("Type", out var typeProp))
        {
            item.Type = typeProp.GetString() ?? string.Empty;
        }

        if (root.TryGetProperty("DisabledInPlatforms", out var disabledProp))
        {
             item.DisabledInPlatforms = JsonSerializer.Deserialize<List<string>>(disabledProp.GetRawText(), options) ?? new();
        }

        if (root.TryGetProperty("IsPlatformSpecific", out var platformProp))
        {
            item.IsPlatformSpecific = platformProp.GetBoolean();
        }

        if (root.TryGetProperty("Value", out var valueProp))
        {
            var rawValue = valueProp.GetRawText();
            switch (item.Type)
            {
                case "textbox":
                    item.Value = JsonSerializer.Deserialize<PluginSettingValueTextBox>(rawValue, options);
                    break;
                case "checkbox":
                    item.Value = JsonSerializer.Deserialize<PluginSettingValueCheckBox>(rawValue, options);
                    break;
                case "select":
                    item.Value = JsonSerializer.Deserialize<PluginSettingValueSelect>(rawValue, options);
                    break;
                case "label":
                    item.Value = JsonSerializer.Deserialize<PluginSettingValueLabel>(rawValue, options);
                    break;
                case "head":
                    item.Value = JsonSerializer.Deserialize<PluginSettingValueHead>(rawValue, options);
                    break;
                case "newline":
                    item.Value = new PluginSettingValueNewLine();
                    break;
                // Add other types as needed
                default:
                    // Keep as JsonElement or ignore
                    break;
            }
        }

        return item;
    }

    public override void Write(Utf8JsonWriter writer, PluginSettingDefinitionItem value, JsonSerializerOptions options)
    {
        writer.WriteStartObject();
        writer.WriteString("Type", value.Type);
        writer.WriteBoolean("IsPlatformSpecific", value.IsPlatformSpecific);
        
        writer.WritePropertyName("DisabledInPlatforms");
        JsonSerializer.Serialize(writer, value.DisabledInPlatforms, options);

        writer.WritePropertyName("Value");
        JsonSerializer.Serialize(writer, value.Value, options);
        
        writer.WriteEndObject();
    }
}

public class PluginSettingValueStyle
{
    public double PaddingLeft { get; set; }
    public double PaddingTop { get; set; }
    public double PaddingRight { get; set; }
    public double PaddingBottom { get; set; }
    public double Width { get; set; }
    public double LabelWidth { get; set; }
}

public class PluginSettingValidatorItem
{
    public string Type { get; set; } = string.Empty;
    public string ErrorArg { get; set; } = string.Empty;
}

public class PluginSettingValueTextBox
{
    public string Key { get; set; } = string.Empty;
    public string Label { get; set; } = string.Empty;
    public string Suffix { get; set; } = string.Empty;
    public string DefaultValue { get; set; } = string.Empty;
    public string Tooltip { get; set; } = string.Empty;
    public int MaxLines { get; set; } = 1;
    public PluginSettingValueStyle? Style { get; set; }
    public List<PluginSettingValidatorItem> Validators { get; set; } = new();
}

public class PluginSettingValueCheckBox
{
    public string Key { get; set; } = string.Empty;
    public string Label { get; set; } = string.Empty;
    public bool DefaultValue { get; set; }
    public string Tooltip { get; set; } = string.Empty;
    public PluginSettingValueStyle? Style { get; set; }
}

public class PluginSettingValueSelect
{
    public string Key { get; set; } = string.Empty;
    public string Label { get; set; } = string.Empty;
    public string Suffix { get; set; } = string.Empty;
    public string DefaultValue { get; set; } = string.Empty;
    public string Tooltip { get; set; } = string.Empty;
    public List<PluginSettingValueSelectOption> Options { get; set; } = new();
    public PluginSettingValueStyle? Style { get; set; }
}

public class PluginSettingValueSelectOption
{
    public string Label { get; set; } = string.Empty;
    public string Value { get; set; } = string.Empty;
}

public class PluginSettingValueLabel
{
    [JsonPropertyName("Content")]
    public string Content { get; set; } = string.Empty;
    public PluginSettingValueStyle? Style { get; set; }
}

public class PluginSettingValueHead
{
    [JsonPropertyName("Content")]
    public string Content { get; set; } = string.Empty;
    public PluginSettingValueStyle? Style { get; set; }
}

public class PluginSettingValueNewLine
{
}
