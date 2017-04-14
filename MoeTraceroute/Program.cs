using MoeTraceroute.Network;
using MoeTraceroute.Utility;
using QQWry;
using System;
using System.Collections.Generic;
using System.IO;
using System.Linq;
using System.Net;
using System.Threading;
using System.Timers;

namespace MoeTraceroute
{
    class Program
    {
        // Default Options
        public static int Timeout { set; get; } = 5000;
        public static int Interval { set; get; } = 1000;
        public static int MaxHop { set; get; } = 30;
        public static bool EnableASN { set; get; } = false;
        public static bool DomainCheck { set; get; } = true;

        private static NtrIcmp Trace = new NtrIcmp();
        private static List<NtrResultItem> NtrResultList = new List<NtrResultItem>();
        private static readonly int NTR_TIMEDOUT = -1;

        private static QQWryLocator QQWry;
        private static string BgpQueryServer = "http://api.iptoasn.com/v1/as/ip/";
        private static NtrAsn AsnHelper = new NtrAsn();
        
        static void Main(string[] args)
        {
            var options = new Option();

            // Parser options via CommandLinePaser
            if (CommandLine.Parser.Default.ParseArguments(args, options))
            {
                Timeout = options.Timeout * 1000;
                Interval = options.Interval * 1000;
                MaxHop = options.MaxHop;
                EnableASN = options.EnableASN;
                DomainCheck = options.DomainCheck;
            }
            else
            {
                return; // if invalid option, end application
            }

            // Parser the first parameter is IP or Domain
            if (args.Count() <= 0) // LESS ONE PARAMETER
            {
                Console.WriteLine(options.GetUsage());
                return;
            }
            else if (!(ConsoleHelper.IsValidIPAddress(args[0]) // IS IP ADDRESS
                    || ConsoleHelper.IsValidDomainName(args[0]) // OR DOMAIN
                ))
            {
                Console.WriteLine("ERROR: {0} is unknown IP address or domain.\n", args[0]);
                Console.WriteLine(options.GetUsage());
                return;
            }

            try // to load ip location database
            {
                QQWry = new QQWryLocator(AppDomain.CurrentDomain.BaseDirectory + "\\qqwry.dat");
            }
            catch { }

            // SetUp ASN Paser
            AsnHelper.BgpQueryServer = BgpQueryServer;

            // SetUp Tracer
            var NtrResult = new NtrResultItem[MaxHop];
            Trace.Timeout = Timeout;
            if (ConsoleHelper.IsValidDomainName(args[0]))
                try {
                    Trace.HostName = Dns.GetHostAddresses(args[0])[0].ToString();
                }
                catch
                {
                    Console.WriteLine($"ERROR: cannot resolve domain '{args[0]}' to IP address!\n");
                    return;
                }
            else
                Trace.HostName = args[0];

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
            DisplayTimer.Interval = 1000;
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
                    NtrResultList[Num].Avg = (BestPing + LastPing) / 2;

                    // Packets sent and loss count
                    if (LastPing == NTR_TIMEDOUT) // Count Lost Ping
                        NtrResultList[Num].LoCn++;
                    long TotalSent = ++NtrResultList[Num].Sent;
                    float LossCount = NtrResultList[Num].LoCn;
                    NtrResultList[Num].Loss = Convert.ToInt32((LossCount / TotalSent) * 100);

                    // Try to Get Geo Location
                    if (IPAddress.TryParse(NtrResultList[Num].Host, out _) )
                        try
                        {
                            var Location = QQWry.Query(NtrResultList[Num].Host);
                            NtrResultList[Num].Geo = Location.Country + " " + Location.Local;
                        }
                        catch
                        { }

                    // Try to Get BGP AS Number
                    if (EnableASN)
                        NtrResultList[Num].ASN = AsnHelper.Parse(NtrResultList[Num].Host);
                    else
                        NtrResultList[Num].ASN = "- -";
                }).Start();
            }
        }


        private static void Display(object source, ElapsedEventArgs e)
        {
            Console.Title = $"Ntr {Trace.HostName}  -  MoeArt OpenSource  http://lab.acgdraw.com";
            Console.Clear();
            int MaxLength = Console.WindowWidth - 1;

            // Print Tool Information
            string ToolName = "NTR - MoeArt's Netowrk Traceroute";
            string ToolCopyright = "(c)2017 MoeArt OpenSource, www.acgdraw.com";
            ConsoleHelper.WriteCenter(ToolName, MaxLength);
            ConsoleHelper.WriteCenter(ToolCopyright, MaxLength);
            Console.WriteLine();

            // Print Title Bar
            string Format = "{0,3}  {1,-17} {2,5} {3,5} {4,5} {5,5} {6,5} {7,5}  {8,-7} {9}";
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
            for (int i=0; i<NtrResultList.Count(); i++)
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
                Console.WriteLine(ChineseSubStr.GetSubString(FormattedItem,0,MaxLength));

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
