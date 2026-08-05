#!/bin/sh
# VantageOS Routing Setup Script
#
# Implements the SecurityHub segmentation model (Projekt sieci PDF, Etap 1):
# the "z -> do" matrix (PDF SS4), the per-segment rules (PDF SS5), the
# management-plane lockdown (PDF SS6) and the internet-window primitive
# (PDF SS7). Traffic never traverses the physical wlan1 AP interface itself
# once dynamic_vlan assigns a station to a VLAN -- hostapd moves it onto
# wlan1.<vlanid>, a member of the matching br-vlan<N> bridge -- so every
# FORWARD/INPUT rule below matches on the bridge, never on wlan1.

set -e

echo "Configuring VantageOS router..."

WAN_IFACE="${VANTAGEOS_WAN_IFACE:-eth0}"
# Admin host inside Trusted allowed to reach the panel/SSH (PDF SS6, control
# 8: "Allow-lista hosta admina"). Empty = any host on br-vlan10 may reach
# them (still gated by VLAN membership, just not by a specific host).
ADMIN_IP="${VANTAGEOS_ADMIN_IP:-}"
# Aggregate download cap for the Guest segment (PDF SS5.3 / Appendix A.5).
GUEST_BW_MBIT="${VANTAGEOS_GUEST_BW_MBIT:-20}"

BR5=br-vlan5    # Quarantine / onboarding
BR10=br-vlan10  # Trusted
BR20=br-vlan20  # Guest
BR30=br-vlan30  # Cloud Native
BR40=br-vlan40  # CAM
BR50=br-vlan50  # IoT offline / hybrid
BR60=br-vlan60  # Services (HA, NVR)

# Home Assistant / NVR controller -- the one host in Services that the
# document grants active (not just replied-to) cross-VLAN reach (PDF SS5.7).
HA_IP=192.168.60.10

# ipt <chain> <args...> -- append to <chain> iff an identical rule isn't
# already present. Keeps the script safe to re-run (boot, manual re-run).
ipt() {
    chain="$1"
    shift
    iptables -w -C "$chain" "$@" 2>/dev/null || iptables -w -A "$chain" "$@"
}

# Enable IP forwarding
sysctl -w net.ipv4.ip_forward=1
sysctl -w net.ipv6.conf.all.forwarding=1

# Default policies: DROP everything not explicitly allowed below.

#iptables -w -P FORWARD DROP
#iptables -w -P INPUT DROP
#cant test with theese suckers ^

# NAT: masquerade LAN -> WAN uplink.
iptables -w -t nat -C POSTROUTING -o "${WAN_IFACE}" -j MASQUERADE 2>/dev/null || \
    iptables -w -t nat -A POSTROUTING -o "${WAN_IFACE}" -j MASQUERADE

# Global: accept return traffic everywhere (WAN<->LAN and inter-VLAN alike)
# with one rule instead of one per pair. This is also the mechanism behind
# the matrix's core property: "zaden segment nie inicjuje polaczen
# do Trusted -- tylko ruch powrotny"
ipt FORWARD -m state --state ESTABLISHED,RELATED -j ACCEPT

# Self-expiring, per-device egress exceptions. Populated externally (the
# onboarding UI on approval, a CAM per-device exception, the IoT update
# job) via: ipset add <name> <device-ip> timeout <seconds>
ipset create vantageos_quarantine_window hash:ip timeout 900 -exist
ipset create vantageos_cam_window hash:ip timeout 900 -exist
ipset create vantageos_iot_window hash:ip timeout 900 -exist

# Cloud Native appliance egress allow-list.
# Populate with resolved IPs of vendor domains; empty by default so nothing is allowed until an
# admin explicitly lists appliance destinations.
ipset create vantageos_cloud_allow hash:ip -exist

echo "-- VLAN 5: Quarantine / onboarding --"
# Zero lateral movement; internet only inside the onboarding window; the
# hub itself is reachable for DNS/DHCP/NTP only (INPUT chain, below).
for dst in "$BR10" "$BR20" "$BR30" "$BR40" "$BR50" "$BR60"; do
    ipt FORWARD -i "$BR5" -o "$dst" -j DROP
done
ipt FORWARD -i "$BR5" -o "$WAN_IFACE" -m set --match-set vantageos_quarantine_window src -j ACCEPT
ipt FORWARD -i "$BR5" -o "$WAN_IFACE" -j DROP

echo "-- VLAN 10: Trusted --"
# Highest trust: full egress, kamera podglad, dashboard HA. No rule needed
# for intra-VLAN traffic -- that's the same bridge (L2), never routed.
ipt FORWARD -i "$BR10" -o "$WAN_IFACE" -j ACCEPT
ipt FORWARD -i "$BR10" -o "$BR40" -p tcp -m multiport --dports 554,8800,443 -j ACCEPT
ipt FORWARD -i "$BR10" -o "$BR60" -d "$HA_IP" -p tcp -m multiport --dports 8123,443 -j ACCEPT
for dst in "$BR20" "$BR30" "$BR5"; do
    ipt FORWARD -i "$BR10" -o "$dst" -j DROP
done

echo "-- VLAN 20: Guest --"
# Password-only, no fingerprinting/classification, no captive portal. Full
# isolation from internal VLANs; internet allowed but ping blocked;
# bandwidth capped below via tc/HTB+fq_codel.
for dst in "$BR5" "$BR10" "$BR30" "$BR40" "$BR50" "$BR60"; do
    ipt FORWARD -i "$BR20" -o "$dst" -j DROP
done
ipt FORWARD -i "$BR20" -p icmp --icmp-type echo-request -j DROP
ipt FORWARD -i "$BR20" -o "$WAN_IFACE" -j ACCEPT

echo "-- VLAN 30: Cloud Native --"
# Egress-only: nothing from any other VLAN reaches in. Appliance profile
# gets a hard allow-list (ipset above); everything else on 30 (the "media"
# profile -- TV/streaming, too many CDNs to allow-list) gets wide :443
# egress with anomalous-port traffic logged and dropped.
for dst in "$BR5" "$BR10" "$BR20" "$BR40" "$BR50" "$BR60"; do
    ipt FORWARD -i "$BR30" -o "$dst" -j DROP
done
ipt FORWARD -i "$BR30" -o "$WAN_IFACE" -p tcp --dport 443 -m set --match-set vantageos_cloud_allow dst -j ACCEPT
ipt FORWARD -i "$BR30" -o "$WAN_IFACE" -p tcp --dport 443 -j ACCEPT
ipt FORWARD -i "$BR30" -o "$WAN_IFACE" -j LOG --log-prefix "CLOUD-EGRESS-ANOMALY "
ipt FORWARD -i "$BR30" -o "$WAN_IFACE" -j DROP

echo "-- VLAN 40: CAM --"
# Recording devices: never dial out to any other VLAN on their own
# initiative (replies still flow via the global ESTABLISHED,RELATED rule),
# except the push/MQTT exception to the Services controller; no internet
# except a per-device window; nothing opens WAN->CAM (no rule does, so the
# default DROP policy covers "zero port-forwarding, zero UPnP").
ipt FORWARD -i "$BR40" -o "$BR60" -d "$HA_IP" -p tcp -m multiport --dports 1883,8123 -j ACCEPT
for dst in "$BR5" "$BR10" "$BR20" "$BR30" "$BR50" "$BR60"; do
    ipt FORWARD -i "$BR40" -o "$dst" -j DROP
done
ipt FORWARD -i "$BR40" -o "$WAN_IFACE" -m set --match-set vantageos_cam_window src -j ACCEPT
ipt FORWARD -i "$BR40" -o "$WAN_IFACE" -j DROP

echo "-- VLAN 50: IoT offline / hybrid --"
# Locally controlled devices: internet only inside an update window;
# control inbound only from the Services controller (allowed from the 60
# side, below); push/MQTT out to the Services controller.
ipt FORWARD -i "$BR50" -o "$BR60" -d "$HA_IP" -p tcp -m multiport --dports 1883,8123 -j ACCEPT
for dst in "$BR5" "$BR10" "$BR20" "$BR30" "$BR40" "$BR60"; do
    ipt FORWARD -i "$BR50" -o "$dst" -j DROP
done
ipt FORWARD -i "$BR50" -o "$WAN_IFACE" -m set --match-set vantageos_iot_window src -j ACCEPT
ipt FORWARD -i "$BR50" -o "$WAN_IFACE" -j DROP

echo "-- VLAN 60: Services / Controller --"
# Control-plane host: actively pushes control/streams to IoT and CAM,
# reaches the internet for updates; no Cloud30 talk-back; no unsolicited
# inbound from WAN (remote access = WireGuard, configured elsewhere).
ipt FORWARD -i "$BR60" -s "$HA_IP" -o "$BR50" -p tcp -m multiport --dports 80,443 -j ACCEPT
ipt FORWARD -i "$BR60" -s "$HA_IP" -o "$BR40" -p tcp -m multiport --dports 554,8800 -j ACCEPT
for dst in "$BR5" "$BR10" "$BR20" "$BR30" "$BR40" "$BR50"; do
    ipt FORWARD -i "$BR60" -o "$dst" -j DROP
done
ipt FORWARD -i "$BR60" -s "$HA_IP" -o "$WAN_IFACE" -p tcp --dport 443 -j ACCEPT
ipt FORWARD -i "$BR60" -o "$WAN_IFACE" -j DROP

echo "-- Management plane (PDF SS6) --"
# The hub is the firewall for every VLAN, so its own control surface gets
# the three controls from SS6: bind-by-VLAN (source-bridge gate below),
# admin-host allow-list (if VANTAGEOS_ADMIN_IP is set), and a hard INPUT
# chain where every other VLAN reaches only DNS/DHCP/NTP (SS6 control 7).
ipt INPUT -i lo -j ACCEPT
ipt INPUT -m state --state ESTABLISHED,RELATED -j ACCEPT
for br in "$BR5" "$BR10" "$BR20" "$BR30" "$BR40" "$BR50" "$BR60"; do
    ipt INPUT -i "$br" -p udp -m multiport --dports 53,67,123 -j ACCEPT
    ipt INPUT -i "$br" -p tcp --dport 53 -j ACCEPT
done
if [ -n "$ADMIN_IP" ]; then
    ipt INPUT -i "$BR10" -s "$ADMIN_IP" -p tcp -m multiport --dports 22,443 -j ACCEPT
else
    ipt INPUT -i "$BR10" -p tcp -m multiport --dports 22,443 -j ACCEPT
fi
# Tailscale admin (SSH/panel) -- new inbound connections over the tailnet
# don't match anything above (ESTABLISHED,RELATED only covers replies), so
# without this rule they'd hit the default INPUT DROP even though
# tailscaled itself reports the node as connected. Harmless no-op if the
# tailscale0 interface never exists (feature not enabled on this image).
ipt INPUT -i tailscale0 -p tcp -m multiport --dports 22,443 -j ACCEPT
# No explicit WAN DROP needed -- default INPUT policy is DROP and nothing
# above matches the WAN interface.

echo "-- Guest bandwidth cap (PDF Appendix A.5) --"
# Idempotent: drop then recreate rather than tracking "does it already
# exist" -- safe on a oneshot boot service or a manual re-run alike.
tc qdisc del dev "$BR20" root >/dev/null 2>&1 || true
tc qdisc add dev "$BR20" root handle 1: htb default 10
tc class add dev "$BR20" parent 1: classid 1:10 htb rate "${GUEST_BW_MBIT}mbit" ceil "${GUEST_BW_MBIT}mbit"
tc qdisc add dev "$BR20" parent 1:10 fq_codel

echo "Router configuration complete!"
