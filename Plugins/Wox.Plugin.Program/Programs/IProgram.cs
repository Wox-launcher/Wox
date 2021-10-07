using System.Collections.Generic;
using Wox.Infrastructure;

namespace Wox.Plugin.Program.Programs
{
    public interface IProgram
    {
        List<Result> ContextMenus(IPublicAPI api);
        Result Result(string query, IPublicAPI api, StringMatcher stringMatcher);
        string Name { get; }
        string Location { get; }
    }
}
