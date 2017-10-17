using Newtonsoft.Json.Linq;
using System.Net;

namespace MoeTraceroute.Network
{
    class IP2API
    {
        public string IpQueryServer { set; get; } = "http://ip-api.com/json/";
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

                // If not in list try parse via IP2API
                try
                {
                    using (WebClient wc = new WebClient())
                    {
                        var json = wc.DownloadString($"{IpQueryServer}{Host}");
                        JToken token = JObject.Parse(json);
                        GeoRet = (string)token.SelectToken("country") + " " + (string)token.SelectToken("city");

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
