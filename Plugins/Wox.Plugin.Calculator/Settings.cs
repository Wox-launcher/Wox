using System;
using System.Collections.Generic;
using System.Linq;
using System.Text;
using System.Threading.Tasks;

namespace Wox.Plugin.Caculator
{
    public class Settings
    {
        public DecimalSeparator DecimalSeparator { get; set; } = DecimalSeparator.UseSystemLocale;
        public int MaxDecimalPlaces { get; set; } = 10;      
    }
}
