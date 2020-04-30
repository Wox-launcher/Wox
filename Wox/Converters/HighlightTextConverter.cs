using System;
using System.Collections.Generic;
using System.Globalization;
using System.Linq;
using System.Text;
using System.Threading.Tasks;
using System.Windows;
using System.Windows.Data;
using System.Windows.Documents;
using System.Windows.Media;
using Wox.Core.Resource;

namespace Wox.Converters
{
    public class HighlightTextConverter : IMultiValueConverter
    {
        public object Convert(object[] value, Type targetType, object parameter, CultureInfo cultureInfo)
        {
            var text = value[0] as string;
            var highlightData = value[1] as List<int>;
            var selected = value[2] as bool? == true;

            var textBlock = new Span();

            if (highlightData == null || !highlightData.Any())
            {
                // No highlight data, just return the text
                return new Run(text);
            }

            var settings = (Application.Current as App).Settings;
            ResourceDictionary resources = Application.Current.Resources;

            var highlightColor = (Brush) (selected?
                resources.Contains("ItemSelectedHighlightColor")? resources["ItemSelectedHighlightColor"]: resources["BaseItemSelectedHighlightColor"]:
                resources.Contains("ItemHighlightColor")? resources["ItemHighlightColor"]: resources["BaseItemHighlightColor"]);
            var highlightStyle = FontHelper.GetFontStyleFromInvariantStringOrNormal(settings.ResultHighlightFontStyle);
            var highlightWeight = FontHelper.GetFontWeightFromInvariantStringOrNormal(settings.ResultHighlightFontWeight);
            var highlightStretch = FontHelper.GetFontStretchFromInvariantStringOrNormal(settings.ResultHighlightFontStretch);

            for (var i = 0; i < text.Length; i++)
            {
                var currentCharacter = text.Substring(i, 1);
                if (this.ShouldHighlight(highlightData, i))
                {
                    textBlock.Inlines.Add((new Run(currentCharacter)
                    {
                        Foreground = highlightColor,
                        FontWeight = highlightWeight,
                        FontStyle = highlightStyle,
                        FontStretch = highlightStretch
                    }));
                }
                else
                {
                    textBlock.Inlines.Add(new Run(currentCharacter));
                }
            }
            return textBlock;
        }

        public object[] ConvertBack(object value, Type[] targetType, object parameter, CultureInfo culture)
        {
            return new[] { DependencyProperty.UnsetValue, DependencyProperty.UnsetValue };
        }

        private bool ShouldHighlight(List<int> highlightData, int index)
        {
            return highlightData.Contains(index);
        }
    }
}
