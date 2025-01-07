using MoeTraceroute.Network;
using MoeTraceroute.Utility;
using QQWry;
using System;
using System.Collections.Generic;
using CommandLine;
using CommandLine.Text;
using System.Linq;
using System.Net;
using System.Threading;
using System.Timers;
using MoeTraceroute.Ip2Asn;

namespace MoeTraceroute
{
    class Program
    {
        // Default Options
        public static int Timeout { set; get; } = 5000;
        public static int Interval { set; get; } = 1000;
        public static int MaxHop { set; get; } = 25;
        public static bool EnableASN { set; get; } = false;
        public static bool DomainCheck { set; get; } = true;
        public static bool UseIPIPGeo { set; get; } = false;
        public static bool ipv6 = false;

        private static NtrIcmp Trace = new NtrIcmp();
        private static List<NtrResultItem> NtrResultList = new List<NtrResultItem>();
        private static readonly DateTime NTR_STARTTIME = DateTime.Now;
        private static readonly int NTR_TIMEDOUT = -1;
        private static string NTR_HOSTNAME = "";
        private static long NTR_1HOPCOUNT = 0; // Slow down Hop 1 Speed

        private static QQWryLocator QQWry;
        private static ASNs asns;
        //private static string BgpQueryServer = "http://api.iptoasn.com/v1/as/ip/";
        private static string BgpQueryServer = "https://freeapi.dnslytics.net/v1/ip2asn/";
        private static NtrAsn AsnHelper = new NtrAsn();
        private static IP2API Ip2Api = new IP2API();
        private static IPIPNet IPIPNet = new IPIPNet();

        private static void DisplayHelp(ParserResult<Option> result)
        {
            Console.WriteLine(HelpText.AutoBuild(result, h =>
            {
                return HelpText.DefaultParsingErrorsHandler(result, h);
            }, e => e));
        }
        static void Main(string[] args)
        {
            //var options = new Option();

            /*
             * PARSER OPTIONS
             */
            // Parser options via CommandLinePaser
            //if (CommandLine.Parser.Default.ParseArguments(args, options))
            //{
            //    Timeout = options.Timeout * 1000;
            //    Interval = options.Interval * 1000;
            //    MaxHop = options.MaxHop;
            //    EnableASN = options.EnableASN;
            //    UseIPIPGeo = options.UseIPIPGeo;
            //    DomainCheck = options.DomainCheck;
            //}
            //else
            //{
            //    return; // if invalid option, end application
            //}

            ParserResult<Option> parse = Parser.Default.ParseArguments<Option>(args);

            parse.WithParsed(options =>
            {
                Timeout = options.Timeout * 1000;
                MaxHop = options.MaxHop;
                EnableASN = options.EnableASN;
                UseIPIPGeo = false;  // The API has been deprecated
                DomainCheck = options.DomainCheck;
            }).WithNotParsed(errs =>
           {
               Environment.Exit(0); // if invalid option, end application
           });

            // Timeout or Interval MUST greater than 0
            if (Timeout == 0 || Interval == 0)
            {
                //Console.WriteLine(options.GetUsage());
                DisplayHelp(parse);
                return;
            }



            /*
             * DOMAIN OR IP PROCCESS
             */
            // Parser the first parameter is IP or Domain
            if (args.Count() <= 0) // LESS ONE PARAMETER
            {
                //Console.WriteLine(options.GetUsage());
                DisplayHelp(parse);
                return;
            }
            else if (!(ConsoleHelper.IsValidIPAddress(args[0]) // IS IP ADDRESS
                    || ConsoleHelper.IsValidDomainName(args[0]) // OR DOMAIN
                ))
            {
                Console.WriteLine("ERROR: {0} is unknown IP address or domain.\n", args[0]);
                //Console.WriteLine(options.GetUsage());
                DisplayHelp(parse);
                return;
            }

            // save hostname
            NTR_HOSTNAME = args[0];

            // resolving ip address
            string NTR_DESTIPADDR;
            if (ConsoleHelper.IsValidDomainName(NTR_HOSTNAME))
            {
                try
                {
                    NTR_DESTIPADDR = Dns.GetHostAddresses(NTR_HOSTNAME)[0].ToString();
                }
                catch
                {
                    Console.WriteLine($"ERROR: cannot resolve domain '{NTR_HOSTNAME}' to IP address!\n");
                    return;
                }
            }
            else
            {
                NTR_DESTIPADDR = NTR_HOSTNAME;
            }

            // is ipv6 or ipv4
            ipv6 = ConsoleHelper.IsIPv6(NTR_DESTIPADDR);



            /*
             * INITIZATION
             */
            // to load ip location database when ipv4 only
            if (ipv6 == false)
            {
                try
                {
                    QQWry = new QQWryLocator(AppDomain.CurrentDomain.BaseDirectory + "\\qqwry.dat");
                }
                catch { }
            }

            // To load local asn database
            if (EnableASN)
            {
                Console.WriteLine("The ASN database is being initialized; please wait.");
                asns = new ASNs(AppDomain.CurrentDomain.BaseDirectory + "\\ip2asn-combined.tsv");
            }
            

            // SetUp ASN Paser
            AsnHelper.BgpQueryServer = BgpQueryServer;

            // SetUp Tracer
            var NtrResult = new NtrResultItem[MaxHop];
            Trace.Timeout = Timeout;
            Trace.HostName = NTR_DESTIPADDR;

            // Create a new List used to store results
            for (int i = 0; i < MaxHop; i++)
            {
                NtrResultList.Add(new NtrResultItem());
            }

            // Route Tracer Timer
            System.Timers.Timer TracerTimer = new System.Timers.Timer();
            TracerTimer.Elapsed += new ElapsedEventHandler(Tracer);
            TracerTimer.Interval = Interval;
            TracerTimer.Enabled = true;

            // Display Timer
            System.Timers.Timer DisplayTimer = new System.Timers.Timer();
            DisplayTimer.Elapsed += new ElapsedEventHandler(Display);
            DisplayTimer.Interval = Interval;
            DisplayTimer.Enabled = true;

            //Console.ReadLine();
            Console.ReadKey();
        }

        private static void Tracer(object source, ElapsedEventArgs e)
        {
            for (int i = 0; i < MaxHop; i++)
            {
                // Must store value in here, if not will cause incorrect value
                int Num = i;
                int Hop = Num + 1;

                new Thread(() =>
                {
                    // Slow down First Hop Speed
                    // BECAUSE: some router block fast icmp requestion.
                    if (Hop == 1 && NTR_1HOPCOUNT++ % 2 != 0)
                        return;

                    string NewHost = Trace.GetRouterIpByHop(Hop);
                    NtrResultList[Num].Hop = Hop;

                    // If new Hop Hostname is empty means no route get
                    if (NewHost != string.Empty)
                        NtrResultList[Num].Host = NewHost;

                    // Packets Rtt count
                    long LastPing = Trace.GetDestRttByHop(Hop);
                    long BestPing = NtrResultList[Num].Best;
                    long BestPingCbrt = BestPing;
                    long WrstPing = NtrResultList[Num].Wrst;
                    NtrResultList[Num].Last = LastPing;
                    if (BestPing == NTR_TIMEDOUT) // Calibrate when -1 means no value
                        BestPingCbrt = Timeout;
                    if (LastPing < BestPingCbrt && LastPing != NTR_TIMEDOUT) // Best (Compare with Calibrated value)
                        NtrResultList[Num].Best = LastPing;
                    if (LastPing > WrstPing) // Worst
                        NtrResultList[Num].Wrst = LastPing;

                    // Average Rtt count
                    if (LastPing > NTR_TIMEDOUT)
                        if (NtrResultList[Num].Rtts.Count < 128)
                        {
                            NtrResultList[Num].Rtts.Add(LastPing);
                        }
                        else
                        {
                            NtrResultList[Num].Rtts.RemoveAt(0); // remove first
                            NtrResultList[Num].Rtts.Add(LastPing);
                        }
                    if (NtrResultList[Num].Rtts.Count > 0)
                        NtrResultList[Num].Avg = Convert.ToInt32(NtrResultList[Num].Rtts.Average());

                    // Packets sent and loss count
                    if (LastPing == NTR_TIMEDOUT) // Count Lost Ping
                        NtrResultList[Num].LoCn++;
                    long TotalSent = ++NtrResultList[Num].Sent;
                    float LossCount = NtrResultList[Num].LoCn;
                    NtrResultList[Num].Loss = Convert.ToInt32((LossCount / TotalSent) * 100);

                    // Try to Get Geo Location
                    if (IPAddress.TryParse(NtrResultList[Num].Host, out _))
                    {
                        try
                        {
                            if (ipv6)
                            {
                                if (NtrResultList[Num].Geo == String.Empty)
                                {
                                    NtrResultList[Num].Geo = "-";
                                    NtrResultList[Num].Geo = Ip2Api.Parse(NtrResultList[Num].Host);
                                }
                            }
                            if (UseIPIPGeo)
                            {
                                if (NtrResultList[Num].Geo == String.Empty)
                                {
                                    NtrResultList[Num].Geo = "-";
                                    NtrResultList[Num].Geo = IPIPNet.Parse(NtrResultList[Num].Host);
                                }
                            }
                            else
                            {
                                var Location = QQWry.Query(NtrResultList[Num].Host);
                                NtrResultList[Num].Geo = Location.Country + " " + Location.Local;
                            }
                        }
                        catch
                        { }
                    }

                    // Try to Get BGP AS Number
                    if (EnableASN)
                    {
                        if(UseIPIPGeo)
                        {
                            NtrResultList[Num].ASN = AsnHelper.Parse(NtrResultList[Num].Host);
                        }
                        else
                        {
                            NtrResultList[Num].ASN = asns.lookup_by_ip(NtrResultList[Num].Host);
                        }
                    }
                    else
                        NtrResultList[Num].ASN = "- -";
                }).Start();
            }
        }


        private static void Display(object source, ElapsedEventArgs e)
        {
            Console.Title = $"Ntr {NTR_HOSTNAME}  -  MoeArt OpenSource  http://lab.acgdraw.com";
            Console.Clear();
            int MaxLength = Console.WindowWidth - 1;

            // Print Tool Information
            string ToolName = "NTR - MoeArt's Network Traceroute - Special Edition for Laba Festival";
            string ToolCopyright = "(c)2020 - 2025 MoeArt OpenSource, www.acgdraw.com";
            ConsoleHelper.WriteCenter(ToolName, MaxLength);
            ConsoleHelper.WriteCenter(ToolCopyright, MaxLength);

            string InfoLeft = $"Dest: {NTR_HOSTNAME}";
            string InfoRight = "ST: " + NTR_STARTTIME.ToShortDateString() + " " + NTR_STARTTIME.ToLongTimeString();
            Console.WriteLine("{0}{1}", InfoLeft, InfoRight.PadLeft(MaxLength - InfoLeft.Length));

            // Print Title Bar
            string Formatv4 = "{0,3}  {1,-17} {2,5} {3,5} {4,5} {5,5} {6,5} {7,5}  {8,-7} {9}";
            string Formatv6 = "{0,3}  {1,-40} {2,5} {3,5} {4,5} {5,5} {6,5} {7,5}  {8,-7} {9}";
            string Format = ipv6 ? Formatv6 : Formatv4;
            string Title = String.Format(Format,
                "#",
                "DESTINATION",
                "LOSS%",
                "SENT",
                "LAST",
                "BEST",
                "AVG",
                "WRST",
                "ASN",
                "LOCATION").PadRight(MaxLength);

            // Reserve color for Title bar
            ConsoleColor OriginalForeColor = Console.ForegroundColor;
            ConsoleColor OriginalBackColor = Console.BackgroundColor;
            Console.BackgroundColor = OriginalForeColor;
            Console.ForegroundColor = OriginalBackColor;
            Console.WriteLine(Title.Substring(0, MaxLength));
            Console.ResetColor();

            // Print Items from Result List
            for (int i = 0; i < NtrResultList.Count(); i++)
            {
                NtrResultItem Item = NtrResultList[i];
                string FormattedItem = String.Format(Format,
                    Item.Hop == 0 ? "?" : Item.Hop.ToString(),
                    Item.Host == string.Empty ? "Request timed out" : Item.Host,
                    Item.Loss,
                    Item.Sent,
                    Item.Last == NTR_TIMEDOUT ? "*" : Item.Last.ToString(),
                    Item.Best == NTR_TIMEDOUT ? "*" : Item.Best.ToString(),
                    Item.Avg == NTR_TIMEDOUT ? "*" : Item.Avg.ToString(),
                    Item.Wrst == NTR_TIMEDOUT ? "*" : Item.Wrst.ToString(),
                    Item.ASN,
                    Item.Geo).PadRight(MaxLength);
                Console.WriteLine(ChineseSubStr.GetSubString(FormattedItem, 0, MaxLength));

                // Break at the End, This hop same as Hostname means the End
                if (Item.Host == Trace.HostName)
                {
                    MaxHop = i + 1; // Set Max Hop to the End Hop
                    break;          // Break Displaying
                }

            }

        }

    }
}
