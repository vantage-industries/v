> **Notatka robocza.** Rozwiązanie, do którego to doprowadziło (dynamic VLAN na
> wlan1 + osobne SSID Guest na wlan0), jest opisane w
> [`DOKUMENTACJA.md`](DOKUMENTACJA.md) (sekcje "Architektura sieci" i "Znane
> ograniczenia i otwarte tematy").

we need to figure out how to get 2 ssids at once working on one wifi chip:
https://community.infineon.com/t5/Wi-Fi-Combo/Does-CYW43455-really-not-support-multiple-SSIDs/td-p/46323
However, the softAP does not support multiple SSID beacons.:
https://community.infineon.com/t5/Wi-Fi-Combo/Looking-for-details-regarding-CYW43455-concurrent-Station-and-AP-operation/td-p/122651

other option, based on psk assign vlan to mac?
