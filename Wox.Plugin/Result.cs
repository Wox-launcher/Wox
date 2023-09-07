namespace Wox.Plugin;

public class Result
{
    private string _icoPath;

    public string Title { get; set; }

    public string SubTitle { get; set; }

    public string IcoPath { get; set; }

    /// <summary>
    ///     return true to hide wox after select result
    /// </summary>
    public Func<ActionContext, bool> Action { get; set; }

    public int Score { get; set; }

    /// <summary>
    ///     A list of indexes for the characters to be highlighted in Title
    /// </summary>
    public IList<int> TitleHighlightData { get; set; }

    /// <summary>
    ///     A list of indexes for the characters to be highlighted in SubTitle
    /// </summary>
    public IList<int> SubTitleHighlightData { get; set; }

    /// <summary>
    ///     Only results that originQuery match with current query will be displayed in the panel
    /// </summary>
    public Query OriginQuery { get; set; }

    /// <summary>
    ///     Plugin directory
    /// </summary>
    public string PluginDirectory { get; set; }

    /// <summary>
    ///     Additional data associate with this result
    /// </summary>
    public object ContextData { get; set; }

    /// <summary>
    ///     Plugin ID that generated this result
    /// </summary>
    public string PluginID { get; set; }

    public override bool Equals(object obj)
    {
        var r = obj as Result;
        var equality = r?.PluginID == PluginID && r?.Title == Title && r?.SubTitle == SubTitle;
        return equality;
    }

    public override int GetHashCode()
    {
        var hash1 = PluginID?.GetHashCode() ?? 0;
        var hash2 = Title?.GetHashCode() ?? 0;
        var hash3 = SubTitle?.GetHashCode() ?? 0;
        var hashcode = hash1 ^ hash2 ^ hash3;
        return hashcode;
    }

    public override string ToString()
    {
        return Title + SubTitle;
    }
}