DESCRIPTION = "VantageOS routing configuration"
LICENSE = "MIT"
LIC_FILES_CHKSUM = "file://${COMMON_LICENSE_DIR}/MIT;md5=0835ade698e0bcf8506ecda2f7b4f302"

FILESEXTRAPATHS:prepend := "${THISDIR}/files:"

# Bump PR to force rebuild
PR = "r2"

SRC_URI = " \
    file://10-vantageos-wan.network \
    file://20-vantageos-ap.network \
    file://routing-setup.sh \
    file://vantageos-router.default \
    file://vantageos-routing-setup.service \
    file://vantageos-hostapd.service \
    file://vantageos-dnsmasq.service \
"

inherit systemd

SYSTEMD_SERVICE:${PN} = " \
    vantageos-routing-setup.service \
    vantageos-hostapd.service \
    vantageos-dnsmasq.service \
"
SYSTEMD_AUTO_ENABLE:${PN} = "enable"

do_install() {
    # Install systemd-networkd interface config
    install -d ${D}${sysconfdir}/systemd/network
    install -m 0644 ${UNPACKDIR}/10-vantageos-wan.network ${D}${sysconfdir}/systemd/network/
    install -m 0644 ${UNPACKDIR}/20-vantageos-ap.network ${D}${sysconfdir}/systemd/network/

    # Install routing setup script
    install -d ${D}${bindir}
    install -m 0755 ${UNPACKDIR}/routing-setup.sh ${D}${bindir}/vantageos-routing-setup

    # Install AP service configuration
    install -d ${D}${sysconfdir}/default
    install -m 0644 ${UNPACKDIR}/vantageos-router.default ${D}${sysconfdir}/default/vantageos-router

    install -d ${D}${systemd_system_unitdir}
    install -m 0644 ${UNPACKDIR}/vantageos-routing-setup.service ${D}${systemd_system_unitdir}/
    install -m 0644 ${UNPACKDIR}/vantageos-hostapd.service ${D}${systemd_system_unitdir}/
    install -m 0644 ${UNPACKDIR}/vantageos-dnsmasq.service ${D}${systemd_system_unitdir}/

    # Create marker file to verify installation
    echo "VantageOS routing config installed" > ${D}${sysconfdir}/vantageos-routing-installed
}

pkg_postinst_ontarget:${PN}() {
    # Enable IP forwarding on first boot
    grep -q "^net.ipv4.ip_forward=1" /etc/sysctl.conf || echo "net.ipv4.ip_forward=1" >> /etc/sysctl.conf
    grep -q "^net.ipv6.conf.all.forwarding=1" /etc/sysctl.conf || echo "net.ipv6.conf.all.forwarding=1" >> /etc/sysctl.conf
    sysctl -p || true
}

FILES:${PN} = " \
    ${sysconfdir}/systemd/network/10-vantageos-wan.network \
    ${sysconfdir}/systemd/network/20-vantageos-ap.network \
    ${sysconfdir}/default/vantageos-router \
    ${systemd_system_unitdir}/vantageos-routing-setup.service \
    ${systemd_system_unitdir}/vantageos-hostapd.service \
    ${systemd_system_unitdir}/vantageos-dnsmasq.service \
    ${sysconfdir}/vantageos-routing-installed \
    ${bindir}/vantageos-routing-setup \
"

RDEPENDS:${PN} = " \
    dnsmasq \
    hostapd \
    iptables \
    iproute2 \
    systemd-networkd \
"
