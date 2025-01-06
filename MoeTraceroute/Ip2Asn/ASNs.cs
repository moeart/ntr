using System;
using System.Collections.Generic;
using System.Linq;
using System.Text;
using System.Threading.Tasks;
using System.IO;
using System.Collections.Concurrent;

namespace MoeTraceroute.Ip2Asn
{
    class ASNs
    {
        private SortedSet<ASN> asns = new SortedSet<ASN>();
        private ConcurrentDictionary<string, string> cache = new ConcurrentDictionary<string, string>();

        //Currently, gz packages are not directly supported; this will be updated in future versions.
        public ASNs(string tsvPath)
        {
            using (StreamReader reader = new StreamReader(tsvPath))
            {
                string line;
                while ((line = reader.ReadLine()) != null)
                {
                    string[] columns = line.Split('\t');
                    if (columns.Length < 5)
                        throw new FormatException("Can not parse " + tsvPath);

                    Ip first_ip = Ip.Parse(columns[0]);
                    Ip last_ip = Ip.Parse(columns[1]);

                    ASN asn = new ASN(first_ip, last_ip, columns[2], columns[3], columns[4]);
                    asns.Add(asn);
                }
            }
        }
        public string lookup_by_ip(string ip)
        {
            try
            {
                Ip find_ip = Ip.Parse(ip);
                if (cache.ContainsKey(ip))
                {
                    return cache[ip];
                }
                ASN found = lookup_by_ip(find_ip);
                if (found == null)
                    return null;
                cache.TryAdd(ip, found.number);
                return found.number;
            }
            catch
            {
                return null;
            }
        }
        private ASN lookup_by_ip(Ip ip)
        {
            //need add cache
            ASN fasn = ASN.From_fingle_ip(ip);
            ASN found = asns.Where(x => x <= fasn).DefaultIfEmpty().Max();
            return found;
        }
    }
}
