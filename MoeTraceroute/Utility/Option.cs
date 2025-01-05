using CommandLine;
using CommandLine.Text;

namespace MoeTraceroute.Utility
{
    class Option
    {
        [Option('t', "timeout", Default = 5,
          HelpText = "Stop waiting router response in seconds. (min:1)")]
        public int Timeout { get; set; }

        [Option('i', "interval", Default = 1,
          HelpText = "Seconds between each traceroute. (min:1)")]
        public int Interval { get; set; }

        [Option('m', "max-hop", Default = 25,
          HelpText = "How many hops try to find. (min:1, max:255)")]
        public int MaxHop { get; set; }

        [Option('b', "enable-asn", Default = false,
          HelpText = "Enable IP to BGP AS number query.")]
        public bool EnableASN { get; set; }

        //[Option('o', "online-geoip", Default = false,
        //  HelpText = "Use Online GeoIP database instead of local GeoIP.")]
        //public bool UseIPIPGeo { get; set; }

        [Option('d', "unverify-tld", Default = false,
          HelpText = "Disable Domain Available Verification.")]
        public bool DomainCheck { get; set; }

        //[ParserState]
        //public IParserState LastParserState { get; set; }

        //[HelpOption]
        //public string GetUsage()
        //{
        //    return HelpText.AutoBuild(this,
        //      (HelpText current) => HelpText.DefaultParsingErrorsHandler(this, current));
        //}
    }
}
