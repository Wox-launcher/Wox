using System;
using System.Globalization;
using System.Windows.Data;

namespace Wox.Converters
{
    public class DivideConverter : IValueConverter
    {
        public object Convert(object value, Type targetType, object parameter, CultureInfo culture)
        {
            return (dynamic)value / System.Convert.ToDouble(parameter);
        }

        public object ConvertBack(object value, Type targetType, object parameter, CultureInfo culture)
        {
            return (dynamic)value * System.Convert.ToDouble(parameter);
        }
    }
}
