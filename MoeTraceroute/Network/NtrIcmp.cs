using System;
using System.Diagnostics;
using System.Net.NetworkInformation;
using System.Text;

namespace MoeTraceroute.Network
{
    class NtrIcmp
    {
        public string HostName { set; get; }
        public int Timeout { set; get; } = 1000;
        public int Ttl { set; get; } = 0;

        private PingReply Ping()
        {
            Ping pingSender = new Ping();
            PingOptions options = new PingOptions();
            
            // Setup ICMP options
            options.DontFragment = true;
            if (Ttl > 0)
                options.Ttl = Ttl;

            // Create a buffer of 64 bytes of data to be transmitted.
            string data = "http://www.acgdraw.com/?Traceroute_Test_Data&dat=www.acgdraw.com";
            byte[] buffer = Encoding.ASCII.GetBytes(data);

            PingReply reply = pingSender.Send(HostName, Timeout, buffer, options);
            return reply;
        }

        // Get Original Time-to-Live value
        public long GetOriginTtl()
        {
            Ttl = 0; // Set to automatically Ttl
            PingReply reply = Ping();

            if (reply.Status == IPStatus.Success
                || reply.Status == IPStatus.TtlExpired)
            {
                return reply.Options.Ttl;
            }

            return 0;
        }

        // Get destination hop's IP address
        public string GetRouterIpByHop(int Hop)
        {
            Ttl = Hop;
            PingReply reply = Ping();

            if (reply.Status == IPStatus.Success
                || reply.Status == IPStatus.TtlExpired)
            {
                return reply.Address.ToString();
            }

            return string.Empty;
        }

        // Get destination hop's Rtt
        public long GetDestRttByHop(int Hop)
        {
            Ttl = Hop;
            Stopwatch stopWatch = new Stopwatch();
            stopWatch.Start();
            PingReply reply = Ping();
            stopWatch.Stop();

            if (reply.Status == IPStatus.Success
                || reply.Status == IPStatus.TtlExpired)
            {
                return Convert.ToInt32(stopWatch.Elapsed.TotalMilliseconds);
            }

            return -1;
        }
    }
}
