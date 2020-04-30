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
using Wox.Infrastructure.UserSettings;

namespace Wox.Converters
{
    internal class HightLightStyle
    {
        public Brush Color { get; set; }
        public FontStyle FontStyle { get; set; }
        public FontWeight FontWeight { get; set; }
        public FontStretch FontStretch { get; set; }

        internal HightLightStyle(bool selected)
        {
            var app = Application.Current as App;
            Settings settings = app.Settings;
            ResourceDictionary resources = app.Resources;

            Color = (Brush)(selected ?
                resources.Contains("ItemSelectedHighlightColor") ? resources["ItemSelectedHighlightColor"] : resources["BaseItemSelectedHighlightColor"] :
                resources.Contains("ItemHighlightColor") ? resources["ItemHighlightColor"] : resources["BaseItemHighlightColor"]);
            FontStyle = FontHelper.GetFontStyleFromInvariantStringOrNormal(settings.ResultHighlightFontStyle);
            FontWeight = FontHelper.GetFontWeightFromInvariantStringOrNormal(settings.ResultHighlightFontWeight);
            FontStretch = FontHelper.GetFontStretchFromInvariantStringOrNormal(settings.ResultHighlightFontStretch);
        }

    }
    public class HighlightTextConverter : IMultiValueConverter
    {
        private static Lazy<HightLightStyle> _highLightStyle = new Lazy<HightLightStyle>(() => new HightLightStyle(false));
        private static Lazy<HightLightStyle> _highLightSelectedStyle = new Lazy<HightLightStyle>(() => new HightLightStyle(true));

        public object Convert(object[] value, Type targetType, object parameter, CultureInfo cultureInfo)
        {
            var text = value[0] as string;
            var highlightData = value[1] as List<int>;
            var selected = value[2] as bool? == true;


            if (highlightData == null || !highlightData.Any())
            {
                // No highlight data, just return the text
                return new Run(text);
            }

            HightLightStyle style;
            if (selected)
            {
                style = _highLightSelectedStyle.Value;
            }
            else
            {
                style = _highLightStyle.Value;
            }
            
            var textBlock = new Span();
            for (var i = 0; i < text.Length; i++)
            {
                var currentCharacter = text.Substring(i, 1);
                if (this.ShouldHighlight(highlightData, i))
                {
                    textBlock.Inlines.Add((new Run(currentCharacter)
                    {
                        Foreground = style.Color,
                        FontWeight = style.FontWeight,
                        FontStyle = style.FontStyle,
                        FontStretch = style.FontStretch
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
