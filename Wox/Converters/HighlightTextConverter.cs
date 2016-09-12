using System;
using System.Collections.Generic;
using System.Globalization;
using System.Linq;
using System.Windows;
using System.Windows.Data;
using System.Windows.Documents;

namespace Wox.Converters
{
    public class HighlightTextConverter : IMultiValueConverter
    {
        public object Convert(object[] value, Type targetType, object parameter, CultureInfo cultureInfo)
        {
            var text = value[0] as string;
            var hdata = value[1] as List<int>;
            
            var textBlock = new Span();

            if (hdata == null || !hdata.Any())
            {
                // no highlight data, just return the text
                return new Run(text);
            }

            for (var i = 0; i < text.Length; i++)
            {
                var ch = text.Substring(i, 1);
                // should this character be highlighted?
                if (hdata.Contains(i))
                {
                    textBlock.Inlines.Add(new Bold(new Run(ch)));
                }
                else
                {
                    textBlock.Inlines.Add(new Run(ch));
                }
            }
            return textBlock;
        }

        public object[] ConvertBack(object value, Type[] targetType, object parameter, CultureInfo culture)
        {
            return new[] { DependencyProperty.UnsetValue, DependencyProperty.UnsetValue };
        }
    }
}
