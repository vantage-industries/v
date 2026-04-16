DESCRIPTION = "VantageOS USB Gadget configuration for Raspberry Pi 5"
LICENSE = "MIT"
LIC_FILES_CHKSUM = "file://${COMMON_LICENSE_DIR}/MIT;md5=0835ade698e0bcf8506ecda2f7b4f302"

inherit systemd

FILESEXTRAPATHS:prepend := "${THISDIR}/files:"

SRC_URI = " \
    file://usb-gadget.sh \
    file://usb-gadget.service \
    file://10-br0.netdev \
    file://20-br0.network \
    file://25-usb0.network \
    file://25-usb1.network \
"

RDEPENDS:${PN} = " \
    bash \
    kernel-module-libcomposite \
    systemd-networkd \
"

SYSTEMD_SERVICE:${PN} = "usb-gadget.service"

FILES:${PN} = " \
    ${bindir}/usb-gadget.sh \
    ${systemd_system_unitdir}/usb-gadget.service \
    ${sysconfdir}/systemd/network/10-br0.netdev \
    ${sysconfdir}/systemd/network/20-br0.network \
    ${sysconfdir}/systemd/network/25-usb0.network \
    ${sysconfdir}/systemd/network/25-usb1.network \
"

do_install() {
    install -d ${D}${bindir}
    install -m 0755 ${UNPACKDIR}/usb-gadget.sh ${D}${bindir}/usb-gadget.sh

    install -d ${D}${systemd_system_unitdir}
    install -m 0644 ${UNPACKDIR}/usb-gadget.service ${D}${systemd_system_unitdir}/usb-gadget.service

    install -d ${D}${sysconfdir}/systemd/network
    install -m 0644 ${UNPACKDIR}/10-br0.netdev ${D}${sysconfdir}/systemd/network/
    install -m 0644 ${UNPACKDIR}/20-br0.network ${D}${sysconfdir}/systemd/network/
    install -m 0644 ${UNPACKDIR}/25-usb0.network ${D}${sysconfdir}/systemd/network/
    install -m 0644 ${UNPACKDIR}/25-usb1.network ${D}${sysconfdir}/systemd/network/
}

pkg_postinst_ontarget:${PN}() {
    systemctl enable systemd-networkd || true
    systemctl enable usb-gadget.service || true
}
