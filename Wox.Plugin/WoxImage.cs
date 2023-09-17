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
    private WoxImage(WoxImageType woxImageType, string imageData)
    {
        ImageType = woxImageType;
        ImageData = imageData;
    }

    public string ImageData { get; }
    public WoxImageType ImageType { get; }

    public static WoxImage FromAbsolutePath(string path)
    {
        return new WoxImage(WoxImageType.AbsolutePath, path);
    }

    public static WoxImage FromRelativeToPluginPath(string path)
    {
        return new WoxImage(WoxImageType.RelativeToPluginPath, path);
    }

    public static WoxImage FromSvg(string svg)
    {
        return new WoxImage(WoxImageType.Svg, svg);
    }

    public static WoxImage FromBase64(string base64)
    {
        return new WoxImage(WoxImageType.Base64, base64);
    }

    public static WoxImage FromRemote(string url)
    {
        return new WoxImage(WoxImageType.Remote, url);
    }
}