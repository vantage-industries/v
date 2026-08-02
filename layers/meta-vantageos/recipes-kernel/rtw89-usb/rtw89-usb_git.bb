SUMMARY = "Mainline rtw89 (mac80211) driver backport with USB transport, for RTL8852BU adapters (e.g. Alfa AWUS036AX)"
DESCRIPTION = "This kernel's in-tree rtw89 only has PCIe transport (CONFIG_RTW89_8852BU landed \
upstream in Linux 6.17, this kernel is 6.12). morrownr/rtw89 backports the real mac80211 rtw89 \
driver -- including usb.c/rtw8852bu.c -- as an out-of-tree module, so we get the standards-compliant \
mac80211 driver instead of Realtek's vendor SDK blob, which lacked both monitor mode and AP_VLAN. \
rtw89 itself does not explicitly declare NL80211_IFTYPE_AP_VLAN, but mac80211 core adds it \
automatically for any driver that supports AP mode and doesn't set IEEE80211_HW_SW_CRYPTO_CONTROL \
(net/mac80211/main.c, ieee80211_register_hw()) -- rtw89 meets both conditions. Confirmed on-device: \
`iw list` shows AP/VLAN under this driver's supported interface modes."
HOMEPAGE = "https://github.com/morrownr/rtw89"
LICENSE = "GPL-2.0-only | BSD-3-Clause"
LIC_FILES_CHKSUM = "file://Makefile;beginline=1;endline=1;md5=7f2f44c1ef8a9b540a4a52473b3739be"

inherit module

SRC_URI = "git://github.com/morrownr/rtw89.git;protocol=https;branch=main"
SRCREV = "08b8d326937a200a706ec9c501374eec15835b5a"
PV = "1.0+git"

# The driver's Makefile uses KDIR (not KERNEL_SRC/KERNEL_PATH, which
# module.bbclass already passes), evaluated only in the recursive
# "-C $(KDIR) M=$$PWD modules" call, so it needs to be forced here.
EXTRA_OEMAKE += "KDIR=${STAGING_KERNEL_DIR}"

# Upstream has no "modules_install" target (module.bbclass's default), only
# a host-installing "install" target that writes straight to /lib/modules
# and calls depmod on the build host -- not usable in a Yocto sysroot build.
# Install the built .ko files ourselves into the standard OE module path.
do_install() {
	install -d ${D}${nonarch_base_libdir}/modules/${KERNEL_VERSION}/extra
	install -m 0644 ${B}/*.ko ${D}${nonarch_base_libdir}/modules/${KERNEL_VERSION}/extra/
}

# rtw8852bu.c's usb_device_id table includes 0x0bda:0xb832, the Alfa
# AWUS036AX -- confirmed against this repo's source and against the live
# device's own USB descriptors (lsusb -v on the target).
KERNEL_MODULE_AUTOLOAD += "rtw89_8852bu_git"
