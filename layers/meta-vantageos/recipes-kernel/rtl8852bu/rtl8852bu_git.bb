SUMMARY = "Out-of-tree driver for Realtek RTL8852BU USB Wi-Fi 6 adapters (e.g. Alfa AWUS036AX)"
DESCRIPTION = "The in-tree rtw89 driver in this kernel only has PCIe transport (no usb.c, \
no CONFIG_RTW89_USB) -- USB support for rtw89 landed upstream after this kernel's version. \
morrownr's rtl8852bu carries Realtek's vendor driver as an out-of-tree kbuild module so \
RTL8852BU-based USB adapters (0bda:b832) work on this kernel without waiting for a kernel bump."
HOMEPAGE = "https://github.com/morrownr/rtl8852bu-20250826"
LICENSE = "GPL-2.0-only"
LIC_FILES_CHKSUM = "file://LICENSE;md5=b1918d7d89f091725a3188ff95f7c72b"

inherit module

SRC_URI = "git://github.com/morrownr/rtl8852bu-20250826.git;protocol=https;branch=main"
SRCREV = "38fc5a310f775d40074f4f02dfab988d5a8530e4"
PV = "1.19.21-86+git"

# The driver's own Makefile expects KSRC (not KERNEL_SRC/KERNEL_PATH, which
# module.bbclass already passes) to point at the kernel build tree, and uses
# it unconditionally (":=") to probe the kernel version -- without this it
# falls back to /lib/modules/$(uname -r)/build on the build host, which is
# wrong for a cross build. Command-line make variables override the
# Makefile's ":=" assignment, so this is enough to fix it.
EXTRA_OEMAKE += "KSRC=${STAGING_KERNEL_DIR} KVER=${KERNEL_VERSION}"

# CONFIG_RTL8852B=y + CONFIG_USB_HCI=y are the Makefile defaults, which is
# exactly the AWUS036AX (RTL8852BU) case -- module name resolves to 8852bu.
KERNEL_MODULE_AUTOLOAD += "8852bu"
