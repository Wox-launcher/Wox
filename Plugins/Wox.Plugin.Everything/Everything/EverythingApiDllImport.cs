using System;
using System.Runtime.InteropServices;
using System.Text;

namespace Wox.Plugin.Everything.Everything
{
    public sealed class EverythingApiDllImport
    {
        [DllImport(Main.DLL, CharSet = CharSet.Unicode)]
        internal static extern int Everything_SetSearchW(string lpSearchString);

        [DllImport(Main.DLL)]
        internal static extern void Everything_SetMatchPath(bool bEnable);

        [DllImport(Main.DLL)]
        internal static extern void Everything_SetMatchCase(bool bEnable);

        [DllImport(Main.DLL)]
        internal static extern void Everything_SetMatchWholeWord(bool bEnable);

        [DllImport(Main.DLL)]
        internal static extern void Everything_SetRegex(bool bEnable);

        [DllImport(Main.DLL)]
        internal static extern void Everything_SetMax(int dwMax);

        [DllImport(Main.DLL)]
        internal static extern void Everything_SetOffset(int dwOffset);

        [DllImport(Main.DLL)]
        internal static extern bool Everything_GetMatchPath();

        [DllImport(Main.DLL)]
        internal static extern bool Everything_GetMatchCase();

        [DllImport(Main.DLL)]
        internal static extern bool Everything_GetMatchWholeWord();

        [DllImport(Main.DLL)]
        internal static extern bool Everything_GetRegex();

        [DllImport(Main.DLL)]
        internal static extern uint Everything_GetMax();

        [DllImport(Main.DLL)]
        internal static extern uint Everything_GetOffset();

        [DllImport(Main.DLL, CharSet = CharSet.Unicode)]
        internal static extern string Everything_GetSearchW();

        [DllImport(Main.DLL)]
        internal static extern EverythingApi.StateCode Everything_GetLastError();

        [DllImport(Main.DLL, CharSet = CharSet.Unicode)]
        internal static extern bool Everything_QueryW(bool bWait);

        [DllImport(Main.DLL)]
        internal static extern void Everything_SortResultsByPath();

        [DllImport(Main.DLL)]
        internal static extern int Everything_GetNumFileResults();

        [DllImport(Main.DLL)]
        internal static extern int Everything_GetNumFolderResults();

        [DllImport(Main.DLL)]
        internal static extern int Everything_GetNumResults();

        [DllImport(Main.DLL)]
        internal static extern int Everything_GetTotFileResults();

        [DllImport(Main.DLL)]
        internal static extern int Everything_GetTotFolderResults();

        [DllImport(Main.DLL)]
        internal static extern int Everything_GetTotResults();

        [DllImport(Main.DLL)]
        internal static extern bool Everything_IsVolumeResult(int nIndex);

        [DllImport(Main.DLL)]
        internal static extern bool Everything_IsFolderResult(int nIndex);

        [DllImport(Main.DLL)]
        internal static extern bool Everything_IsFileResult(int nIndex);

        [DllImport(Main.DLL, CharSet = CharSet.Unicode)]
        internal static extern void Everything_GetResultFullPathNameW(int nIndex, StringBuilder lpString, int nMaxCount);
        // https://www.voidtools.com/forum/viewtopic.php?t=8169
        [DllImport(Main.DLL, CharSet = CharSet.Unicode)]
        public static extern IntPtr Everything_GetResultFileNameW(int nIndex);
        [DllImport(Main.DLL, CharSet = CharSet.Unicode)]
        public static extern IntPtr Everything_GetResultHighlightedPathW(int nIndex);
        [DllImport(Main.DLL, CharSet = CharSet.Unicode)]
        public static extern IntPtr Everything_GetResultHighlightedFileNameW(int nIndex);
        [DllImport(Main.DLL, CharSet = CharSet.Unicode)]
        public static extern IntPtr Everything_GetResultHighlightedFullPathAndFileNameW(int nIndex);
        [DllImport(Main.DLL)]
        public static extern int Everything_GetMajorVersion();
        [DllImport(Main.DLL)]
        public static extern int Everything_GetMinorVersion();
        [DllImport(Main.DLL)]
        public static extern int Everything_GetRevision();
        [DllImport(Main.DLL)]
        public static extern void Everything_SetRequestFlags(EverythingApi.RequestFlag flag);

        [DllImport(Main.DLL)]
        internal static extern void Everything_Reset();
    }
}