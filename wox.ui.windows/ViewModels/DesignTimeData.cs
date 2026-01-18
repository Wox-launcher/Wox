using System.Collections.ObjectModel;
using Wox.UI.Windows.Models;

namespace Wox.UI.Windows.ViewModels;

/// <summary>
/// 用于独立测试 UI 的设计时数据
/// 不依赖 wox.core，可以直接运行查看 UI 效果
/// </summary>
public static class DesignTimeData
{
    public static MainViewModel CreateSampleViewModel()
    {
        var vm = new MainViewModel
        {
            QueryText = "calculator 2+2"
        };

        // 添加示例结果
        var sampleResults = new ObservableCollection<ResultItem>
        {
            new ResultItem
            {
                Id = "1",
                Title = "4",
                SubTitle = "2 + 2 = 4",
                Score = 100,
                Actions = new List<ResultAction>
                {
                    new ResultAction
                    {
                        Id = "copy",
                        Name = "Copy to clipboard",
                        IsDefault = true
                    }
                }
            },
            new ResultItem
            {
                Id = "2",
                Title = "Calculator",
                SubTitle = "Windows Calculator",
                Score = 90,
                Icon = new WoxImage
                {
                    ImageType = "file",
                    ImageData = "C:\\Windows\\System32\\calc.exe"
                },
                Actions = new List<ResultAction>
                {
                    new ResultAction
                    {
                        Id = "open",
                        Name = "Open",
                        IsDefault = true
                    }
                }
            },
            new ResultItem
            {
                Id = "3",
                Title = "Google Search: calculator 2+2",
                SubTitle = "Search on Google",
                Score = 80,
                Preview = new Preview
                {
                    PreviewType = "text",
                    PreviewData = "Preview: This will open Google search with 'calculator 2+2' query"
                },
                Actions = new List<ResultAction>
                {
                    new ResultAction
                    {
                        Id = "search",
                        Name = "Search",
                        IsDefault = true
                    }
                }
            },
            new ResultItem
            {
                Id = "4",
                Title = "Wolfram Alpha",
                SubTitle = "Advanced computational engine",
                Score = 75
            },
            new ResultItem
            {
                Id = "5",
                Title = "Unit Converter",
                SubTitle = "Convert units and measurements",
                Score = 70
            }
        };

        foreach (var result in sampleResults)
        {
            vm.Results.Add(result);
        }

        // 默认选中第一个
        if (vm.Results.Count > 0)
        {
            vm.SelectedIndex = 0;
            vm.SelectedResult = vm.Results[0];
        }

        return vm;
    }

    /// <summary>
    /// 测试长文本结果
    /// </summary>
    public static ObservableCollection<ResultItem> CreateLongTextResults()
    {
        return new ObservableCollection<ResultItem>
        {
            new ResultItem
            {
                Id = "1",
                Title = "This is a very long title that should be truncated with ellipsis when it exceeds the available width",
                SubTitle = "And this is an even longer subtitle that contains a lot of information about this particular result item and it should also be truncated properly",
                Score = 100
            },
            new ResultItem
            {
                Id = "2",
                Title = "多语言测试：这是一个很长的中文标题，用来测试中文字符的显示和截断效果",
                SubTitle = "副标题也包含中文内容：这是用来测试混合字符（Chinese中文、English、日本語）的显示效果",
                Score = 90
            }
        };
    }

    /// <summary>
    /// 测试图标结果
    /// </summary>
    public static ObservableCollection<ResultItem> CreateIconResults()
    {
        return new ObservableCollection<ResultItem>
        {
            new ResultItem
            {
                Id = "1",
                Title = "File Icon",
                SubTitle = "Using file path icon",
                Icon = new WoxImage
                {
                    ImageType = "file",
                    ImageData = "C:\\Windows\\System32\\notepad.exe"
                },
                Score = 100
            },
            new ResultItem
            {
                Id = "2",
                Title = "Base64 Icon",
                SubTitle = "Using base64 encoded icon",
                Icon = new WoxImage
                {
                    ImageType = "base64",
                    // 1x1 红色像素的 PNG (base64)
                    ImageData = "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8z8DwHwAFBQIAX8jx0gAAAABJRU5ErkJggg=="
                },
                Score = 90
            }
        };
    }

    /// <summary>
    /// 测试预览面板
    /// </summary>
    public static ObservableCollection<ResultItem> CreatePreviewResults()
    {
        return new ObservableCollection<ResultItem>
        {
            new ResultItem
            {
                Id = "1",
                Title = "Text Preview",
                SubTitle = "Simple text preview",
                Preview = new Preview
                {
                    PreviewType = "text",
                    PreviewData = "This is a simple text preview.\n\nIt can contain multiple lines.\n\nAnd even more content..."
                },
                Score = 100
            },
            new ResultItem
            {
                Id = "2",
                Title = "Code Preview",
                SubTitle = "Code snippet preview",
                Preview = new Preview
                {
                    PreviewType = "text",
                    PreviewData = "public class HelloWorld\n{\n    public static void Main()\n    {\n        Console.WriteLine(\"Hello, Wox!\");\n    }\n}"
                },
                Score = 90
            },
            new ResultItem
            {
                Id = "3",
                Title = "Long Preview",
                SubTitle = "Very long preview content",
                Preview = new Preview
                {
                    PreviewType = "text",
                    PreviewData = string.Join("\n\n", Enumerable.Repeat("This is a very long preview content that should demonstrate scrolling behavior in the preview panel.", 20))
                },
                Score = 80
            }
        };
    }
}
