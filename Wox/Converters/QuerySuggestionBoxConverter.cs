using System;
using System.Globalization;
using System.Windows.Data;
using NLog;
using Wox.Infrastructure.Logger;
using Wox.ViewModel;

namespace Wox.Converters
{
    public class QuerySuggestionBoxConverter : IMultiValueConverter
    {
        private static readonly Logger Logger = LogManager.GetCurrentClassLogger();

        public object Convert(object[] values, Type targetType, object parameter, CultureInfo culture)
        {
            if (values.Length != 2)
            {
                return string.Empty;
            }

            // first prop is the current query string
            var queryText = (string)values[0];

            if (string.IsNullOrEmpty(queryText))
            {
                return string.Empty;
            }

            // second prop is the current selected item result
            var val = values[1];
            if (val == null)
            {
                return string.Empty;
            }
            if (!(val is ResultViewModel))
            {
                return System.Windows.Data.Binding.DoNothing;
            }

            try
            {
                var selectedItem = (ResultViewModel)val;

                var selectedResult = selectedItem.Result;
                var selectedResultActionKeyword = string.IsNullOrEmpty(selectedResult.ActionKeywordAssigned) ? "" : selectedResult.ActionKeywordAssigned + " ";
                var selectedResultPossibleSuggestion = selectedResultActionKeyword + selectedResult.Title;

                if (!selectedResultPossibleSuggestion.StartsWith(queryText, StringComparison.CurrentCultureIgnoreCase))
                    return string.Empty;

                // When user typed lower case and result title is uppercase, we still want to display suggestion
                var textConverter = new MultilineTextConverter();
                return textConverter.Convert(queryText + selectedResultPossibleSuggestion.Substring(queryText.Length), null, null, culture);
            }
            catch (Exception e)
            {
                Logger.WoxError("fail to convert text for suggestion box", e);
                return string.Empty;
            }
        }

        public object[] ConvertBack(object value, Type[] targetTypes, object parameter, CultureInfo culture)
        {
            throw new NotImplementedException();
        }
    }
}