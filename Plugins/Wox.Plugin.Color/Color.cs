using System;
using System.Collections.Generic;
using System.Drawing;
using System.Drawing.Imaging;
using System.IO;
using System.Linq;
using System.Windows;
using System.Text.RegularExpressions;

namespace Wox.Plugin.Color
{
    public sealed class ColorsPlugin : IPlugin, IPluginI18n
    {
        private string DIR_PATH = Path.Combine(Path.GetTempPath(), @"Plugins\Colors\");
        private PluginInitContext context;
        private const int IMG_SIZE = 32;
        private const string RGB_PATTERN = ("rgb\\( *([0-9]{1,3}) *, *([0-9]{1,3}) *, *([0-9]{1,3}) *\\)");

        private DirectoryInfo ColorsDirectory { get; set; }

        public ColorsPlugin()
        {
            if (!Directory.Exists(DIR_PATH))
            {
                ColorsDirectory = Directory.CreateDirectory(DIR_PATH);
            }
            else
            {
                ColorsDirectory = new DirectoryInfo(DIR_PATH);
            }
        }

        public List<Result> Query(Query query)
        {
            var raw = query.Search;
            if (!IsAvailable(raw)) return new List<Result>(0);
            try
            {
                System.Drawing.Color color;
                Match rgbMatch = Regex.Match(raw, RGB_PATTERN, RegexOptions.IgnoreCase);
                if (rgbMatch.Success)
                {
                    color = System.Drawing.Color.FromArgb(Convert.ToInt16(rgbMatch.Groups[1].Value), Convert.ToInt16(rgbMatch.Groups[2].Value),
                        Convert.ToInt16(rgbMatch.Groups[3].Value));
                }
                else
                {
                    color = ColorTranslator.FromHtml(raw);
                }
                var cached = Find(color);
                if (cached.Length == 0)
                {
                    var path = CreateImage(color);
                    return new List<Result>
                    {
                        new Result
                        {
                            Title = raw,
                            SubTitle = rgbMatch.Success ? $"#{color.Name.Remove(0, 2).ToUpper()}" :
                                $"RGB({color.R},{color.G},{color.B})",
                            IcoPath = path,
                            Action = _ =>
                            {
                                Clipboard.SetText(raw);
                                return true;
                            }
                        }
                    };
                }
                return cached.Select(x => new Result
                {
                    Title = raw,
                    SubTitle = rgbMatch.Success ? $"#{color.Name.Remove(0, 2).ToUpper()}" :
                                $"RGB({color.R},{color.G},{color.B})",
                    IcoPath = x.FullName,
                    Action = _ =>
                    {
                        Clipboard.SetText(raw);
                        return true;
                    }
                }).ToList();
            }
            catch (Exception exception)
            {
                // todo: log
                return new List<Result>(0);
            }
        }

        private bool IsAvailable(string query)
        {
            // todo: names
            var hexLength = query.Length - 1; // minus `#` sign
            return (query.StartsWith("#") && (hexLength == 3 || hexLength == 6)) || (query.ToLower().StartsWith("rgb") && Regex.IsMatch(query, RGB_PATTERN, RegexOptions.IgnoreCase));
        }

        public FileInfo[] Find(System.Drawing.Color color)
        {
            var file = string.Format("{0}.png", color.Name.Remove(0,2));
            return ColorsDirectory.GetFiles(file, SearchOption.TopDirectoryOnly);
        }

        private string CreateImage(System.Drawing.Color color)
        {
            using (var bitmap = new Bitmap(IMG_SIZE, IMG_SIZE))
            using (var graphics = Graphics.FromImage(bitmap))
            {
                graphics.Clear(color);

                var path = CreateFileName(color.Name.Remove(0, 2));
                bitmap.Save(path, ImageFormat.Png);
                return path;
            }
        }

        private string CreateFileName(string name)
        {
            return string.Format("{0}{1}.png", ColorsDirectory.FullName, name);
        }

        public void Init(PluginInitContext context)
        {
            this.context = context;
        }


        public string GetTranslatedPluginTitle()
        {
            return context.API.GetTranslation("wox_plugin_color_plugin_name");
        }

        public string GetTranslatedPluginDescription()
        {
            return context.API.GetTranslation("wox_plugin_color_plugin_description");
        }
    }
}