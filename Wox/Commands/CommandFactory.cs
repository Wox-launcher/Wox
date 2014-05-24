using Wox.Core;

namespace Wox.Commands
{
    internal static class CommandFactory
    {
        private static PluginCommand pluginCmd;
        private static SystemCommand systemCmd;

        public static void DispatchCommand(Query query)
        {
            //lazy init command instance.
            if (pluginCmd == null)
            {
                pluginCmd = new PluginCommand();
            }
            if (systemCmd == null)
            {
                systemCmd = new SystemCommand();
            }

            systemCmd.Dispatch(query);
            pluginCmd.Dispatch(query);
        }
    }
}
