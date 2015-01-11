﻿using System;
using System.Collections.Generic;
using System.Linq;
using System.Text;
using Wox.Infrastructure.Storage;

namespace Wox.ImageLoader
{
    [Serializable]
    public class ImageCacheStroage : BinaryStorage<ImageCacheStroage>
    {
        public int counter = 0;
        public const int maxCached = 200;
        public Dictionary<string, int> TopUsedImages = new Dictionary<string, int>();

        protected override string ConfigName
        {
            get { return "ImageCache"; }
        }

        public void Add(string path)
        {
            if (TopUsedImages.ContainsKey(path))
            {
                TopUsedImages[path] = TopUsedImages[path] + 1 ;
            }
            else
            {
                TopUsedImages.Add(path, 1);
            }

            if (TopUsedImages.Count > maxCached)
            {
                TopUsedImages = TopUsedImages.OrderByDescending(o => o.Value)
                    .Take(maxCached)
                    .ToDictionary(i => i.Key, i => i.Value);
            }

            if (++counter == 30)
            {
                counter = 0;
                Save();
            }
        }

        public void Remove(string path)
        {
            if (TopUsedImages.ContainsKey(path))
            {
                TopUsedImages.Remove(path);
            }
        }
    }
}
