using System;
using System.IO;
using System.Reflection;
using System.Runtime.Serialization.Formatters;
using System.Runtime.Serialization.Formatters.Binary;
using System.Threading;
using Wox.Infrastructure.Exception;
using Wox.Infrastructure.Logger;

namespace Wox.Infrastructure.Storage
{
    /// <summary>
    /// Stroage object using binary data
    /// Normally, it has better performance, but not readable
    /// You MUST mark implement class as Serializable
    /// </summary>
    public class BinaryStorage<T> where T : class, new()
    {
        private T _binary;
        private string FileSuffix => ".dat";

        private string DirectoryPath { get; }

        private string FilePath;

        private string FileName { get; }

        public BinaryStorage()
        {
            FileName = typeof(T).Name;
            DirectoryPath = Path.Combine(WoxDirectroy.Executable, "Config");
            FilePath = Path.Combine(DirectoryPath, FileName + FileSuffix);;
        }

        public  T Load()
        {
            if (!File.Exists(FilePath))
            {
                if (!Directory.Exists(DirectoryPath))
                {
                    Directory.CreateDirectory(DirectoryPath);
                }
                File.Create(FilePath).Close();
            }
            Deserializa();
            return _binary;
        }

        private void Deserializa()
        {
            //http://stackoverflow.com/questions/2120055/binaryformatter-deserialize-gives-serializationexception
            AppDomain.CurrentDomain.AssemblyResolve += CurrentDomain_AssemblyResolve;
            try
            {
                using (FileStream fileStream = new FileStream(FilePath, FileMode.Open, FileAccess.Read, FileShare.ReadWrite))
                {
                    if (fileStream.Length > 0)
                    {
                        BinaryFormatter binaryFormatter = new BinaryFormatter
                        {
                            AssemblyFormat = FormatterAssemblyStyle.Simple
                        };
                        _binary = binaryFormatter.Deserialize(fileStream) as T;
                        if (_binary == null)
                        {
                            _binary = new T();
#if (DEBUG)
                            {
                                throw new WoxException("deserialize failed");
                            }
#endif
                        }
                    }
                    else
                    {
                        _binary = new T();
                    }
                }
            }
            catch (System.Exception e)
            {
                Log.Error(e);
                _binary = new T();
#if (DEBUG)
                {
                    throw;
                }
#endif
            }
            finally
            {
                AppDomain.CurrentDomain.AssemblyResolve -= CurrentDomain_AssemblyResolve;
            }
        }

        private Assembly CurrentDomain_AssemblyResolve(object sender, ResolveEventArgs args)
        {
            Assembly ayResult = null;
            string sShortAssemblyName = args.Name.Split(',')[0];
            Assembly[] ayAssemblies = AppDomain.CurrentDomain.GetAssemblies();
            foreach (Assembly ayAssembly in ayAssemblies)
            {
                if (sShortAssemblyName == ayAssembly.FullName.Split(',')[0])
                {
                    ayResult = ayAssembly;
                    break;
                }
            }
            return ayResult;
        }

        public void Save()
        {
            ThreadPool.QueueUserWorkItem(o =>
            {
                try
                {
                    FileStream fileStream = new FileStream(FilePath, FileMode.Create);
                    BinaryFormatter binaryFormatter = new BinaryFormatter
                    {
                        AssemblyFormat = FormatterAssemblyStyle.Simple
                    };
                    binaryFormatter.Serialize(fileStream, _binary);
                    fileStream.Close();
                }
                catch (System.Exception e)
                {
                    Log.Error(e);
#if (DEBUG)
                    {
                        throw;
                    }
#endif
                }
            });
        }
    }
}
