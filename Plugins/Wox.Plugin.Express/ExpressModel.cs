using Newtonsoft.Json;
using System;

namespace Wox.Plugin.Express
{
    //public class ExpressModel
    //{
    //    /// <summary>
    //    /// 快递名称
    //    /// </summary>
    //    [JsonProperty("name")]
    //    public string Name { get; set; }

    //    /// <summary>
    //    /// 单号
    //    /// </summary>
    //    [JsonProperty("order")]
    //    internal string Order { get; set; }

    //    /// <summary>
    //    /// 错误信息
    //    /// </summary>
    //    [JsonProperty("message")]
    //    internal string Message { get; set; }

    //    /// <summary>
    //    /// 0000	接口调用正常,无任何错误
    //    /// 0001	传输参数格式有误
    //    /// 0002	用户编号(uid)无效
    //    /// 0003	用户被禁用
    //    /// 0004	key无效
    //    /// 0005	快递代号(id)无效
    //    /// 0006	访问次数达到最大额度
    //    /// 0007	查询服务器返回错误
    //    /// </summary>
    //    [JsonProperty("errcode")]
    //    internal string Errcode { get; set; }

    //    /// <summary>
    //    /// -1	待查询、在批量查询中才会出现的状态,指提交后还没有进行任何更新的单号
    //    /// 0	查询异常
    //    /// 1	暂无记录、单号没有任何跟踪记录
    //    /// 2	在途中
    //    /// 3	派送中
    //    /// 4	已签收
    //    /// 5	拒收、用户拒签
    //    /// 6	疑难件、以为某些原因无法进行派送
    //    /// 7	无效单
    //    /// 8	超时单
    //    /// 9	签收失败
    //    /// </summary>
    //    [JsonProperty("status")]
    //    internal int Status { get; set; }

    //    internal string getStatus()
    //    {
    //        switch (Status)
    //        {
    //            case -1:
    //                {
    //                    return "待查询";
    //                }
    //            case 0:
    //                {
    //                    return "查询异常";
    //                }
    //            case 1:
    //                {
    //                    return "暂无记录";
    //                }
    //            case 2:
    //                {
    //                    return "在途中";
    //                }
    //            case 3:
    //                {
    //                    return "派送中";
    //                }
    //            case 4:
    //                {
    //                    return "已签收";
    //                }
    //            case 5:
    //                {
    //                    return "用户拒签";
    //                }
    //            case 6:
    //                {
    //                    return "疑难件";
    //                }
    //            case 7:
    //                {
    //                    return "无效单";
    //                }
    //            case 8:
    //                {
    //                    return "超时单";
    //                }
    //            case 9:
    //                {
    //                    return "签收失败";
    //                }
    //        }

    //        return Status.ToString();
    //    }

    //    [JsonProperty("data")]
    //    public ExpressData[] Data { get; set; }
    //}

    //public class ExpressData
    //{
    //    [JsonProperty("time")]
    //    public string Time { get; set; }

    //    [JsonProperty("content")]
    //    public string Content { get; set; }
    //}

    ////{"id":"yuantong","name":"圆通快递","order":"881754461664541778","message":"",
    //// "errcode":"0000","status":4,"data":[{"time":"2016-05-09 20:16:31",
    //// "content":"河北省石家庄市高新开发区公司(点击查询电话) 已揽收"},
    ////{"time":"2016-05-09 22:38:39","content":"河北省石家庄市高新开发区公司 已打包"},
    ////{"time":"2016-05-09 23:31:07","content":"河北省石家庄市高新开发区公司 已发出,下一站 石家庄转运中心"},
    ////{"time":"2016-05-10 01:53:23","content":"石家庄转运中心 已收入"},
    ////{"time":"2016-05-10 02:43:33","content":"石家庄转运中心 已发出,下一站 成都转运中心"},
    ////{"time":"2016-05-11 07:07:28","content":"成都转运中心 已收入"},
    ////{"time":"2016-05-11 08:22:51","content":"成都转运中心 已发出,下一站 四川省成都市双流县天府中和"},
    ////{"time":"2016-05-11 12:51:58","content":"四川省成都市双流县天府中和公司 已收入"},
    ////{"time":"2016-05-11 14:14:42","content":"四川省成都市双流县天府中和公司(点击查询电话)王** 派件中 派件员电话18280067758"},
    ////{"time":"2016-05-11 14:15:26","content":"客户 签收人: 代办点友谊超市 已签收感谢使用圆通速递，期待再次为您服务"}]}

    //public class ExpressDataInfo
    //{
    //    [JsonProperty("expresskey")]
    //    public string Key { get; set; }

    //    /// <summary>
    //    /// 快递名
    //    /// </summary>
    //    [JsonProperty("expressname")]
    //    public string Name { get; set; }
    //}

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