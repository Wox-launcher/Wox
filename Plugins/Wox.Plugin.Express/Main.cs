using Newtonsoft.Json;
using System.Collections.Generic;
using System.IO;
using System.Net.Http;
using System.Text.RegularExpressions;
using System.Windows.Forms;

namespace Wox.Plugin.Express
{
    public class Main : IPlugin, IPluginI18n, IResultUpdated
    {
        private const string __URL = "http://api.open.baidu.com/pae/channel/data/asyncqury?appid=4001";

        //private const string __KEYWORD = "kdcx";

        private string m_keyword = "";

        private const string __TIPS = @"请输入快递缩写,例如顺丰:shunfeng+空格单号";

        private PluginInitContext context { get; set; }

        /// <summary>
        /// 快递信息
        /// </summary>
        private List<ExpressDataInfo> m_listInfo = new List<ExpressDataInfo>();

        public event ResultUpdatedEventHandler ResultsUpdated;

        public List<Result> Query(Query query)
        {
            // 格式 key 快递 单号
            string[] _info = query.RawQuery.Split(' ');

            // 需要查询的信息
            //string[] _info = _str.Split(' ');

            List<Result> _list = new List<Result>();

            // 判断快递名称
            var _str = query.RawQuery.Substring(m_keyword.Length + 1);

            // 格式匹配
            if (_info.Length < 2)
            {
                _list.Add(new Result
                {
                    Title = __TIPS,
                    SubTitle = "",
                    IcoPath = "txt",
                    Score = 20,
                    Action = e =>
                    {
                        return false;
                    }
                });

                return _list;
            }
            else
            {
                // 查询快递id
                string _id = _info[1];

                // 快递公司名
                string _expressName = "未知快递";

                bool _find = false;

                foreach (ExpressDataInfo _i in m_listInfo)
                {
                    if (_id.Equals(_i.Key))
                    {
                        _id = _i.Key;
                        _expressName = _i.Name;

                        _find = true;

                        break;
                    }

                    // 模糊查找
                    if (_i.Key.Contains(_id) ||
                        _i.Name.Contains(_id))
                    {
                        _list.Add(new Result
                        {
                            Title = "查询:" + _i.Key + "_" + _i.Name,
                            SubTitle = "点击选取",
                            IcoPath = "txt",
                            Score = 20,
                            Action = e =>
                            {
                                // 重新拼接字符串
                                this.context.API.ChangeQuery(_info[0] + " " + _i.Key + " ");

                                return false;
                            }
                        });
                    }
                }

                // 根据中文找对应key
                if (!_find)
                {
                    return _list;
                }

                if (_info.Length > 2)
                {
                    string _order = _info[2];

                    // 开始查询
                    if (_order.Length > 0)
                    {
                        _list.Add(new Result
                        {
                            Title = "查询" + _expressName + "_单号:" + _order,
                            SubTitle = "点击开始查询",
                            IcoPath = "txt",
                            Score = 20,
                            Action = e =>
                            {
                                QueryExpress(_id, _order, query);

                                return false;
                            }
                        });
                    }
                }
                else
                {
                    _list.Add(new Result
                    {
                        Title = @"输入需要查询的单号",
                        SubTitle = "",
                        IcoPath = "txt",
                        Score = 20,
                        Action = e =>
                        {
                            return false;
                        }
                    });
                }

                return _list;
            }
        }

        public void Init(PluginInitContext context)
        {
            this.context = context;

            // init Info
            // string _filePath = Path.GetDirectoryName(Assembly.GetEntryAssembly().Location) + "/Plugins/Wox.Plugin.Express/Resources/ExpressInfo.json";
            string _filePath = Path.Combine(context.CurrentPluginMetadata.PluginDirectory, "Resources\\ExpressInfo.json");

            if (File.Exists(_filePath))
            {
                string _txtStr = File.ReadAllText(_filePath);
                object _object = JsonConvert.DeserializeObject<List<ExpressDataInfo>>(_txtStr);
                m_listInfo = _object as List<ExpressDataInfo>;
            }
        }

        public string GetTranslatedPluginTitle()
        {
            return context.API.GetTranslation("wox_plugin_express_plugin_name");
        }

        public string GetTranslatedPluginDescription()
        {
            return context.API.GetTranslation("wox_plugin_express_plugin_description");
        }

        #region 接口

        /// <summary>
        /// 查询快递
        /// </summary>
        /// <param name="_id">快递id</param>
        /// <param name="_order">单号</param>
        /// <returns></returns>
        private List<Result> QueryExpress(string _id, string _order, Query _query)
        {
            List<Result> _list = new List<Result>();

            HttpClientHandler _handler = new HttpClientHandler();

            // 这里为false表示不采用HttpClient的默认Cookie,而是采用httpRequestmessage的Cookie
            _handler.UseCookies = false;

            using (var _client = new HttpClient(_handler))
            {
                string _cookie = GetCookie(_id, _order, _client);

                var request = new HttpRequestMessage(HttpMethod.Get,
                                                   __URL +
                                                   //__PARAM +
                                                   //@"&order=" + _order +
                                                   //@"&id=" + _id);
                                                   @"&com=" + _id +
                                                   @"&nu=" + _order +
                                                   @"&qq-pf-to=pcqq.c2c");

                request.Headers.Add("Cookie", _cookie);

                HttpResponseMessage respones = _client.SendAsync(request).Result;

                var jsonString = respones.Content.ReadAsStringAsync().Result;

                var _expressModel = JsonConvert.DeserializeObject<ExpressModel>(jsonString);

                // 是否是错误
                if (_expressModel.Status == 0)
                {
                    // 第一条状态信息
                    Result _result = new Result
                    {
                        Title = "快递状态:" + _expressModel.Data.Info.GetStatus(),
                        SubTitle = "Copy this text to the clipboard",
                        IcoPath = "txt",
                        Score = 20,
                        Action = e =>
                        {
                            try
                            {
                                return false;
                            }
                            catch (System.Runtime.InteropServices.ExternalException)
                            {
                                return false;
                            }
                        }
                    };

                    _list.Add(_result);

                    // 表单
                    //for (int i = _expressModel.Data.Info.Context.Length - 1; i > 0; --i)
                    for (int i = 0; i < _expressModel.Data.Info.Context.Length; ++i)
                    {
                        ExpressContext _data = _expressModel.Data.Info.Context[i];

                        _result = new Result
                        {
                            Title = _data.Time() + "_" + _data.Desc,
                            SubTitle = "Copy this text to the clipboard",
                            IcoPath = "txt",
                            Score = 20,
                            Action = e =>
                            {
                                //context_.Api.HideAndClear();

                                try
                                {
                                    Clipboard.SetText(_data.Time() + "_" + _data.Desc);

                                    return true;
                                }
                                catch (System.Runtime.InteropServices.ExternalException)
                                {
                                    return false;
                                }
                            }
                        };

                        _list.Add(_result);
                    }
                }
                else
                {
                    Result _result = new Result
                    {
                        Title = "查询出现了错误:" + _expressModel.Message,
                        SubTitle = "Copy this text to the clipboard",
                        IcoPath = "txt",
                        Score = 20,
                        Action = e =>
                        {
                            try
                            {
                                Clipboard.SetText(_expressModel.Message);

                                return true;
                            }
                            catch (System.Runtime.InteropServices.ExternalException)
                            {
                                return false;
                            }
                        }
                    };

                    _list.Add(_result);
                }

                // 可以为空
                ResultsUpdated?.Invoke(this, new ResultUpdatedEventArgs
                {
                    Results = _list,
                    Query = _query
                });

                return _list;
            }
        }

        private static string GetCookie(string _id, string _order, HttpClient _client)
        {
            var _msg = new HttpRequestMessage(HttpMethod.Get,
                                              __URL +
                                              @"&com=" + _id +
                                              @"&nu=" + _order +
                                              @"&qq-pf-to=pcqq.c2c");

            HttpResponseMessage _respones = _client.SendAsync(_msg).Result;

            IEnumerable<string> _cookies = _respones.Headers.GetValues("Set-Cookie");

            string _cookie = "";

            foreach (string _str in _cookies)
            {
                _cookie = GetCookieValue(_str);

                break;
            }

            return _cookie;
        }

        private static string GetCookieValue(string _cookie)
        {
            Regex _regex = new Regex(".*?;");
            Match _value = _regex.Match(_cookie);
            string _cookieValue = _value.Groups[0].Value;

            return _cookieValue.Substring(0, _cookieValue.Length - 1);
        }

        #endregion 接口
    }
}