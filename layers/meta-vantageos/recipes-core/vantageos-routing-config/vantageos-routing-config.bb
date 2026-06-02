DESCRIPTION = "VantageOS routing configuration"
LICENSE = "MIT"
LIC_FILES_CHKSUM = "file://${COMMON_LICENSE_DIR}/MIT;md5=0835ade698e0bcf8506ecda2f7b4f302"

inherit systemd

FILESEXTRAPATHS:prepend := "${THISDIR}/files:"

# Bump PR to force rebuild
PR = "r6"

SRC_URI = " \
    file://10-vantageos-routing.network \
    file://25-wlan0.network \
    file://dnsmasq.conf \
    file://dnsmasq.service.d/10-vantageos.conf \
    file://90-vantageos-router.conf \
    file://hostapd.conf \
    file://vantageos-hostapd.service \
    file://vantageos-router.service \
    file://routing-setup.sh \
"

do_install() {
    # Install systemd-networkd routing configs
    install -d ${D}${sysconfdir}/systemd/network
    install -m 0644 ${UNPACKDIR}/10-vantageos-routing.network ${D}${sysconfdir}/systemd/network/
    install -m 0644 ${UNPACKDIR}/25-wlan0.network ${D}${sysconfdir}/systemd/network/

    # Install persistent forwarding sysctl config
    install -d ${D}${sysconfdir}/sysctl.d
    install -m 0644 ${UNPACKDIR}/90-vantageos-router.conf ${D}${sysconfdir}/sysctl.d/

    # Install captive portal DNS config
    install -d ${D}${sysconfdir}/dnsmasq.d
    install -m 0644 ${UNPACKDIR}/dnsmasq.conf ${D}${sysconfdir}/dnsmasq.d/vantageos.conf
    
    # Install hostapd config
    install -d ${D}${sysconfdir}/hostapd
    install -m 0644 ${UNPACKDIR}/hostapd.conf ${D}${sysconfdir}/hostapd/vantageos-hostapd.conf
    
    # Install routing setup script
    install -d ${D}${bindir}
    install -m 0755 ${UNPACKDIR}/routing-setup.sh ${D}${bindir}/vantageos-routing-setup
    
    # Install systemd services
    install -d ${D}${systemd_system_unitdir}
    install -d ${D}${systemd_system_unitdir}/dnsmasq.service.d
    install -m 0644 ${UNPACKDIR}/dnsmasq.service.d/10-vantageos.conf ${D}${systemd_system_unitdir}/dnsmasq.service.d/10-vantageos.conf
    install -m 0644 ${UNPACKDIR}/vantageos-hostapd.service ${D}${systemd_system_unitdir}/
    install -m 0644 ${UNPACKDIR}/vantageos-router.service ${D}${systemd_system_unitdir}/
    
    # Create marker file to verify installation
    echo "VantageOS routing config installed" > ${D}${sysconfdir}/vantageos-routing-installed
}

pkg_postinst_ontarget:${PN}() {
    systemctl enable vantageos-hostapd.service || true
    systemctl enable vantageos-router.service || true
}

FILES:${PN} = " \
    ${sysconfdir}/systemd/network/10-vantageos-routing.network \
    ${sysconfdir}/systemd/network/25-wlan0.network \
    ${sysconfdir}/dnsmasq.d/vantageos.conf \
    ${sysconfdir}/sysctl.d/90-vantageos-router.conf \
    ${sysconfdir}/hostapd/vantageos-hostapd.conf \
    ${sysconfdir}/vantageos-routing-installed \
    ${bindir}/vantageos-routing-setup \
    ${systemd_system_unitdir}/dnsmasq.service.d/10-vantageos.conf \
    ${systemd_system_unitdir}/vantageos-hostapd.service \
    ${systemd_system_unitdir}/vantageos-router.service \
"

RDEPENDS:${PN} = " \
    iproute2 \
    hostapd \
    dnsmasq \
    iptables \
"

SYSTEMD_SERVICE:${PN} = "vantageos-hostapd.service vantageos-router.service"
