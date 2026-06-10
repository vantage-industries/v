#!/bin/sh
# VantageOS Routing Setup Script

set -eu

echo "Configuring VantageOS routing..."
echo "Version: 1.0.2"

if [ -r /etc/default/vantageos-router ]; then
    . /etc/default/vantageos-router
fi

LAN_IFACE="${LAN_IFACE:-wlan0}"
WAN_IFACE="${WAN_IFACE:-}"
LAN_ADDR="${LAN_ADDR:-192.168.50.1}"
LAN_PREFIX="${LAN_PREFIX:-24}"
LAN_SUBNET="${LAN_SUBNET:-192.168.50.0/24}"
DHCP_START="${DHCP_START:-192.168.50.100}"
DHCP_END="${DHCP_END:-192.168.50.200}"
DHCP_NETMASK="${DHCP_NETMASK:-255.255.255.0}"
WIFI_SSID="${WIFI_SSID:-VantageOS}"
WIFI_PASSPHRASE="${WIFI_PASSPHRASE:-vantageos-router}"
WIFI_CHANNEL="${WIFI_CHANNEL:-6}"
DNS_UPSTREAM_1="${DNS_UPSTREAM_1:-1.1.1.1}"
DNS_UPSTREAM_2="${DNS_UPSTREAM_2:-8.8.8.8}"

detect_wan_iface() {
    ip route show default | awk '{
        for (i = 1; i <= NF; i++) {
            if ($i == "dev") {
                print $(i + 1)
                exit
            }
        }
    }'
}

ensure_sysctl_conf() {
    key="$1"
    value="$2"

    if grep -q "^${key}=" /etc/sysctl.conf 2>/dev/null; then
        sed -i "s|^${key}=.*|${key}=${value}|" /etc/sysctl.conf
    else
        echo "${key}=${value}" >> /etc/sysctl.conf
    fi
}

ensure_filter_rule() {
    if ! iptables -C "$@" 2>/dev/null; then
        iptables -A "$@"
    fi
}

ensure_nat_rule() {
    if ! iptables -t nat -C "$@" 2>/dev/null; then
        iptables -t nat -A "$@"
    fi
}

write_runtime_configs() {
    mkdir -p /run/vantageos

    cat > /run/vantageos/hostapd.conf <<EOF
interface=${LAN_IFACE}
driver=nl80211

ssid=${WIFI_SSID}
hw_mode=g
channel=${WIFI_CHANNEL}
ieee80211n=1
wmm_enabled=1

auth_algs=1
wpa=2
wpa_key_mgmt=WPA-PSK
rsn_pairwise=CCMP
wpa_passphrase=${WIFI_PASSPHRASE}
EOF

    cat > /run/vantageos/dnsmasq.conf <<EOF
interface=${LAN_IFACE}
bind-interfaces
listen-address=${LAN_ADDR}

dhcp-authoritative
dhcp-range=${DHCP_START},${DHCP_END},${DHCP_NETMASK},12h
dhcp-option=option:router,${LAN_ADDR}
dhcp-option=option:dns-server,${LAN_ADDR}

domain-needed
bogus-priv
cache-size=1000
server=${DNS_UPSTREAM_1}
server=${DNS_UPSTREAM_2}
EOF
}

# Enable IP forwarding
sysctl -w net.ipv4.ip_forward=1
sysctl -w net.ipv6.conf.all.forwarding=1

# Make it permanent
ensure_sysctl_conf net.ipv4.ip_forward 1
ensure_sysctl_conf net.ipv6.conf.all.forwarding 1

if [ -z "$WAN_IFACE" ]; then
    WAN_IFACE="$(detect_wan_iface || true)"
fi

WAN_IFACE="${WAN_IFACE:-eth0}"

if ! ip link show "$LAN_IFACE" >/dev/null 2>&1; then
    echo "ERROR: AP interface '$LAN_IFACE' does not exist. Set LAN_IFACE in /etc/default/vantageos-router."
    exit 1
fi

ip link set "$LAN_IFACE" up
ip addr flush dev "$LAN_IFACE"
ip addr add "${LAN_ADDR}/${LAN_PREFIX}" dev "$LAN_IFACE"
write_runtime_configs

ensure_nat_rule POSTROUTING -s "$LAN_SUBNET" -o "$WAN_IFACE" -j MASQUERADE
ensure_filter_rule FORWARD -i "$LAN_IFACE" -o "$WAN_IFACE" -j ACCEPT
ensure_filter_rule FORWARD -i "$WAN_IFACE" -o "$LAN_IFACE" -m conntrack --ctstate RELATED,ESTABLISHED -j ACCEPT

echo "Routing configuration complete!"
echo "LAN interface: $LAN_IFACE ${LAN_ADDR}/${LAN_PREFIX}"
echo "WAN interface: $WAN_IFACE"
echo "Current routes:"
ip route
