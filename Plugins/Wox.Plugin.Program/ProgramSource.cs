using System;
using System.Collections.Generic;
using System.Linq;
using System.Text;
using System.Threading.Tasks;

namespace Wox.Plugin.Program
{
    public class ProgramSource
    {
        public string Location { get; set; }

        public override bool Equals(object obj)
        {
            var s = obj as ProgramSource;
            return Location.Equals(s?.Location);
        }

        public override int GetHashCode()
        {
            return this.Location.GetHashCode();
        }
    }
}
