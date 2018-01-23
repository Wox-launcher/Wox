using System;
using System.Globalization;
using System.Linq;
using System.Windows.Data;

namespace Wox.Converters
{
    public class SwitchPositionConverter : IMultiValueConverter
    {
        public object Convert(object[] values, Type targetType, object parameter, CultureInfo culture)
        {
            return values.OfType<double>().Aggregate(1.0, (current, result) => current * result);
        }

        public object[] ConvertBack(object value, Type[] targetTypes, object parameter, CultureInfo culture)
        {
            return null;
        }
    }
}
