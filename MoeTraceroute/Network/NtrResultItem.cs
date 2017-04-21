using System.Collections.Generic;

namespace MoeTraceroute.Network
{
    class NtrResultItem
    {
        public int        Hop  { set; get; } = 0;
        public string     Host { set; get; } = string.Empty;
        public long       Loss { set; get; } = 0;
        public int        Sent { set; get; } = 0;
        public long       Last { set; get; } = -1;
        public long       Avg  { set; get; } = -1;
        public long       Best { set; get; } = -1;
        public long       Wrst { set; get; } = -1;
        public string     Geo  { set; get; } = string.Empty;
        public string     ASN  { set; get; } = string.Empty;
        public int        LoCn { set; get; } = 0; // Lost Counter
        public List<long> Rtts { set; get; } = new List<long> { };
    }
}
