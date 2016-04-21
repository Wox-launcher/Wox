using System.IO;
using System.Reflection;
using System.Threading;
using Newtonsoft.Json;
using Wox.Infrastructure.Exception;
using Wox.Infrastructure.Logger;

namespace Wox.Infrastructure.Storage
{
    /// <summary>
    /// Serialize object using json format.
    /// </summary>
    public class JsonStrorage<T> where T : new()
    {
        private T _json;
        private readonly JsonSerializerSettings _serializerSettings;

        internal JsonStrorage()
        {
            FileName = typeof(T).Name;
            DirectoryPath = Path.Combine(WoxDirectroy.Executable, DirectoryName);
            FilePath = Path.Combine(DirectoryPath, FileName + FileSuffix);
            _serializerSettings = new JsonSerializerSettings
            {
                // use property initialization instead of DefaultValueAttribute
                NullValueHandling = NullValueHandling.Ignore
            };
        }

        protected string FileName { get; set; }
        protected string FilePath { get; set; }
        protected const string FileSuffix = ".json";
        protected string DirectoryPath { get; set; }
        protected const string DirectoryName = "Config";


        public T Load()
        {
            if (!Directory.Exists(DirectoryPath))
            {
                Directory.CreateDirectory(DirectoryPath);
            }

            if (File.Exists(FilePath))
            {
                var searlized = File.ReadAllText(FilePath);
                if (!string.IsNullOrWhiteSpace(searlized))
                {
                    Deserializa(searlized);
                }
                else
                {
                    LoadDefault();
                }
            }
            else
            {
                LoadDefault();
            }

            return _json;
        }

        private void Deserializa(string searlized)
        {
            try
            {
                _json = JsonConvert.DeserializeObject<T>(searlized, _serializerSettings);
            }
            catch (JsonSerializationException e)
            {
                LoadDefault();
                Log.Warn($"Load default value, bacause can't deserialize file: {FileName}");
                Log.Warn(e.Message);
                Log.Warn(e.StackTrace);
            }

        }

        private void LoadDefault()
        {
            _json = JsonConvert.DeserializeObject<T>("{}", _serializerSettings);
            Save();
        }

        public void Save()
        {
            ThreadPool.QueueUserWorkItem(o =>
            {
                string jsonString = JsonConvert.SerializeObject(_json, Formatting.Indented);
                File.WriteAllText(FilePath, jsonString);
            });
        }

        private void Populate(T target, T input)
        {
            var type = typeof(T);
            var filds = type.GetFields(BindingFlags.Public);
            foreach (var fild in filds)
            {
                var value = fild.GetValue(input);
                fild.SetValue(target, value);
            }
        }
    }
}
