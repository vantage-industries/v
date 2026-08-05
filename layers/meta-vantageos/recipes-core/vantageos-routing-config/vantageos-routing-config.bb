DESCRIPTION = "VantageOS routing configuration"
LICENSE = "MIT"
LIC_FILES_CHKSUM = "file://${COMMON_LICENSE_DIR}/MIT;md5=0835ade698e0bcf8506ecda2f7b4f302"

inherit systemd

FILESEXTRAPATHS:prepend := "${THISDIR}/files:"

# Bump PR to force rebuild
PR = "r15"

SRC_URI = " \
    file://10-vantageos-routing.network \
    file://10-alfa-wlan.link \
    file://25-wlan1.network \
    file://26-wlan0.network \
    file://30-vlan5-quarantine.network \
    file://31-vlan10-trusted.network \
    file://33-vlan30-cloud.network \
    file://34-vlan40-cam.network \
    file://35-vlan50-iot.network \
    file://36-vlan60-services.network \
    file://dnsmasq.conf \
    file://dnsmasq.service.d/10-vantageos.conf \
    file://90-vantageos-router.conf \
    file://hostapd.conf \
    file://hostapd-guest.conf \
    file://wpa_psk \
    file://vantageos-hostapd.service \
    file://vantageos-router.service \
    file://routing-setup.sh \
"

do_install() {
    # Install systemd-networkd routing configs
    install -d ${D}${sysconfdir}/systemd/network
    install -m 0644 ${UNPACKDIR}/10-vantageos-routing.network ${D}${sysconfdir}/systemd/network/
    install -m 0644 ${UNPACKDIR}/10-alfa-wlan.link ${D}${sysconfdir}/systemd/network/
    install -m 0644 ${UNPACKDIR}/25-wlan1.network ${D}${sysconfdir}/systemd/network/
    install -m 0644 ${UNPACKDIR}/26-wlan0.network ${D}${sysconfdir}/systemd/network/
    install -m 0644 ${UNPACKDIR}/30-vlan5-quarantine.network ${D}${sysconfdir}/systemd/network/
    install -m 0644 ${UNPACKDIR}/31-vlan10-trusted.network ${D}${sysconfdir}/systemd/network/
    install -m 0644 ${UNPACKDIR}/33-vlan30-cloud.network ${D}${sysconfdir}/systemd/network/
    install -m 0644 ${UNPACKDIR}/34-vlan40-cam.network ${D}${sysconfdir}/systemd/network/
    install -m 0644 ${UNPACKDIR}/35-vlan50-iot.network ${D}${sysconfdir}/systemd/network/
    install -m 0644 ${UNPACKDIR}/36-vlan60-services.network ${D}${sysconfdir}/systemd/network/

    # Install persistent forwarding sysctl config
    install -d ${D}${sysconfdir}/sysctl.d
    install -m 0644 ${UNPACKDIR}/90-vantageos-router.conf ${D}${sysconfdir}/sysctl.d/

    # Install captive portal DNS config
    install -d ${D}${sysconfdir}/dnsmasq.d
    install -m 0644 ${UNPACKDIR}/dnsmasq.conf ${D}${sysconfdir}/dnsmasq.d/vantageos.conf
    
    # Install hostapd config
    install -d ${D}${sysconfdir}/hostapd
    install -m 0644 ${UNPACKDIR}/hostapd.conf ${D}${sysconfdir}/hostapd/vantageos-hostapd.conf
    # 0600: carries wpa_passphrase in plaintext, same as hostapd.conf's wpa_psk
    install -m 0600 ${UNPACKDIR}/hostapd-guest.conf ${D}${sysconfdir}/hostapd/vantageos-hostapd-guest.conf
    install -m 0600 ${UNPACKDIR}/wpa_psk ${D}${sysconfdir}/hostapd/wpa_psk

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
    ${sysconfdir}/systemd/network/10-alfa-wlan.link \
    ${sysconfdir}/systemd/network/25-wlan1.network \
    ${sysconfdir}/systemd/network/26-wlan0.network \
    ${sysconfdir}/systemd/network/30-vlan5-quarantine.network \
    ${sysconfdir}/systemd/network/31-vlan10-trusted.network \
    ${sysconfdir}/systemd/network/33-vlan30-cloud.network \
    ${sysconfdir}/systemd/network/34-vlan40-cam.network \
    ${sysconfdir}/systemd/network/35-vlan50-iot.network \
    ${sysconfdir}/systemd/network/36-vlan60-services.network \
    ${sysconfdir}/dnsmasq.d/vantageos.conf \
    ${sysconfdir}/sysctl.d/90-vantageos-router.conf \
    ${sysconfdir}/hostapd/vantageos-hostapd.conf \
    ${sysconfdir}/hostapd/vantageos-hostapd-guest.conf \
    ${sysconfdir}/hostapd/wpa_psk \
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
    ipset \
"

SYSTEMD_SERVICE:${PN} = "vantageos-hostapd.service vantageos-router.service"
