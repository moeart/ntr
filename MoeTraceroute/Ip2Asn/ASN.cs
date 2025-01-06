using System;
using System.Collections.Generic;
using System.Linq;
using System.Text;
using System.Threading.Tasks;

namespace MoeTraceroute.Ip2Asn
{
    class ASN : IComparable<ASN>
    {
        public Ip first_ip;
        public Ip last_ip;
        public string number;
        public string country;
        public string descryption;

        public ASN(Ip first_ip, Ip last_ip, string number, string country, string descryption)
        {
            this.first_ip = first_ip;
            this.last_ip = last_ip;
            this.number = number;
            this.country = country;
            this.descryption = descryption;
        }

        public static ASN From_fingle_ip(Ip ip)
        {
            //Ip temp = Ip.Parse(ip);
            return new ASN(ip, ip, null, null, null);
        }

        public int CompareTo(ASN other)
        {
            return first_ip.CompareTo(other.first_ip);
        }

        public static bool operator <(ASN left, ASN right)
        {
            return left.CompareTo(right) < 0;
        }

        public static bool operator >(ASN left, ASN right)
        {
            return left.CompareTo(right) > 0;
        }

        public static bool operator <=(ASN left, ASN right)
        {
            return left.CompareTo(right) <= 0;
        }

        public static bool operator >=(ASN left, ASN right)
        {
            return left.CompareTo(right) >= 0;
        }
    }
}
