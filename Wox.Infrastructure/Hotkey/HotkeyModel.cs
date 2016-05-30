using System;
using System.Collections.Generic;
using System.Linq;
using Wox.Plugin;

namespace Wox.Infrastructure.Hotkey
{
    public class HotkeyModel : BaseModel
    {
        private const char Seperater = '+';

        private static readonly Key[] Win = { Key.LWIN, Key.RWIN, Key.WIN };
        private static readonly Key[] Alt = { Key.LALT, Key.RALT, Key.ALT };
        private static readonly Key[] Ctrl = { Key.LCTRL, Key.RCTRL, Key.CTRL };
        private static readonly Key[] Shift = { Key.LSHIFT, Key.RSHIFT, Key.SHIFT };
        private static readonly Key[] AllModifierKeys = Win.Concat(Alt).Concat(Ctrl).Concat(Shift).ToArray();

        public Key Key { get; } = Key.None;
        public Key[] ModifierKeys { get; } = new Key[0];

        public HotkeyModel() { }

        //todo use parse instead of constructor, so class can express empty
        /// <exception cref="ArgumentException"></exception>
        public HotkeyModel(string hotkey)
        {
            var keys = hotkey.Split(Seperater)
                             .Select(k => k.Trim().ToUpper())
                             .Select(k => (Key)Enum.Parse(typeof(Key), k))
                             .ToArray();

            ModifierKeys = keys.Where(k => AllModifierKeys.Contains(k)).ToArray();
            ModifierKeys = LeftRightRemoved(ModifierKeys);

            Key = keys.FirstOrDefault(k => !AllModifierKeys.Contains(k));
            if (!ContainsNonModifyKey(Key))
            {
                Key = ModifierKeys.Last();
            }
        }

        public HotkeyModel(Key pressedKey)
        {
            if (!AllModifierKeys.Contains(pressedKey))
            {
                Key = pressedKey;
                ModifierKeys = AllModifierKeys.Where(ModifierKeyPressed).ToArray();
                ModifierKeys = LeftRightRemoved(ModifierKeys);
            }
            else
            {
                Key = Key.None;
            }
        }


        private Key[] LeftRightRemoved(Key[] keys)
        {
            var removed = new List<Key>();
            if (keys.Any(Ctrl.Contains))
            {
                removed.Add(Key.CTRL);
            }
            if (keys.Any(Shift.Contains))
            {
                removed.Add(Key.SHIFT);
            }
            if (keys.Any(Win.Contains))
            {
                removed.Add(Key.WIN);
            }
            if (keys.Any(Alt.Contains))
            {
                removed.Add(Key.ALT);
            }
            return removed.ToArray();
        }

        private bool ModifierKeyPressed(Key key)
        {
            const int keyPressed = 0x8000;
            var pressed = Convert.ToBoolean(InterceptKeys.GetKeyState(key) & keyPressed);
            return pressed;
        }

        private bool ContainsNonModifyKey(Key key)
        {
            var contains = key != 0;
            return contains;
        }

        public override string ToString()
        {
            var hotkey = string.Join(Seperater.ToString(), ModifierKeys);
            if (hotkey.Length != 0 && Key != Key.None)
            {
                hotkey += $"+{Key}";
            }
            else
            {
                hotkey = Key.None.ToString();
            }
            return hotkey;
        }

        public override int GetHashCode()
        {
            var hashcode = Key.GetHashCode();
            foreach (var key in ModifierKeys)
            {
                hashcode = hashcode ^ key.GetHashCode();
            }
            return hashcode;
        }

        public override bool Equals(object obj)
        {
            var hotkey = obj as HotkeyModel;
            if (hotkey != null)
            {
                if (Key.Equals(hotkey.Key) &&
                    ModifierKeys.Length == hotkey.ModifierKeys.Length)
                {
                    for (var i = 0; i < ModifierKeys.Length; i++)
                    {
                        if (ModifierKeys[i] != hotkey.ModifierKeys[i])
                        {
                            return false;
                        }
                    }
                    return true;
                }
                else
                {
                    return false;
                }
            }
            else
            {
                return false;
            }
        }
    }
}