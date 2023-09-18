using System.Text.Json.Serialization;

namespace Wox.Plugin;

public enum WoxImageType
{
    /// <summary>
    ///     Represent a image from absolute file path
    /// </summary>
    AbsolutePath,

    /// <summary>
    ///     Represent a image from relative file path (relative to your plugin directory)
    /// </summary>
    RelativeToPluginPath,

    /// <summary>
    ///     Represent a image in svg format
    /// </summary>
    Svg,

    /// <summary>
    ///     Represent a image in base64 format
    /// </summary>
    Base64,

    /// <summary>
    ///     Represent a image from remote url
    /// </summary>
    Remote
}

public class WoxImage
{
    public string ImageData { get; init; } = "";

    [JsonConverter(typeof(JsonStringEnumConverter))]
    public WoxImageType ImageType { get; init; }
}