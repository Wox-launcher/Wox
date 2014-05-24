using System;
using System.Linq;
using Newtonsoft.Json;

namespace Wox.Core.Data.Storage.UserSettings {
	public class FolderLink {
        [JsonProperty]
		public string Path { get; set; }

	    public string Nickname
	    {
	        get { return Path.Split(new char[] { System.IO.Path.DirectorySeparatorChar }, StringSplitOptions.None).Last(); }
	    }
    }
}
