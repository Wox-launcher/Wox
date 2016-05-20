using Newtonsoft.Json;
using System;

namespace Wox.Plugin.Express
{
    public class ExpressModel
    {
        /// <summary>
        /// 错误信息
        /// </summary>
        [JsonProperty("msg")]
        internal string Message { get; set; }

        /// <summary>
        /// 错误代码
        /// </summary>
        [JsonProperty("status")]
        internal int Status { get; set; }

        [JsonProperty("data")]
        public ExpressData Data { get; set; }
    }

    public class ExpressData
    {
        [JsonProperty("info")]
        public ExpressInfo Info { get; set; }

        [JsonProperty("content")]
        public string Content { get; set; }
    }

    public class ExpressInfo
    {
        /// <summary>
        /// 当前状态
        /// "0": "在途，即货物处于运输过程中",
        /// "1": "揽件，货物已由快递公司揽收并且产生了第一条跟踪信息",
        /// "2": "疑难，货物寄送过程出了问题",
        /// "3": "签收，收件人已签收",
        /// "4": "退签，即货物由于用户拒签、超区等原因退回，而且发件人已经签收",
        /// "5": "派件，即快递正在进行同城派件",
        /// "6": "退回，货物正处于退回发件人的途中"
        /// </summary>
        [JsonProperty("state")]
        public int State { get; set; }

        /// <summary>
        /// 获取快递状态
        /// </summary>
        /// <returns></returns>
        internal string GetStatus()
        {
            switch (State)
            {
                case 0:
                    {
                        return "在途";
                    }
                case 1:
                    {
                        return "揽件";
                    }
                case 2:
                    {
                        return "疑难";
                    }
                case 3:
                    {
                        return "签收";
                    }
                case 4:
                    {
                        return "退签";
                    }
                case 5:
                    {
                        return "派件";
                    }
                case 6:
                    {
                        return "退回";
                    }
            }

            return State.ToString();
        }

        /// <summary>
        /// 公司
        /// </summary>
        [JsonProperty("com")]
        public string Com { get; set; }

        /// <summary>
        /// 进度描述
        /// </summary>
        [JsonProperty("context")]
        public ExpressContext[] Context { get; set; }
    }

    public class ExpressContext
    {
        [JsonProperty("time")]
        private float time { get; set; }

        public string Time()
        {
            DateTime _dt = new DateTime(1970, 1, 1, 8, 0, 0);

            return _dt.AddSeconds(time).ToString();
        }

        [JsonProperty("desc")]
        public string Desc { get; set; }
    }

    public class ExpressDataInfo
    {
        [JsonProperty("expresskey")]
        public string Key { get; set; }

        /// <summary>
        /// 快递名
        /// </summary>
        [JsonProperty("expressname")]
        public string Name { get; set; }
    }
}