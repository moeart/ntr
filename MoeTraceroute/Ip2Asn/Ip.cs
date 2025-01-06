using System;
using System.Collections.Generic;
using System.Linq;
using System.Text;
using System.Threading.Tasks;

namespace MoeTraceroute.Ip2Asn
{
    class Ip : System.Net.IPAddress, IComparable<Ip>
    {
        public Ip(long newAddress) : base(newAddress)
        {
        }

        public Ip(byte[] address) : base(address)
        {
        }

        public Ip(byte[] address, long scopeid) : base(address, scopeid)
        {
        }

        public static new Ip Parse(string ipString)
        {
            System.Net.IPAddress parent = System.Net.IPAddress.Parse(ipString);
            return new Ip(parent.GetAddressBytes());
        }
        public int CompareTo(Ip other)
        {
            byte[] current_ip = MapToIPv6().GetAddressBytes();
            byte[] other_ip = other.MapToIPv6().GetAddressBytes();

            for (int i = 0; i < 16; i++)
            {
                if (current_ip[i] != other_ip[i])
                {
                    return current_ip[i].CompareTo(other_ip[i]);
                }
            }
            return 0;
        }
    }
}
