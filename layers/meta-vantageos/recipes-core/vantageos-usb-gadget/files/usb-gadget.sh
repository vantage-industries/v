#!/bin/bash
set -e

CONFIGFS="/sys/kernel/config/usb_gadget"
GADGET_NAME="vantageos"

cleanup_gadget() {
    if [ -d "$CONFIGFS/$GADGET_NAME" ]; then
        if [ -s "$CONFIGFS/$GADGET_NAME/UDC" ]; then
            echo "" > "$CONFIGFS/$GADGET_NAME/UDC" 2>/dev/null || true
            sleep 1
        fi
        for config in "$CONFIGFS/$GADGET_NAME/configs/"*; do
            [ -d "$config" ] || continue
            for link in "$config/"*; do
                [ -L "$link" ] && rm -f "$link" 2>/dev/null || true
            done
            rm -rf "$config/strings/0x409" 2>/dev/null || true
            rmdir "$config" 2>/dev/null || true
        done
        for func in "$CONFIGFS/$GADGET_NAME/functions/"*; do
            [ -d "$func" ] && rmdir "$func" 2>/dev/null || true
        done
        rm -rf "$CONFIGFS/$GADGET_NAME/strings/0x409" 2>/dev/null || true
        rmdir "$CONFIGFS/$GADGET_NAME" 2>/dev/null || true
    fi
}

if [ ! -d "$CONFIGFS" ]; then
    modprobe libcomposite || {
        echo "Failed to load libcomposite module"
        exit 1
    }
    sleep 1
fi

cleanup_gadget

mkdir -p "$CONFIGFS/$GADGET_NAME"
cd "$CONFIGFS/$GADGET_NAME"

echo 0x1d6b > idVendor
echo 0x0104 > idProduct
echo 0x0100 > bcdDevice
echo 0x0200 > bcdUSB
echo 2 > bDeviceClass

mkdir -p strings/0x409
SERIAL=$(awk '/Serial/ {print $NF}' /proc/cpuinfo | tail -c 16)
echo "${SERIAL:-0000000000000000}" > strings/0x409/serialnumber
echo "VantageOS" > strings/0x409/manufacturer
echo "VantageOS USB Device" > strings/0x409/product

# Config 1: ECM (Linux/macOS)
mkdir -p configs/c.1/strings/0x409
echo "ECM" > configs/c.1/strings/0x409/configuration
echo 250 > configs/c.1/MaxPower
echo 0x80 > configs/c.1/bmAttributes

mkdir -p functions/ecm.usb0
echo "00:dc:c8:f7:75:15" > functions/ecm.usb0/host_addr
echo "00:dd:dc:eb:6d:a1" > functions/ecm.usb0/dev_addr
ln -s functions/ecm.usb0 configs/c.1/

# Config 2: RNDIS (Windows)
mkdir -p configs/c.2
echo 0x80 > configs/c.2/bmAttributes
echo 250 > configs/c.2/MaxPower
mkdir -p configs/c.2/strings/0x409
echo "RNDIS" > configs/c.2/strings/0x409/configuration

echo "1" > os_desc/use
echo "0xcd" > os_desc/b_vendor_code
echo "MSFT100" > os_desc/qw_sign

mkdir -p functions/rndis.usb0
echo "00:dc:c8:f7:75:16" > functions/rndis.usb0/dev_addr
echo "00:dd:dc:eb:6d:a2" > functions/rndis.usb0/host_addr
echo "RNDIS" > functions/rndis.usb0/os_desc/interface.rndis/compatible_id
echo "5162001" > functions/rndis.usb0/os_desc/interface.rndis/sub_compatible_id

ln -s functions/rndis.usb0 configs/c.2
ln -s configs/c.2 os_desc

# Bind to UDC
UDC=$(ls /sys/class/udc/ | head -n 1)
if [ -z "$UDC" ]; then
    echo "No UDC found"
    exit 1
fi

echo "$UDC" > UDC

# Give networkd a moment to see the new interfaces
sleep 2
