﻿using Microsoft.Win32;
using System;
using System.IO;
using System.Collections.Generic;
using System.Linq;
using System.Runtime.InteropServices;
using System.Text;
using System.Threading;
using Wox.Infrastructure.Logger;
using Wox.Plugin.Everything.Everything.Exceptions;
using System.Diagnostics;

namespace Wox.Plugin.Everything.Everything
{
    public sealed class EverythingApi
    {
        public enum StateCode
        {
            OK,
            MemoryError,
            IPCError,
            RegisterClassExError,
            CreateWindowError,
            CreateThreadError,
            InvalidIndexError,
            InvalidCallError
        }
        public enum RequestFlag
        {
            FileName = 0x00000001,
            Path = 0x00000002,
            FullPathAndFileName = 0x00000004,
            Extenssion = 0x00000008,
            Size = 0x00000010,
            DateCreated = 0x00000020,
            DateModified = 0x00000040,
            DateAccessed = 0x00000080,
            Attributes = 0x00000100,
            FileListFileName = 0x00000200,
            RunCount = 0x00000400,
            DateRun = 0x00000800,
            DateRecentlyChanged = 0x00001000,
            HighlightedFileName = 0x00002000,
            HighlightedPath = 0x00004000,
            HighlightedFullPathAndFileName = 0x00008000,
        }

        /// <summary>
        /// Gets or sets a value indicating whether [match path].
        /// </summary>
        /// <value><c>true</c> if [match path]; otherwise, <c>false</c>.</value>
        public bool MatchPath
        {
            get {
                if (EverythingExists()) {
                    return EverythingApiDllImport.Everything_GetMatchPath();
                }
                return false;
            }
            set {
                if (EverythingExists()) {
                    EverythingApiDllImport.Everything_SetMatchPath(value);
                }
            }
        }

        /// <summary>
        /// Gets or sets a value indicating whether [match case].
        /// </summary>
        /// <value><c>true</c> if [match case]; otherwise, <c>false</c>.</value>
        public bool MatchCase
        {
            get {
                if (EverythingExists()) {
                    return EverythingApiDllImport.Everything_GetMatchCase();
                }
                return false;
            }
            set {
                if (EverythingExists()) {
                    EverythingApiDllImport.Everything_SetMatchCase(value);
                }
            }
        }

        /// <summary>
        /// Gets or sets a value indicating whether [match whole word].
        /// </summary>
        /// <value><c>true</c> if [match whole word]; otherwise, <c>false</c>.</value>
        public bool MatchWholeWord
        {
            get {
                if (EverythingExists()) {
                    return EverythingApiDllImport.Everything_GetMatchWholeWord();
                }
                return false;
            }
            set {
                if (EverythingExists()) {
                    EverythingApiDllImport.Everything_SetMatchWholeWord(value);
                }
            }
        }

        /// <summary>
        /// Gets or sets a value indicating whether [enable regex].
        /// </summary>
        /// <value><c>true</c> if [enable regex]; otherwise, <c>false</c>.</value>
        public bool EnableRegex
        {
            get {
                if (EverythingExists()) {
                    return EverythingApiDllImport.Everything_GetRegex();
                }
                return false;
            }
            set {
                if (EverythingExists()) {
                    EverythingApiDllImport.Everything_SetRegex(value);
                }
            }
        }

        public bool CheckEverything(PluginInitContext context)
        {
            bool ret = false;
            if (EverythingExists()) {
                m_EverythingCheckedAndExisted = true;
                TryFindEverything();
                ret = true;
            }
            else if(m_EverythingCheckedAndExisted) {
                if (!string.IsNullOrEmpty(m_EverythingFullPath)) {
                    Process.Start(m_EverythingFullPath);
                }
                System.Windows.Application.Current.Dispatcher.Invoke(() => {
                    context.API.RestarApp();
                });
            }
            return ret;
        }
        /// <summary>
        /// Searches the specified key word and reset the everything API afterwards
        /// </summary>
        /// <param name="keyWord">The key word.</param>
        /// <param name="token">when cancelled the current search will stop and exit (and would not reset)</param>
        /// <param name="offset">The offset.</param>
        /// <param name="maxCount">The max count.</param>
        /// <returns></returns>
        public List<SearchResult> Search(string keyWord, CancellationToken token, int maxCount)
        {
            var results = new List<SearchResult>();

            if (string.IsNullOrEmpty(keyWord))
                throw new ArgumentNullException(nameof(keyWord));
            if (maxCount < 0)
                throw new ArgumentOutOfRangeException(nameof(maxCount));

            if (token.IsCancellationRequested) { return results; }
            if (keyWord.StartsWith("@")) {
                EverythingApiDllImport.Everything_SetRegex(true);
                keyWord = keyWord.Substring(1);
            }
            else {
                EverythingApiDllImport.Everything_SetRegex(false);
            }

            if (token.IsCancellationRequested) { return results; }
            EverythingApiDllImport.Everything_SetRequestFlags(RequestFlag.HighlightedFileName | RequestFlag.HighlightedFullPathAndFileName);
            if (token.IsCancellationRequested) { return results; }
            EverythingApiDllImport.Everything_SetOffset(0);
            if (token.IsCancellationRequested) { return results; }
            EverythingApiDllImport.Everything_SetMax(maxCount);
            if (token.IsCancellationRequested) { return results; }
            EverythingApiDllImport.Everything_SetSearchW(keyWord);

            if (token.IsCancellationRequested) { return results; }
            if (!EverythingApiDllImport.Everything_QueryW(true)) {
                CheckAndThrowExceptionOnError();
                return results;
            }

            if (token.IsCancellationRequested) { return results; }
            int count = EverythingApiDllImport.Everything_GetNumResults();
            for (int idx = 0; idx < count; ++idx) {
                if (token.IsCancellationRequested) { return results; }
                // https://www.voidtools.com/forum/viewtopic.php?t=8169
                string fileNameHighted = Marshal.PtrToStringUni(EverythingApiDllImport.Everything_GetResultHighlightedFileNameW(idx));
                string fullPathHighted = Marshal.PtrToStringUni(EverythingApiDllImport.Everything_GetResultHighlightedFullPathAndFileNameW(idx));
                if (fileNameHighted == null | fullPathHighted == null) {
                    CheckAndThrowExceptionOnError();
                }
                if (token.IsCancellationRequested) { return results; }
                ConvertHightlightFormat(fileNameHighted, out List<int> fileNameHightlightData, out string fileName);
                if (token.IsCancellationRequested) { return results; }
                ConvertHightlightFormat(fullPathHighted, out List<int> fullPathHightlightData, out string fullPath);

                var result = new SearchResult {
                    FileName = fileName,
                    FileNameHightData = fileNameHightlightData,
                    FullPath = fullPath,
                    FullPathHightData = fullPathHightlightData,
                };

                if (token.IsCancellationRequested) { return results; }
                if (EverythingApiDllImport.Everything_IsFolderResult(idx)) {
                    result.Type = ResultType.Folder;
                }
                else {
                    result.Type = ResultType.File;
                }

                results.Add(result);
            }

            return results;
        }
        private void TryFindEverything()
        {
            if (string.IsNullOrEmpty(m_EverythingFullPath)) {
                const int c_Capacity = 4096;
                const string c_Key = "Everything.exe";
                var dir = Registry.GetValue("HKEY_LOCAL_MACHINE\\SOFTWARE\\voidtools\\Everything", "InstallLocation", string.Empty) as string;
                if (!string.IsNullOrEmpty(dir)) {
                    var fullPath = Path.Combine(dir, c_Key);
                    if (File.Exists(fullPath)) {
                        m_EverythingFullPath = fullPath;
                    }
                }
                if (string.IsNullOrEmpty(m_EverythingFullPath)) {
                    EverythingApiDllImport.Everything_SetMatchPath(false);
                    EverythingApiDllImport.Everything_SetMatchCase(false);
                    EverythingApiDllImport.Everything_SetMatchWholeWord(true);
                    EverythingApiDllImport.Everything_SetRegex(false);
                    EverythingApiDllImport.Everything_SetSort(EverythingApiDllImport.EVERYTHING_SORT_PATH_ASCENDING);

                    EverythingApiDllImport.Everything_SetSearchW(c_Key);
                    EverythingApiDllImport.Everything_SetOffset(0);
                    EverythingApiDllImport.Everything_SetMax(100);
                    EverythingApiDllImport.Everything_SetRequestFlags(RequestFlag.FileName | RequestFlag.Path);
                    if (EverythingApiDllImport.Everything_QueryW(true)) {
                        int num = EverythingApiDllImport.Everything_GetNumResults();
                        for (int i = 0; i < num; ++i) {
                            var sb = new StringBuilder(c_Capacity);
                            string fileName = Marshal.PtrToStringUni(EverythingApiDllImport.Everything_GetResultFileNameW(i));
                            EverythingApiDllImport.Everything_GetResultFullPathNameW(i, sb, c_Capacity);
                            if (string.Compare(fileName, c_Key, true) == 0) {
                                var fullPath = sb.ToString();
                                if (File.Exists(fullPath)) {
                                    m_EverythingFullPath = fullPath;
                                }
                            }
                        }
                    }
                }
            }
        }

        private static void ConvertHightlightFormat(string contentHightlighted, out List<int> hightlightData, out string fn)
        {
            hightlightData = new List<int>();
            StringBuilder content = new StringBuilder();
            bool flag = false;
            char[] contentArray = contentHightlighted.ToCharArray();
            int count = 0;
            for (int i = 0; i < contentArray.Length; i++)
            {
                char current = contentHightlighted[i];
                if (current == '*')
                {
                    flag = !flag;
                    count = count + 1;
                }
                else
                {
                    if (flag)
                    {
                        hightlightData.Add(i - count);
                    }
                    content.Append(current);
                }
            }
            fn = content.ToString();
        }

        private string m_EverythingFullPath = string.Empty;
        private bool m_EverythingCheckedAndExisted = false;

        [DllImport("kernel32.dll")]
        private static extern int LoadLibrary(string name);

        public void Load(string sdkPath)
        {
            LoadLibrary(sdkPath);
        }

        public static bool EverythingExists()
        {
            var hwnd = FindWindowW(EVERYTHING_IPC_WNDCLASS, null);
            return hwnd != IntPtr.Zero;
        }

        [DllImport("user32.dll", CharSet = CharSet.Unicode)]
        private static extern IntPtr FindWindowW(string lpClassName, string lpWindowName);
        private const string EVERYTHING_IPC_WNDCLASS = "EVERYTHING_TASKBAR_NOTIFICATION";

        private static void CheckAndThrowExceptionOnError()
        {
            if (EverythingExists()) {
                switch (EverythingApiDllImport.Everything_GetLastError()) {
                    case StateCode.CreateThreadError:
                        throw new CreateThreadException();
                    case StateCode.CreateWindowError:
                        throw new CreateWindowException();
                    case StateCode.InvalidCallError:
                        throw new InvalidCallException();
                    case StateCode.InvalidIndexError:
                        throw new InvalidIndexException();
                    case StateCode.IPCError:
                        throw new IPCErrorException();
                    case StateCode.MemoryError:
                        throw new MemoryErrorException();
                    case StateCode.RegisterClassExError:
                        throw new RegisterClassExException();
                }
            }
        }
    }
}
