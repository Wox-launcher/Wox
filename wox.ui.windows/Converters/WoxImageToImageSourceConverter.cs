using System;
using System.Globalization;
using System.Windows.Data;
using System.Windows.Media;
using Wox.UI.Windows.Models;
using Wox.UI.Windows.Services;

namespace Wox.UI.Windows.Converters;

public class WoxImageToImageSourceConverter : IValueConverter
{
    public object? Convert(object value, Type targetType, object parameter, CultureInfo culture)
    {
        if (value is WoxImage woxImage)
        {
            return ImageService.ConvertToImageSource(woxImage);
        }
        return null;
    }

    public object ConvertBack(object value, Type targetType, object parameter, CultureInfo culture)
    {
        throw new NotImplementedException();
    }
}
