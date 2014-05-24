using System;

namespace Wox.Core
{
    public static class ChineseToPinYin
    {
        [Obsolete]
        public static string ToPinYin(string txt)
        {
            return txt.Unidecode();
        }
    }
}
