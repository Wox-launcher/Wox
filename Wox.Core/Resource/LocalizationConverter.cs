using System;
using System.ComponentModel;
using System.Globalization;
using System.Reflection;
using System.Windows.Data;

namespace Wox.Core
{
    public class LocalizationConverter : IValueConverter
    {
        public object Convert(object value, Type targetType, object parameter, CultureInfo culture)
        {
            if (targetType == typeof(string) && value != null)
            {
                FieldInfo fi = value.GetType().GetField(value.ToString());
                if (fi != null)
                {
                    string localizedDescription = string.Empty;
                    var attributes = (DescriptionAttribute[])fi.GetCustomAttributes(typeof(DescriptionAttribute), false);
                    if ((attributes.Length > 0) && (!String.IsNullOrEmpty(attributes[0].Description)))
                    {
                        localizedDescription = attributes[0].Description;
                    }

                    return (!String.IsNullOrEmpty(localizedDescription)) ? localizedDescription : value.ToString();
                }
            }
            
            return string.Empty;
        }

        public object ConvertBack(object value, Type targetType, object parameter, CultureInfo culture)
        {
            throw new NotImplementedException();
        }
    }
}
