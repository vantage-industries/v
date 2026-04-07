#!/bin/sh
# VantageOS Routing Setup Script

set -e

echo "Configuring VantageOS routing..."
echo "Version: 1.0.1"

# Enable IP forwarding
sysctl -w net.ipv4.ip_forward=1
sysctl -w net.ipv6.conf.all.forwarding=1

# Make it permanent
if ! grep -q "net.ipv4.ip_forward=1" /etc/sysctl.conf; then
    echo "net.ipv4.ip_forward=1" >> /etc/sysctl.conf
fi

if ! grep -q "net.ipv6.conf.all.forwarding=1" /etc/sysctl.conf; then
    echo "net.ipv6.conf.all.forwarding=1" >> /etc/sysctl.conf
fi

# Add custom routes here
# Example: ip route add 10.0.0.0/8 via 192.168.1.1

echo "Routing configuration complete!"
echo "Current routes:"
ip route
