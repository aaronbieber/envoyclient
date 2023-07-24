# Envoyclient

This is a client for communicating with the Enphase Envoy solar inverter
management unit, written in Go. It's intended to be used to gather solar metrics
for use in a local tracking system. Perhaps you'd like to load the metrics into
Graphite using my [carbonclient](https://github.com/aaronbieber/carbonclient).

## Configuration

This library is designed for newer Envoy units that require a connection token
from `entrez.enphaseenergy.com`. If you have an older unit (or older firmware)
and you do not need a token to access https://envoy.local/, this client will not
work for you.

In order to receive a token, envoyclient needs to authenticate to
enphaseenergy.com with your main homeowner Enphase email and password, the same
ones that you use to log into https://enlighten.enphaseenergy.com. It may make
sense for you to store those values in a local configuration file or similar.

To get connected, create a new client. If you have the Enlighten app installed
on your smartphone, you can find the Envoy's serial number by navigating to
"Menu" (at the bottom), "System", "Devices", and "Gateway." The number you want
follows the heading "SN:".

Finally you need the IP address of your Envoy. In most cases you can ping
`envoy.local` and it will respond. Alternately, you can comb through the clients
connected to your wifi access point, if using wifi.

```go
envoy, err := envoyclient.NewClient(
  envoyclient.Config{
    Email:         "your Enphase account email",
    Password:      "your Enphase account password",
    EnvoySerialNo: "the serial number of your Envoy unit",
    EnvoyIP:       "the IP of your Envoy unit"})
```

You can then retrieve data with a call to `GetProductionData()`.

```go
data, err := envoy.GetProductionData()
if err != nil {
  panic(err)
}
```

Data is returned as:

```go
type ProductionData struct {
  ProductionWattsNow  float64
  ConsumptionWattsNow float64
}
```

Future versions of the client may return more of the available fields. If you
have a specific need for other fields returned in `production.json`, please open
an issue, or, better yet, submit a PR!

## License

```text
        DO WHAT THE FUCK YOU WANT TO PUBLIC LICENSE 
                    Version 2, December 2004 

 Copyright (C) 2004 Sam Hocevar <sam@hocevar.net> 

 Everyone is permitted to copy and distribute verbatim or modified 
 copies of this license document, and changing it is allowed as long 
 as the name is changed. 

            DO WHAT THE FUCK YOU WANT TO PUBLIC LICENSE 
   TERMS AND CONDITIONS FOR COPYING, DISTRIBUTION AND MODIFICATION 

  0. You just DO WHAT THE FUCK YOU WANT TO.
```
