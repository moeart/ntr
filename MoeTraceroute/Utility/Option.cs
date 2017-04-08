using CommandLine;
using CommandLine.Text;

namespace MoeTraceroute.Utility
{
    class Option
    {
        [Option('t', "timeout", DefaultValue = 5,
          HelpText = "Stop waiting router response in seconds.")]
        public int Timeout { get; set; }

        [Option('i', "interval", DefaultValue = 1,
          HelpText = "Seconds between each traceroute.")]
        public int Interval { get; set; }

        [Option('m', "max-hop", DefaultValue = 30,
          HelpText = "How many hops try to find. (min:1, max:255)")]
        public int MaxHop { get; set; }

        [Option('b', "enable-asn", DefaultValue = false,
          HelpText = "Enable IP to AS number query.")]
        public bool EnableASN { get; set; }

        [ParserState]
        public IParserState LastParserState { get; set; }

        [HelpOption]
        public string GetUsage()
        {
            return HelpText.AutoBuild(this,
              (HelpText current) => HelpText.DefaultParsingErrorsHandler(this, current));
        }
    }
}
