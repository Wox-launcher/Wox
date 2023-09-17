using System.Globalization;
using System.Runtime.InteropServices;
using System.Text.RegularExpressions;
using Mages.Core;
using TextCopy;

namespace Wox.Plugin.Calculator;

public class Main : IPlugin
{
    private static readonly Regex RegValidExpressChar = new(
        @"^(" +
        @"ceil|floor|exp|pi|e|max|min|det|abs|log|ln|sqrt|" +
        @"sin|cos|tan|arcsin|arccos|arctan|" +
        @"eigval|eigvec|eig|sum|polar|plot|round|sort|real|zeta|" +
        @"bin2dec|hex2dec|oct2dec|" +
        @"==|~=|&&|\|\||" +
        @"[ei]|[0-9]|[\+\-\*\/\^\., ""]|[\(\)\|\!\[\]]" +
        @")+$", RegexOptions.Compiled);

    private static readonly Regex RegBrackets = new(@"[\(\)\[\]]", RegexOptions.Compiled);
    private static readonly Engine MagesEngine;

    static Main()
    {
        MagesEngine = new Engine();
    }

    private PluginInitContext Context { get; set; } = null!;

    public void Init(PluginInitContext context)
    {
        Context = context;
    }

    public async Task<List<Result>> Query(Query query)
    {
        if (!CanCalculate(query)) return new List<Result>();

        try
        {
            var expression = query.Search.Replace(",", ".");
            var result = MagesEngine.Interpret(expression);

            if (result.ToString() == "NaN")
                result = Context.API.GetTranslation("wox_plugin_calculator_not_a_number");

            if (result is Function)
                result = Context.API.GetTranslation("wox_plugin_calculator_expression_not_complete");

            if (!string.IsNullOrEmpty(result?.ToString()))
            {
                var roundedResult = Math.Round(Convert.ToDecimal(result), 10, MidpointRounding.AwayFromZero);
                var newResult = ChangeDecimalSeparator(roundedResult, GetDecimalSeparator());

                return await Task.Run(() => new List<Result>
                {
                    new()
                    {
                        Title = newResult,
                        SubTitle = Context.API.GetTranslation("wox_plugin_calculator_copy_number_to_clipboard"),
                        Icon = WoxImage.FromRelativeToPluginPath("Images/calculator.png"),
                        Score = 300,
                        Action = () =>
                        {
                            try
                            {
                                ClipboardService.SetText(newResult);
                                return true;
                            }
                            catch (ExternalException)
                            {
                                Context.API.ShowMsg("Copy failed, please try later");
                                return false;
                            }
                        }
                    }
                });
            }
        }
        catch
        {
            // ignored
        }

        return new List<Result>();
    }

    private bool CanCalculate(Query query)
    {
        // Don't execute when user only input "e" or "i" keyword
        if (query.Search.Length < 2) return false;

        if (!RegValidExpressChar.IsMatch(query.Search)) return false;

        if (!IsBracketComplete(query.Search)) return false;

        return true;
    }

    private string ChangeDecimalSeparator(decimal value, string newDecimalSeparator)
    {
        if (string.IsNullOrEmpty(newDecimalSeparator)) return value.ToString();

        var numberFormatInfo = new NumberFormatInfo
        {
            NumberDecimalSeparator = newDecimalSeparator
        };
        return value.ToString(numberFormatInfo);
    }

    private string GetDecimalSeparator()
    {
        return CultureInfo.CurrentCulture.NumberFormat.NumberDecimalSeparator;
    }

    private bool IsBracketComplete(string query)
    {
        var matchs = RegBrackets.Matches(query);
        var leftBracketCount = 0;
        foreach (Match match in matchs)
            if (match.Value == "(" || match.Value == "[")
                leftBracketCount++;
            else
                leftBracketCount--;

        return leftBracketCount == 0;
    }

    public string GetTranslatedPluginTitle()
    {
        return Context.API.GetTranslation("wox_plugin_caculator_plugin_name");
    }

    public string GetTranslatedPluginDescription()
    {
        return Context.API.GetTranslation("wox_plugin_caculator_plugin_description");
    }
}