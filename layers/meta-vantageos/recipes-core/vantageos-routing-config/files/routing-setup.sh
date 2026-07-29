#!/bin/sh
# VantageOS Routing Setup Script

set -e

echo "Configuring VantageOS router..."

WAN_IFACE="${VANTAGEOS_WAN_IFACE:-eth0}"
AP_IFACE="${VANTAGEOS_AP_IFACE:-wlan0}"

# Enable IP forwarding
sysctl -w net.ipv4.ip_forward=1
sysctl -w net.ipv6.conf.all.forwarding=1

# Default FORWARD policy: DROP (isolate subnets)
iptables -w -P FORWARD DROP

# NAT: masquerade WiFi AP -> WAN uplink.
iptables -w -t nat -C POSTROUTING -o "${WAN_IFACE}" -j MASQUERADE 2>/dev/null || \
    iptables -w -t nat -A POSTROUTING -o "${WAN_IFACE}" -j MASQUERADE

iptables -w -C FORWARD -i "${WAN_IFACE}" -o "${AP_IFACE}" -m state --state RELATED,ESTABLISHED -j ACCEPT 2>/dev/null || \
    iptables -w -A FORWARD -i "${WAN_IFACE}" -o "${AP_IFACE}" -m state --state RELATED,ESTABLISHED -j ACCEPT

iptables -w -C FORWARD -i "${AP_IFACE}" -o "${WAN_IFACE}" -j ACCEPT 2>/dev/null || \
    iptables -w -A FORWARD -i "${AP_IFACE}" -o "${WAN_IFACE}" -j ACCEPT

echo "Router configuration complete!"
