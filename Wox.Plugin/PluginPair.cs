namespace Wox.Plugin;

public class PluginPair
{
    public IPlugin Plugin { get; set; }
    public PluginMetadata Metadata { get; set; }

    public override string ToString()
    {
        return Metadata.Name;
    }

    public override bool Equals(object obj)
    {
        var r = obj as PluginPair;
        if (r != null)
            return string.Equals(r.Metadata.ID, Metadata.ID);
        return false;
    }

    public override int GetHashCode()
    {
        var hashcode = Metadata.ID?.GetHashCode() ?? 0;
        return hashcode;
    }
}