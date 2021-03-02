using System;
using System.Collections.Generic;
using System.Globalization;
using System.IO;
using System.Linq;
using System.Runtime.InteropServices;
using System.Text;
using Newtonsoft.Json.Linq;

namespace Wox.Infrastructure
{
    public static class KeyboardLayout
    {
        #region DllImport

        [DllImport("user32.dll")] private static extern UInt32 GetKeyboardLayoutList(Int32 nBuff, IntPtr[] lpList);
        [DllImport("user32.dll")] private static extern IntPtr GetForegroundWindow();
        [DllImport("user32.dll")] private static extern uint GetWindowThreadProcessId(IntPtr hwnd, IntPtr proccess);
        [DllImport("user32.dll")] private static extern IntPtr GetKeyboardLayout(uint thread);

        #endregion
        
        /// <summary>
        ///     Convert layout of given input to all user-installed languages.
        /// </summary>
        /// <param name="input"></param>
        /// <param name="layoutDir">Directory with the mappings of layouts.</param>
        /// <returns></returns>
        public static ConvertOutput<List<string>> Convert(string input, string layoutDir)
        {
            if (string.IsNullOrEmpty(input)) return new ConvertOutput<List<string>>()
            {
                Result = new List<string>(),
                State = State.Fail,
                Message = "Empty string."
            };

            input = input.ToLower();

            var paths = LayoutPaths(layoutDir);

            var currentMapping = CurrentLayoutMapping(paths);
            if (currentMapping == null) return new ConvertOutput<List<string>>()
            {
                Result = new List<string>() {input},
                State = State.Fail,
                Message = "This method works right if both languages for input and system are equal." +
                          "Check if you keyboard language is changed while building, testing etc."
            };

            // read keys for input
            var keys = FindKeys(input, currentMapping.Properties());

            // find letters for keys of input string 
            var mInput = GetLetters(keys, paths);

            
            return new ConvertOutput<List<string>>()
            {
                Result = mInput,
                State = State.Success
            };
        }

        /// <summary>
        ///     Get letter for virtual key code.
        /// </summary>
        /// <param name="key">Virtual key code.</param>
        /// <param name="jObject">JObject for mapping file.</param>
        /// <returns></returns>
        private static string GetLetter(string key, JObject jObject)
        {
            foreach (var prop in jObject.Properties())
            {
                if (key == prop.Name)
                    return prop.Value.ToString();
                else if (key == "SPACE") return " ";
            }

            return "";
        }

        /// <summary>
        ///     Get letters for virtual key codes.
        /// </summary>
        /// <param name="keys">Virtual key codes.</param>
        /// <param name="paths">Paths to the mapping file where letters should be.</param>
        /// <returns></returns>
        private static List<string> GetLetters(List<string> keys, List<string> paths)
        {
            var mInput = new List<string>();

            foreach (var path in paths)
            {
                var tempInput = new StringBuilder();
                var jObject = JObject.Parse(File.ReadAllText(path));
                foreach (var key in keys)
                {
                    var letter = GetLetter(key, jObject);
                    tempInput.Append(letter);
                }

                mInput.Add(tempInput.ToString());
            }

            return mInput.Distinct().ToList();
        }

        /// <summary>
        ///     Reads the key for given input in current user keyboard layout.
        /// </summary>
        /// <param name="input">Given input</param>
        /// <param name="props">Json properties</param>
        /// <returns>List of virtual key codes.</returns>
        private static List<string> FindKeys(string input, IEnumerable<JProperty> props)
        {
            var keys = new List<string>();
            for (var i = 0; i < input.Length; i++)
            {
                var tempLetter = input[i].ToString();
                foreach (var prop in props)
                    if (tempLetter == prop.Value.ToString())
                        keys.Add(prop.Name);
                    else if (input[i] == ' ') keys.Add("SPACE");
            }

            return keys;
        }

        /// <summary>
        ///     List of mapping paths for user-installed keyboard layouts.
        /// </summary>
        /// <param name="layoutDir">Directory which contain all mappings.</param>
        private static List<string> LayoutPaths(string layoutDir)
        {
            var userLayoutIds = GetUserKeyboardLayoutsList();

            var allLayoutFiles = Directory.EnumerateFiles(layoutDir);
            var paths = new List<string>();
            foreach (var id in userLayoutIds.Distinct())
            {
                var path = allLayoutFiles.First(x => x.Contains(id.ToString()));
                paths.Add(path);
            }

            return paths;
        }

        /// <summary>
        ///     JObject for current keyboard layout.
        /// </summary>
        /// <param name="paths">Paths to the mapping files.</param>
        private static JObject CurrentLayoutMapping(List<string> paths)
        {
            var haveCurrent = int.TryParse(GetCurrentKeyboardLayout(), out var currentLayout);
            if (!haveCurrent) return null;
            var currentLayoutFileName = paths.First(x => x.Contains(currentLayout.ToString()));

            return JObject.Parse(File.ReadAllText(currentLayoutFileName));
        }

        /// <summary>
        ///     Current user keyboard layout.
        /// </summary>
        /// <returns></returns>
        private static string GetCurrentKeyboardLayout()
        {
            var foregroundWindow = GetForegroundWindow();
            var foregroundProcess = GetWindowThreadProcessId(foregroundWindow, IntPtr.Zero);
            var keyboardLayout = GetKeyboardLayout(foregroundProcess).ToInt32() & 0xFFFF;
            try
            {
                var cultureInfo = new CultureInfo(keyboardLayout);
                return $"{cultureInfo.LCID.ToString()}";
            }
            catch (System.Exception)
            {
                var gklInt = GetKeyboardLayout(foregroundProcess).ToInt32();
                return $"Exception for keyboard layout: {keyboardLayout}";
            }
        }

        /// <summary>
        ///     User-installed keyboard layouts.
        /// </summary>
        /// <returns></returns>
        private static List<int> GetUserKeyboardLayoutsList()
        {
            var count = GetKeyboardLayoutList(0, null);
            var ids = new IntPtr[count];
            GetKeyboardLayoutList(ids.Length, ids);

            var lcid = new List<int>(ids.Length);
            foreach (var id in ids)
                lcid.Add(id.ToInt32() & 0xFFFF);

            return lcid;
        }
    }
    
    /// <summary>
    /// Provide state of convert layout operation.
    /// </summary>
    /// <typeparam name="T"></typeparam>
    public class ConvertOutput<T>
    {
        public T Result { get; set; }
        public State State { get; set; }
        /// <summary>
        /// Check message if State is Fail.
        /// </summary>
        public string Message { get; set; }
    }

    public enum State
    {
        Success,
        Fail
    }
}