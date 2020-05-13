using JetBrains.Annotations;
using NLog;
using System;
using System.Collections.Concurrent;
using System.Collections.Generic;
using System.Linq;
using System.Text;
using System.Threading;
using System.Threading.Tasks;
using System.Windows.Forms;
using System.Windows.Media;
using System.Windows.Media.Imaging;

using Wox.Infrastructure.Logger;

namespace Wox.Infrastructure.Image
{
    class CacheEntry
    {
        internal ImageSource Image;
        internal DateTime ExpiredDate;
        internal DateTime LastUsedDate;

        public CacheEntry(ImageSource image, DateTime expiredDate)
        {
            Image = image;
            ExpiredDate = expiredDate;
            LastUsedDate = DateTime.Now;
        }
    }

    class ImageCache
    {
        private readonly TimeSpan _expiredTime = new TimeSpan(24, 0, 0);
        private readonly TimeSpan _checkInterval = new TimeSpan(0, 1, 0);
        private readonly ConcurrentDictionary<String, CacheEntry> _cache;
        private readonly System.Threading.Timer timer;

        private static readonly NLog.Logger Logger = LogManager.GetCurrentClassLogger();

        public ImageCache()
        {
            _cache = new ConcurrentDictionary<string, CacheEntry>();
            timer = new System.Threading.Timer(ExpirationCheck, null, _checkInterval, _checkInterval);
        }

        private void ExpirationCheck(object state)
        {
            DateTime now = DateTime.Now;
            Logger.WoxDebug($"ExpirationCheck start {now}");
            List<KeyValuePair<string, CacheEntry>> pairs = _cache.Where(pair => now > pair.Value.ExpiredDate).ToList();

            foreach (KeyValuePair<string, CacheEntry> pair in pairs)
            {
                bool success = _cache.TryRemove(pair.Key, out CacheEntry entry);
                Logger.WoxDebug($"removed success: <{success}> entry: <{pair.Key}>");
                throw new System.Exception("test timer exception caught");
            }
        }


        private CacheEntry GetEntryFactory(string key, Func<string, ImageSource> imageFactory)
        {
            DateTime expiredDate = DateTime.Now + _expiredTime;
            ImageSource image = imageFactory(key);
            CacheEntry entry = new CacheEntry(image, expiredDate);
            return entry;
        }

        public ImageSource GetOrAdd(string key, Func<string, ImageSource> imageFactory)
        {
            CacheEntry entry = _cache.GetOrAdd(key, (k) => { return GetEntryFactory(k, imageFactory); });
            return entry.Image;
        }
    }
}
