using System;
using System.Collections.Generic;
using System.Runtime.InteropServices;

namespace Wox.Infrastructure.Hotkey
{
    public class GlobalHotkey
    {
        private readonly IntPtr _hookID;
        private readonly Dictionary<HotkeyModel, Action> _hotkeys;
        private static GlobalHotkey _instance;
        private HotkeyModel _capturingHotkey;

        public bool Capturing { get; set; } = false;
        public static GlobalHotkey Instance => _instance ?? (_instance = new GlobalHotkey());

        private GlobalHotkey()
        {
            _hotkeys = new Dictionary<HotkeyModel, Action>();
            _hookID = InterceptKeys.SetHook(LowLevelKeyboardProc);
        }

        public void SetHotkey(HotkeyModel hotkey, Action action)
        {
            if (!_hotkeys.ContainsKey(hotkey))
            {
                _hotkeys[hotkey] = action;
            }
            else
            {
                throw new ArgumentException("hotkey existed");
            }
        }

        public void RemoveHotkey(HotkeyModel hotkey)
        {
            if (_hotkeys.ContainsKey(hotkey))
            {
                _hotkeys.Remove(hotkey);
            }
        }

        private IntPtr LowLevelKeyboardProc(int nCode, IntPtr wParam, IntPtr lParam)
        {
            const int pressed = 0;  // HC_ACTION
            if (nCode == pressed)
            {
                var m = (WindowsMessage)wParam.ToInt32();
                if (m == WindowsMessage.KEYDOWN || m == WindowsMessage.SYSKEYDOWN)
                {
                    var info = (KBDLLHOOKSTRUCT)Marshal.PtrToStructure(lParam, typeof(KBDLLHOOKSTRUCT));
                    var pressedHotkey = new HotkeyModel(info.vkCode);
                    if (pressedHotkey.Key != Key.None &&
                        pressedHotkey.ModifierKeys.Length != 0)
                    {
                        if (!Capturing)
                        {
                            if (_hotkeys.ContainsKey(pressedHotkey))
                            {
                                var action = _hotkeys[pressedHotkey];
                                action();
                                var intercepted = (IntPtr)1;
                                return intercepted;
                            }
                            else
                            {
                                return InterceptKeys.CallNextHookEx(_hookID, nCode, wParam, lParam);
                            }
                        }
                        else if (HotkeyCaptured != null)
                        {
                            var args = new HotkeyCapturedEventArgs
                            {
                                Hotkey = pressedHotkey,
                                Available = !_hotkeys.ContainsKey(pressedHotkey),
                            };
                            HotkeyCaptured.Invoke(this, args);
                            var intercepted = (IntPtr)1;
                            return intercepted;
                        }
                        else
                        {
                            return InterceptKeys.CallNextHookEx(_hookID, nCode, wParam, lParam);
                        }
                    }
                    else
                    {
                        return InterceptKeys.CallNextHookEx(_hookID, nCode, wParam, lParam);
                    }
                }
                else
                {
                    return InterceptKeys.CallNextHookEx(_hookID, nCode, wParam, lParam);
                }
            }
            else
            {
                return InterceptKeys.CallNextHookEx(_hookID, nCode, wParam, lParam);
            }
        }
        public delegate void HotkeyCapturedEventHandler(object sender, HotkeyCapturedEventArgs e);
        public event HotkeyCapturedEventHandler HotkeyCaptured;

        public class HotkeyCapturedEventArgs : EventArgs
        {
            public HotkeyModel Hotkey { get; set; }
            public bool Available { get; set; }
        }

        ~GlobalHotkey()
        {
            InterceptKeys.UnhookWindowsHookEx(_hookID);
        }
    }
}