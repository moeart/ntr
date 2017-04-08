using System;
using System.Net;
using System.Text.RegularExpressions;

namespace MoeTraceroute.Utility
{
    class ConsoleHelper
    {
        public static void WriteCenter(string str, int max)
        {
            int len = str.Length;
            int padding = (max - len) / 2;

            if (len < max)
            {
                for (int i = 0; i < padding; i++)
                    Console.Write(' ');
            }

            Console.WriteLine(str);
        }

        public static bool IsValidDomainName(string name)
        {
            if (Regex.IsMatch(name, @" # Rev:2013-03-26
            # Match DNS host domain having one or more subdomains.
            # Top level domain subset taken from IANA.ORG. See:
            # http://data.iana.org/TLD/tlds-alpha-by-domain.txt
            ^                  # Anchor to start of string.
            (?!.{256})         # Whole domain must be 255 or less.
            (?:                # Group for one or more sub-domains.
              [a-z0-9]         # Either subdomain length from 2-63.
              [a-z0-9-]{0,61}  # Middle part may have dashes.
              [a-z0-9]         # Starts and ends with alphanum.
              \.               # Dot separates subdomains.
            | [a-z0-9]         # or subdomain length == 1 char.
              \.               # Dot separates subdomains.
            )+                 # One or more sub-domains.
            (?:                # Top level domain alternatives.
              [a-z]{2}         # Either any 2 char country code,
            | AERO|ARPA|ASIA|BIZ|CAT|COM|COOP|EDU|  # or TLD 
              GOV|INFO|INT|JOBS|MIL|MOBI|MUSEUM|    # from list.
              NAME|NET|ORG|POST|PRO|TEL|TRAVEL|XXX  # IANA.ORG
            )                  # End group of TLD alternatives.
            $                  # Anchor to end of string.",
            RegexOptions.IgnoreCase | RegexOptions.IgnorePatternWhitespace))
            {
                // Valid named DNS host (domain).
                return true;
            }
            else
            {
                // NOT a valid named DNS host.
                return false;
            }
        }

        public static bool IsValidIPAddress(string ip)
        {
            IPAddress address;
            if (IPAddress.TryParse(ip, out address))
            {
                switch (address.AddressFamily)
                {
                    case System.Net.Sockets.AddressFamily.InterNetwork:
                        // we have IPv4
                        int First = Convert.ToInt32(ip.Split('.')[0]);
                        if (First == 0 || First >= 224) // Reserved IP address
                            return false;
                        break;
                    case System.Net.Sockets.AddressFamily.InterNetworkV6:
                        // we have IPv6
                        if (ip == "::0")
                            return false;
                        break;
                }
                return true;
            }
            return false;
        }
    }
}
