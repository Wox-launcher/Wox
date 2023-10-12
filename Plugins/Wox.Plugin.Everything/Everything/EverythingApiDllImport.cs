using System;
using System.Runtime.InteropServices;
using System.Text;
using static Wox.Plugin.Everything.Everything.EverythingApi;

namespace Wox.Plugin.Everything.Everything
{
    public sealed class EverythingApiDllImport
    {
        [DllImport(Main.DLL, CharSet = CharSet.Unicode)]
        public static extern int Everything_SetSearchW(string lpSearchString);

        [DllImport(Main.DLL)]
        public static extern void Everything_SetMatchPath(bool bEnable);

        [DllImport(Main.DLL)]
        public static extern void Everything_SetMatchCase(bool bEnable);

        [DllImport(Main.DLL)]
        public static extern void Everything_SetMatchWholeWord(bool bEnable);

        [DllImport(Main.DLL)]
        public static extern void Everything_SetRegex(bool bEnable);

        [DllImport(Main.DLL)]
        public static extern void Everything_SetMax(int dwMax);

        [DllImport(Main.DLL)]
        public static extern void Everything_SetOffset(int dwOffset);
        [DllImport(Main.DLL)]
        public static extern void Everything_SetReplyWindow(IntPtr hWnd);
        [DllImport(Main.DLL)]
        public static extern void Everything_SetReplyID(int nId);

        [DllImport(Main.DLL)]
        public static extern bool Everything_GetMatchPath();

        [DllImport(Main.DLL)]
        public static extern bool Everything_GetMatchCase();

        [DllImport(Main.DLL)]
        public static extern bool Everything_GetMatchWholeWord();

        [DllImport(Main.DLL)]
        public static extern bool Everything_GetRegex();

        [DllImport(Main.DLL)]
        public static extern uint Everything_GetMax();

        [DllImport(Main.DLL)]
        public static extern uint Everything_GetOffset();

        [DllImport(Main.DLL, CharSet = CharSet.Unicode)]
        public static extern IntPtr Everything_GetSearchW();

        [DllImport(Main.DLL)]
        public static extern EverythingApi.StateCode Everything_GetLastError();
        [DllImport(Main.DLL)]
        public static extern IntPtr Everything_GetReplyWindow();
        [DllImport(Main.DLL)]
        public static extern int Everything_GetReplyID();

        [DllImport(Main.DLL, CharSet = CharSet.Unicode)]
        public static extern bool Everything_QueryW(bool bWait);

        [DllImport(Main.DLL)]
        public static extern bool Everything_IsQueryReply(int message, IntPtr wParam, IntPtr lParam, uint nId);


        [DllImport(Main.DLL)]
        public static extern void Everything_SortResultsByPath();

        [DllImport(Main.DLL)]
        public static extern int Everything_GetNumFileResults();

        [DllImport(Main.DLL)]
        public static extern int Everything_GetNumFolderResults();

        [DllImport(Main.DLL)]
        public static extern int Everything_GetNumResults();

        [DllImport(Main.DLL)]
        public static extern int Everything_GetTotFileResults();

        [DllImport(Main.DLL)]
        public static extern int Everything_GetTotFolderResults();

        [DllImport(Main.DLL)]
        public static extern int Everything_GetTotResults();

        [DllImport(Main.DLL)]
        public static extern bool Everything_IsVolumeResult(int nIndex);

        [DllImport(Main.DLL)]
        public static extern bool Everything_IsFolderResult(int nIndex);

        [DllImport(Main.DLL)]
        public static extern bool Everything_IsFileResult(int nIndex);

        [DllImport(Main.DLL, CharSet = CharSet.Unicode)]
        public static extern void Everything_GetResultFullPathNameW(int nIndex, StringBuilder lpString, int nMaxCount);
        [DllImport(Main.DLL)]
        public static extern void Everything_Reset();
        [DllImport(Main.DLL)]
        public static extern void Everything_CleanUp();
        [DllImport(Main.DLL)]
        public static extern int Everything_GetMajorVersion();
        [DllImport(Main.DLL)]
        public static extern int Everything_GetMinorVersion();
        [DllImport(Main.DLL)]
        public static extern int Everything_GetRevision();
        [DllImport(Main.DLL)]
        public static extern uint Everything_GetBuildNumber();
        [DllImport(Main.DLL)]
        public static extern bool Everything_Exit();
        [DllImport(Main.DLL)]
        public static extern bool Everything_IsDBLoaded();
        [DllImport(Main.DLL)]
        public static extern bool Everything_IsAdmin();
        [DllImport(Main.DLL)]
        public static extern bool Everything_IsAppData();
        [DllImport(Main.DLL)]
        public static extern bool Everything_RebuildDB();
        [DllImport(Main.DLL)]
        public static extern bool Everything_UpdateAllFolderIndexes();
        [DllImport(Main.DLL)]
        public static extern bool Everything_SaveDB();
        [DllImport(Main.DLL)]
        public static extern bool Everything_SaveRunHistory();
        [DllImport(Main.DLL)]
        public static extern bool Everything_DeleteRunHistory();
        [DllImport(Main.DLL)]
        public static extern uint Everything_GetTargetMachine();
        // https://www.voidtools.com/forum/viewtopic.php?t=8169
        [DllImport(Main.DLL, CharSet = CharSet.Unicode)]
        public static extern IntPtr Everything_GetResultFileNameW(int nIndex);

        // Everything 1.4
        [DllImport(Main.DLL)]
        public static extern void Everything_SetSort(uint dwSortType);
        [DllImport(Main.DLL)]
        public static extern uint Everything_GetSort();
        [DllImport(Main.DLL)]
        public static extern uint Everything_GetResultListSort();
        [DllImport(Main.DLL)]
        public static extern void Everything_SetRequestFlags(RequestFlag flag);
        [DllImport(Main.DLL)]
        public static extern RequestFlag Everything_GetRequestFlags();
        [DllImport(Main.DLL)]
        public static extern uint Everything_GetResultListRequestFlags();
        [DllImport(Main.DLL, CharSet = CharSet.Unicode)]
        public static extern IntPtr Everything_GetResultExtensionW(uint nIndex);
        [DllImport(Main.DLL)]
        public static extern bool Everything_GetResultSize(uint nIndex, out long lpFileSize);
        [DllImport(Main.DLL)]
        public static extern bool Everything_GetResultDateCreated(uint nIndex, out long lpFileTime);
        [DllImport(Main.DLL)]
        public static extern bool Everything_GetResultDateModified(uint nIndex, out long lpFileTime);
        [DllImport(Main.DLL)]
        public static extern bool Everything_GetResultDateAccessed(uint nIndex, out long lpFileTime);
        [DllImport(Main.DLL)]
        public static extern uint Everything_GetResultAttributes(uint nIndex);
        [DllImport(Main.DLL, CharSet = CharSet.Unicode)]
        public static extern IntPtr Everything_GetResultFileListFileNameW(uint nIndex);
        [DllImport(Main.DLL)]
        public static extern uint Everything_GetResultRunCount(uint nIndex);
        [DllImport(Main.DLL)]
        public static extern bool Everything_GetResultDateRun(uint nIndex, out long lpFileTime);
        [DllImport(Main.DLL)]
        public static extern bool Everything_GetResultDateRecentlyChanged(uint nIndex, out long lpFileTime);
        [DllImport(Main.DLL, CharSet = CharSet.Unicode)]
        public static extern IntPtr Everything_GetResultHighlightedFileNameW(int nIndex);
        [DllImport(Main.DLL, CharSet = CharSet.Unicode)]
        public static extern IntPtr Everything_GetResultHighlightedPathW(int nIndex);
        [DllImport(Main.DLL, CharSet = CharSet.Unicode)]
        public static extern IntPtr Everything_GetResultHighlightedFullPathAndFileNameW(int nIndex);
        [DllImport(Main.DLL, CharSet = CharSet.Unicode)]
        public static extern uint Everything_GetRunCountFromFileNameW(string lpFileName);
        [DllImport(Main.DLL, CharSet = CharSet.Unicode)]
        public static extern bool Everything_SetRunCountFromFileNameW(string lpFileName, uint dwRunCount);
        [DllImport(Main.DLL, CharSet = CharSet.Unicode)]
        public static extern uint Everything_IncRunCountFromFileNameW(string lpFileName);
    }
}