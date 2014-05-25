using System.Collections.Generic;
using Newtonsoft.Json;
using Wox.Core.Data;

namespace Wox.Plugins.Internal.CMD
{
    public class CMDStorage : BaseStorage<CMDStorage>
    {
        [JsonProperty]
        public Dictionary<string, int> CMDHistory = new Dictionary<string, int>();

        protected override string ConfigName
        {
            get { return "CMDHistory"; }
        }

        public void AddCmdHistory(string cmdName)
        {
            if (CMDHistory.ContainsKey(cmdName))
            {
                CMDHistory[cmdName] += 1;
            }
            else
            {
                CMDHistory.Add(cmdName, 1);
            }
            Save();
        }
    }
}
