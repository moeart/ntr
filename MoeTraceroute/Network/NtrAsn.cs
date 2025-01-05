﻿using Newtonsoft.Json.Linq;
using System.Net;

namespace MoeTraceroute.Network
{
    class NtrAsn
    {
        //https://ipinfo.io/widget/demo/
        public string BgpQueryServer { set; get; } = "http://api.iptoasn.com/v1/as/ip/";
        private string BgpAsnList = "www.acgdraw.com,0|lab.acgdraw.com,0"; // Padding Data

        // Parse IP to ASN via IPTOASN public service
        public string Parse(string Host)
        {
            string AsnRet = string.Empty;

            // If Host not a Ip skip it
            if (IPAddress.TryParse(Host, out _))
            {
                // If host already in the list, parse via list
                foreach (string AsnIp in BgpAsnList.Split('|'))
                {
                    string Ip = AsnIp.Split(',')[0];
                    string Asn = AsnIp.Split(',')[1];

                    if (Ip == Host)
                    {
                        return Asn;
                    }
                }

                // If not in list try parse via IPTOASN
                try
                {
                    ServicePointManager.SecurityProtocol = SecurityProtocolType.Ssl3
                    //| SecurityProtocolType.SystemDefault //注意：不要加入此行
                    | SecurityProtocolType.Tls
                    | SecurityProtocolType.Tls11
                    | SecurityProtocolType.Tls12;
                    using (WebClient wc = new WebClient())
                    {
                        
                        var json = wc.DownloadString($"{BgpQueryServer}{Host}");
                        JToken token = JObject.Parse(json);
                        //JToken asn = token.SelectToken("asn");
                        AsnRet = (string)token.SelectToken("asn");

                        // Store to local List
                        BgpAsnList += $"|{Host},{AsnRet}";
                    }
                }
                catch (System.Exception e)
                {
                    System.Diagnostics.Debug.WriteLine(e.Message);
                }
            }

            return AsnRet;
        }
    }
}
