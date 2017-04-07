using System;
using System.Collections.Generic;
using System.Linq;
using System.Net.NetworkInformation;
using System.Text;
using System.Threading.Tasks;

namespace MoeTraceroute.Network
{
    class Ntr
    {
        public string HostName { set; get; }
        public int Ttl { set; get; } = -1;

        private PingReply Ping()
        {
            Ping pingSender = new Ping();
            PingOptions options = new PingOptions();
            
            // Setup ICMP options
            options.DontFragment = true;
            if (Ttl > -1)
                options.Ttl = Ttl;

            // Create a buffer of 64 bytes of data to be transmitted.
            string data = "http://www.acgdraw.com/?Traceroute_Test_Data&dat=www.acgdraw.com";
            byte[] buffer = Encoding.ASCII.GetBytes(data);
            int timeout = 120;

            PingReply reply = pingSender.Send(HostName, timeout, buffer, options);
            return reply;
            Console.WriteLine(reply.Status.ToString());
            if (reply.Status == IPStatus.Success)
            {
                Console.WriteLine("Address: {0}", reply.Address.ToString());
                Console.WriteLine("RoundTrip time: {0}", reply.RoundtripTime);
                Console.WriteLine("Time to live: {0}", reply.Options.Ttl);
                Console.WriteLine("Don't fragment: {0}", reply.Options.DontFragment);
                Console.WriteLine("Buffer size: {0}", reply.Buffer.Length);
            }
            else if (reply.Status == IPStatus.TtlExpired)
            {
                Console.WriteLine("Address: {0}", reply.Address.ToString());
                Console.WriteLine("RoundTrip time: {0}", reply.RoundtripTime);
                Console.WriteLine("Buffer size: {0}", reply.Buffer.Length);
            }
        }

        public static int GetTtl()
        {

        }
    }
}
