using System;
using System.Diagnostics;
using System.Globalization;
using System.IO;
using System.Text;
using System.Linq;
using System.Windows;
using System.Windows.Documents;
using Wox.Helper;
using Wox.Infrastructure;
using Wox.Infrastructure.Logger;
using Wox.Infrastructure.Exception;

namespace Wox
{
    internal partial class ReportWindow
    {
        public ReportWindow(Exception exception, string id)
        {
            InitializeComponent();
            ErrorTextbox.Document.Blocks.FirstBlock.Margin = new Thickness(0);
            SetException(exception, id);
        }

        private void SetException(Exception exception, string id)
        {
            string path = Log.CurrentLogDirectory;
            var directory = new DirectoryInfo(path);
            var log = directory.GetFiles().OrderByDescending(f => f.LastWriteTime).First();

            var paragraph = Hyperlink("You can help us to fix this issue by opening issue in: ", Constant.Issue);
            paragraph.Inlines.Add($"1. upload log file: {log.FullName}\n");
            paragraph.Inlines.Add($"2. copy below exception message");
            ErrorTextbox.Document.Blocks.Add(paragraph);

            var content = ExceptionFormatter.ExceptionWithRuntimeInfo(exception, id);
            paragraph = new Paragraph();
            paragraph.Inlines.Add(content);
            ErrorTextbox.Document.Blocks.Add(paragraph);
        }

        private Paragraph Hyperlink(string textBeforeUrl, string url)
        {
            var paragraph = new Paragraph();
            paragraph.Margin = new Thickness(0);

            var link = new Hyperlink { IsEnabled = true };
            link.Inlines.Add(url);
            link.NavigateUri = new Uri(url);
            link.RequestNavigate += (s, e) => Process.Start(e.Uri.ToString());
            link.Click += (s, e) => Process.Start(url);

            paragraph.Inlines.Add(textBeforeUrl);
            paragraph.Inlines.Add(link);
            paragraph.Inlines.Add("\n");

            return paragraph;
        }
    }
}
