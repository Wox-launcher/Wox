﻿using System;
using System.Collections.Generic;
using System.IO;
using System.Linq;
using System.Runtime.Serialization.Formatters.Binary;
using System.Text;
using Wox.Infrastructure.Logger;

namespace Wox.Infrastructure.Storage
{
    /// <summary>
    /// Stroage object using binary data
    /// Normally, it has better performance, but not readable
    /// You MUST mark implement class as Serializable
    /// </summary>
    [Serializable]
    public abstract class BinaryStorage<T> : BaseStorage<T> where T : class, IStorage, new()
    {
        protected override string FileSuffix
        {
            get { return ".dat"; }
        }

        protected override void LoadInternal()
        {
            try
            {
                FileStream fileStream = new FileStream(ConfigPath, FileMode.Open, FileAccess.Read, FileShare.ReadWrite);
                BinaryFormatter binaryFormatter = new BinaryFormatter();
                serializedObject = binaryFormatter.Deserialize(fileStream) as T;
                fileStream.Close();
            }
            catch (Exception)
            {
                serializedObject = LoadDefault();
            }
        }

        protected override void SaveInternal()
        {
            try
            {
                FileStream fileStream = new FileStream(ConfigPath, FileMode.Create);
                BinaryFormatter binaryFormatter = new BinaryFormatter();
                binaryFormatter.Serialize(fileStream, serializedObject);
                fileStream.Close();
            }
            catch (Exception e)
            {
                Log.Error(e.Message);
                #if (DEBUG)
                {  
                    throw e;
                }
                #endif
            }
        }
    }
}
