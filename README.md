# NTR (MoeArt's Network TraceRoute)
NTR is a useful tool to help network engineer diagnose network problem. NTR can find out all the routers between source host and destination host via ICMP protocol, and NTR can resolve each router's BGP AS number via IPtoASN public service. NTR can resolve each IP's Geo-location via QQWry database.

# Features
* Works find with windows systems
* Based on .NET Framework 4.5
* Do a traceroute via ICMP protocol
* Convert IP to BGP ASN online
* Get IP's Geo Location
* Console Application in Windows

# Quick Start
### Simplest Way
```batch
C:\> ntr www.acgdraw.com
```

### With BGP AS Convert
```batch
C:\> ntr www.acgdraw.com -b
```

### Other Options
```
Ntr (MoeArt's Network Traceroute) 1.0.0.0
Copyright (c) 2017 MoeArt OpenSource Project

  -t, --timeout       (Default: 5) Stop waiting router response in seconds.
  -i, --interval      (Default: 1) Seconds between each traceroute.
  -m, --max-hop       (Default: 30) How many hops try to find. (min:1, max:255)
  -b, --enable-asn    (Default: False) Enable IP to AS number query.
  
  --help              Display this help screen.

```

# License
This project is released under [MIT License](https://github.com/moeart/ntr/blob/master/LICENSE).    
    
IP Geo-Location database is uses [CZ88.NET](http://www.cz88.net) database, which is a free IP Geo-Location database. You can simply download that database from theirs homepage. And the **QQWry Class** is used [QQWry.NET](https://github.com/Alife/QQWry.NET) which written by **@Alife**.    
    
IP to ASN uses the public service provided by [IPtoASN.com](https://iptoasn.com/), which is serviced under [BSD 2-Clause License](https://github.com/jedisct1/iptoasn-webservice/blob/master/LICENSE). It provided public convert service and offline databases. And here is [Github repo](https://github.com/jedisct1/iptoasn-webservice).

### MoeArt Development Team
Github: https://github.com/moeart
Developer Home: http://lab.acgdraw.com
Official Website: http://www.acgdraw.com

