using System;
using System.Collections.Generic;
using System.ComponentModel;
using System.Globalization;
using System.Linq;
using System.Text;
using System.Threading.Tasks;
using Wox.Core;

namespace Wox.Plugin.Caculator
{    
    [TypeConverter(typeof(LocalizationConverter))]
    public enum DecimalSeparator
    {
        [LocalizedDescription("wox_plugin_calculator_decimal_seperator_use_system_locale")]
        UseSystemLocale,
        
        [LocalizedDescription("wox_plugin_calculator_decimal_seperator_dot")]
        Dot, 
        
        [LocalizedDescription("wox_plugin_calculator_decimal_seperator_comma")]
        Comma
    }
}