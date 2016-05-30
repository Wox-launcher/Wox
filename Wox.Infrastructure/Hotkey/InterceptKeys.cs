using System;
using System.Diagnostics;
using System.Runtime.InteropServices;

namespace Wox.Infrastructure.Hotkey
{
    internal static class InterceptKeys
    {
        public delegate IntPtr LowLevelKeyboardProc(int nCode, IntPtr wParam, IntPtr lParam);
        private static LowLevelKeyboardProc _proc;

        private const int WH_KEYBOARD_LL = 13;
        private const int ALL_THREAD = 0;

        public static IntPtr SetHook(LowLevelKeyboardProc proc)
        {
            using (Process current = Process.GetCurrentProcess())
            using (ProcessModule module = current.MainModule)
            {
                _proc = proc;
                var handle = GetModuleHandle(module.ModuleName);
                return SetWindowsHookEx(WH_KEYBOARD_LL, _proc, handle, ALL_THREAD);
            }
        }

        [DllImport("user32.dll", CharSet = CharSet.Auto, SetLastError = true)]
        public static extern IntPtr SetWindowsHookEx(int idHook, LowLevelKeyboardProc lpfn, IntPtr hMod, uint dwThreadId);

        [DllImport("user32.dll", CharSet = CharSet.Auto, SetLastError = true)]
        [return: MarshalAs(UnmanagedType.Bool)]
        public static extern bool UnhookWindowsHookEx(IntPtr hhk);

        [DllImport("user32.dll", CharSet = CharSet.Auto, SetLastError = true)]
        public static extern IntPtr CallNextHookEx(IntPtr hhk, int nCode, IntPtr wParam, IntPtr lParam);

        [DllImport("kernel32.dll", CharSet = CharSet.Auto, SetLastError = true)]
        public static extern IntPtr GetModuleHandle(string lpModuleName);

        [DllImport("user32.dll", CharSet = CharSet.Auto, ExactSpelling = true, CallingConvention = CallingConvention.Winapi)]
        public static extern short GetKeyState(Key keyCode);
    }
}