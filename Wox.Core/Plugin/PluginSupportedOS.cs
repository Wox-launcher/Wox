using System.Text.Json;
using System.Text.Json.Serialization;

namespace Wox.Plugin;

public enum PluginSupportedOS
{
    Macos,
    Linux,
    Windows
}

public class JsonPluginSupportedOSConverter : JsonConverter<List<PluginSupportedOS>>
{
    public override List<PluginSupportedOS> Read(ref Utf8JsonReader reader, Type typeToConvert, JsonSerializerOptions options)
    {
        if (reader.TokenType != JsonTokenType.StartArray) throw new JsonException($"Expected start of an array, but got {reader.TokenType}.");

        var enumList = new List<PluginSupportedOS>();

        while (reader.Read())
        {
            if (reader.TokenType == JsonTokenType.EndArray) return enumList;

            if (reader.TokenType == JsonTokenType.String)
            {
                if (Enum.TryParse(reader.GetString(), true, out PluginSupportedOS enumValue))
                    enumList.Add(enumValue);
                else
                    throw new JsonException($"Unable to parse enum value: {reader.GetString()}");
            }
        }

        throw new JsonException("Unexpected end of JSON while reading enum list.");
    }

    public override void Write(Utf8JsonWriter writer, List<PluginSupportedOS> value, JsonSerializerOptions options)
    {
        writer.WriteStartArray();

        foreach (var enumValue in value) writer.WriteStringValue(enumValue.ToString());

        writer.WriteEndArray();
    }
}