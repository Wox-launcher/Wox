using System;
using System.Windows.Forms;
using System.Windows.Media;
using System.Windows.Threading;
using NLog;

using Wox.Image;
using Wox.Infrastructure;
using Wox.Infrastructure.Logger;
using Wox.Plugin;


namespace Wox.ViewModel
{
    public class ResultViewModel : BaseModel
    {
        private static readonly Logger Logger = LogManager.GetCurrentClassLogger();

        public ResultViewModel(Result result)
        {
            if (result != null)
            {
                Result = result;
                Image = new Lazy<ImageSource>(() =>
                {
                    return SetImage(result);
                });
            }
        }

        private ImageSource SetImage(Result result)
        {
            string imagePath = result.IcoPath;
            if (string.IsNullOrEmpty(imagePath) && result.Icon != null)
            {
                var r = result;
                ImageSource func(string key)
                {
                    try
                    {
                        return r.Icon();
                    }
                    catch (Exception e)
                    {
                        Logger.WoxError($"IcoPath is empty and exception when calling Icon() for result <{r.Title}> of plugin <{r.PluginDirectory}>", e);
                        return ImageLoader.Load(Constant.ErrorIcon, UpdateImageCallback);
                    }
                }
                return ImageLoader.Load(result.Title, func, UpdateImageCallback);
            }
            else
            {
                // will get here either when icoPath has value\icon delegate is null\when had exception in delegate
                return ImageLoader.Load(imagePath, UpdateImageCallback);
            }
        }

        public void UpdateImageCallback(ImageSource image)
        {
            Image = new Lazy<ImageSource>(() => image);
            OnPropertyChanged(nameof(Image));
        }

        // directly binding will cause unnecessory image load
        // only binding get will cause load twice or more
        // so use lazy binding
        public Lazy<ImageSource> Image { get; set; }

        public Result Result { get; set; }

        public override bool Equals(object obj)
        {
            var r = obj as ResultViewModel;
            if (r != null)
            {
                return Result.Equals(r.Result);
            }
            else
            {
                return false;
            }
        }

        public override int GetHashCode()
        {
            return Result.GetHashCode();
        }

        public override string ToString()
        {
            return Result.ToString();
        }

    }
}
