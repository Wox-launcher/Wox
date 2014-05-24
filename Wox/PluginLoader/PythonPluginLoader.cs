using System.Collections.Generic;
using System.Linq;
using Python.Runtime;
using Wox.Helper;
using Wox.Plugins;

namespace Wox.PluginLoader
{
    public class PythonPluginLoader : BasePluginLoader
    {
        public override List<PluginPair> LoadPlugin()
        {
            if (!CheckPythonEnvironmentInstalled()) return new List<PluginPair>();

            List<PluginPair> plugins = new List<PluginPair>();
            List<PluginMetadata> metadatas = pluginMetadatas.Where(o => o.Language.ToUpper() == AllowedLanguage.Python.ToUpper()).ToList();
            foreach (PluginMetadata metadata in metadatas)
            {
                PythonPluginWrapper python = new PythonPluginWrapper(metadata);
                PluginPair pair = new PluginPair()
                {
                    Plugin = python,
                    Metadata = metadata
                };
                plugins.Add(pair);
            }

            return plugins;
        }

        private bool CheckPythonEnvironmentInstalled() {
            try
            {
                SetPythonHome();
                PythonEngine.Initialize();
                PythonEngine.Shutdown();
            }
            catch {
                Log.Warn("Could't find python environment, all python plugins disabled.");
                return false;
            }
            return true;
        }

        private void SetPythonHome()
        {
            //Environment.SetEnvironmentVariable("PYTHONHOME",Path.Combine(Path.GetDirectoryName(Application.ExecutablePath),"PythonHome"));
            //PythonEngine.PythonHome = 
            //PythonEngine.ProgramName
        }
    }
}
