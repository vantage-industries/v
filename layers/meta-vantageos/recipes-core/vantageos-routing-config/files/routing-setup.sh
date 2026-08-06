#!/bin/sh
# VantageOS Routing Setup Script
#
# The actual firewall/NAT ruleset lives declaratively in vantageos.nft
# (installed alongside this script) and is loaded wholesale via `nft -f`.
# This script's job is just to resolve the two runtime-configurable knobs
# (WAN_IFACE, ADMIN_IP) into the tiny include that file pulls in, apply it,
# then handle the non-nft parts of router setup (forwarding sysctls, the
# Guest bandwidth cap).

set -e

echo "Configuring VantageOS router..."

WAN_IFACE="${VANTAGEOS_WAN_IFACE:-eth0}"
# Admin host inside Trusted allowed to reach the panel/SSH. Empty = any
# host on br-vlan10 may reach them (still gated by VLAN membership, just
# not by a specific host) -- represented in nft as "match any source".
ADMIN_IP="${VANTAGEOS_ADMIN_IP:-}"
ADMIN_IP_MATCH="${ADMIN_IP:-0.0.0.0/0}"
# Aggregate download cap for the Guest segment.
GUEST_BW_MBIT="${VANTAGEOS_GUEST_BW_MBIT:-20}"

BR20=wlan0  # Guest -- see vantageos.nft for why this one's a bare iface.

NFT_RULESET=/etc/nftables.d/vantageos.nft
NFT_RUNTIME_VARS=/run/vantageos-nft-runtime.nft

# Enable IP forwarding
sysctl -w net.ipv4.ip_forward=1
sysctl -w net.ipv6.conf.all.forwarding=1

# Resolve this boot's runtime knobs for vantageos.nft's `include`.
cat > "$NFT_RUNTIME_VARS" <<-EOF
	define wan_iface = ${WAN_IFACE}
	define admin_ip = ${ADMIN_IP_MATCH}
EOF

# Apply the full declarative ruleset in one shot. vantageos.nft opens with
# `flush ruleset`, so this is idempotent -- safe on boot, safe to re-run
# manually -- without needing per-rule existence checks.
nft -f "$NFT_RULESET"

echo "-- Guest bandwidth cap --"
# Idempotent: drop then recreate rather than tracking "does it already
# exist" -- safe on a oneshot boot service or a manual re-run alike.
tc qdisc del dev "$BR20" root >/dev/null 2>&1 || true
tc qdisc add dev "$BR20" root handle 1: htb default 10
tc class add dev "$BR20" parent 1: classid 1:10 htb rate 20mbit ceil 20mbit
tc qdisc add dev "$BR20" parent 1:10 fq_codel

echo "Router configuration complete!"
