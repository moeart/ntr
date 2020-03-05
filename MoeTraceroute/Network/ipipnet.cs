using Newtonsoft.Json.Linq;
using System.Globalization;
using System.Net;
using System.Text.RegularExpressions;

namespace MoeTraceroute.Network
{
    class IPIPNet
    {
        private static string SystemLang = CultureInfo.InstalledUICulture.Name.Contains("zh") ? "CN" : "EN";
        public string IpQueryServer { set; get; } = $"https://btapi.ipip.net/host/info?host=&lang={SystemLang}&ip=";
        private string IpGeoList = "www.acgdraw.com@0|lab.acgdraw.com@0"; // Padding Data

        // Parse IP to GEO via IP2API public service
        public string Parse(string Host)
        {
            string GeoRet = string.Empty;

            // If Host not a Ip skip it
            if (IPAddress.TryParse(Host, out _))
            {
                // If host already in the list, parse via list
                foreach (string GeoIp in IpGeoList.Split('|'))
                {
                    string Ip = GeoIp.Split('@')[0];
                    string Geo = GeoIp.Split('@')[1];

                    if (Ip == Host)
                    {
                        return Geo;
                    }
                }

                // If not in list try parse via IPIPNET
                try
                {
                    using (WebClient wc = new WebClient())
                    {
                        wc.Encoding = System.Text.Encoding.UTF8;
                        wc.Headers.Add("User-Agent", "BestTrace/Windows V3.7.3");
                        var json = wc.DownloadString($"{IpQueryServer}{Host}");
                        JToken token = JObject.Parse(json);
                        // replace string
                        GeoRet = (string)token.SelectToken("area");
                        GeoRet = GeoRet.Replace("\t\t", "_ntr_")
                            .Replace("\t", "")
                            .Replace("_ntr_", "  ");
                        GeoRet = Regex.Replace(GeoRet, @"\d*\.\d*", "");
                        // replace LAN string
                        GeoRet = GeoRet.Contains("局域网") ? "本地局域网" : GeoRet;
                        GeoRet = GeoRet.Contains("LAN Address") ? "Local Area Network" : GeoRet;

                        // Store to local List
                        IpGeoList += $"|{Host}@{GeoRet}";
                        wc.Dispose();
                    }
                }
                catch
                { }
            }

            return GeoRet;
        }
    }
}
