## gorgonzola
Gorgonzola is a DNS client written in Go that can run basically anywhere. 

##### What
It is intended as an extremely lightweight alternative to Pihole and other DNS level ad-blocking tools.

##### Why
Pihole is great, but it tries to do too many things and it has too many moving parts. It has failed me numerous times. I want to have a simple binary that I can run accompanied by a simple configuration file. This is where Gorgonzola comes in.  

Features as of now:  
* uses an configurable upstream DNS to resolve DNS queries
* can use a simple to domain list to block certain DNS queries
* no dependencies

In the makings: 
* whitelist capabilities
* import blacklist, whitelist and the blocklists from Pihole
* support for importing various filter lists through a web UI or CLI: AdGuard, Adblock Plus, uBlock Origin etc.
* automatically recover if crashed 
* rate limiting and DoS/DDoS protection
* a nice and simple web UI to access stats, clients, queries etc.

##### Warning, the current state is `alpha` and not recommended for production.