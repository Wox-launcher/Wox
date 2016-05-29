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
        private bool _modifierPressed = false;

        public bool Capturing { get; set; } = false;
        public static GlobalHotkey Instance => _instance ?? (_instance = new GlobalHotkey());

        private GlobalHotkey()
        {
            _hotkeys = new Dictionary<HotkeyModel, Action>();
            _hookID = InterceptKeys.SetHook(LowLevelKeyboardProc);
        }

        public void SetHotkey(HotkeyModel hotkey, Action action)
        {
            if (!_hotkeys.ContainsKey(hotkey) && hotkey.Key != Key.None && hotkey.ModifierKeys.Length > 0)
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
            IntPtr intercepted = (IntPtr)1;
            
            if (nCode == pressed)
            {
                var m = (WindowsMessage)wParam.ToInt32();
                var info = (KBDLLHOOKSTRUCT)Marshal.PtrToStructure(lParam, typeof(KBDLLHOOKSTRUCT));
                if (m == WindowsMessage.KEYDOWN || m == WindowsMessage.SYSKEYDOWN)
                {
                    var pressedHotkey = new HotkeyModel(info.vkCode);
                    if (pressedHotkey.Key != Key.None &&
                        pressedHotkey.ModifierKeys.Length != 0)
                    {
                        if (!Capturing)
                        {
                            if (_hotkeys.ContainsKey(pressedHotkey))
                            {
                                _modifierPressed = true;
                                var action = _hotkeys[pressedHotkey];
                                action();
                                return intercepted;
                            }
                            else
                            {
                                return InterceptKeys.CallNextHookEx(_hookID, nCode, wParam, lParam);
                            }
                        }
                        else if (HotkeyCaptured != null)
                        {
                            _modifierPressed = true;
                            var args = new HotkeyCapturedEventArgs
                            {
                                Hotkey = pressedHotkey,
                                Available = !_hotkeys.ContainsKey(pressedHotkey),
                            };
                            HotkeyCaptured.Invoke(this, args);
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
                else if (m == WindowsMessage.KEYUP)
                {
                    // if win+r is pressed, windows star menu will still popup
                    // so we need to discard keyup event
                    if (_modifierPressed)
                    {
                        _modifierPressed = false;
                        if (info.vkCode == Key.LWIN || info.vkCode == Key.RWIN || info.vkCode == Key.WIN)
                        {
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