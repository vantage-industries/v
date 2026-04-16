DESCRIPTION = "VantageOS routing configuration"
LICENSE = "MIT"
LIC_FILES_CHKSUM = "file://${COMMON_LICENSE_DIR}/MIT;md5=0835ade698e0bcf8506ecda2f7b4f302"

inherit systemd

FILESEXTRAPATHS:prepend := "${THISDIR}/files:"

# Bump PR to force rebuild
PR = "r3"

SRC_URI = " \
    file://10-vantageos-routing.network \
    file://25-wlan0.network \
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
    
    # Install hostapd config
    install -d ${D}${sysconfdir}/hostapd
    install -m 0644 ${UNPACKDIR}/hostapd.conf ${D}${sysconfdir}/hostapd/vantageos-hostapd.conf
    
    # Install routing setup script
    install -d ${D}${bindir}
    install -m 0755 ${UNPACKDIR}/routing-setup.sh ${D}${bindir}/vantageos-routing-setup
    
    # Install systemd services
    install -d ${D}${systemd_system_unitdir}
    install -m 0644 ${UNPACKDIR}/vantageos-hostapd.service ${D}${systemd_system_unitdir}/
    install -m 0644 ${UNPACKDIR}/vantageos-router.service ${D}${systemd_system_unitdir}/
    
    # Create marker file to verify installation
    echo "VantageOS routing config installed" > ${D}${sysconfdir}/vantageos-routing-installed
}

pkg_postinst_ontarget:${PN}() {
    # Enable IP forwarding on first boot
    if ! grep -q "net.ipv4.ip_forward=1" /etc/sysctl.conf; then
        echo "net.ipv4.ip_forward=1" >> /etc/sysctl.conf
    fi
    if ! grep -q "net.ipv6.conf.all.forwarding=1" /etc/sysctl.conf; then
        echo "net.ipv6.conf.all.forwarding=1" >> /etc/sysctl.conf
    fi
    sysctl -p || true
    
    systemctl enable vantageos-hostapd.service || true
    systemctl enable vantageos-router.service || true
}

FILES:${PN} = " \
    ${sysconfdir}/systemd/network/10-vantageos-routing.network \
    ${sysconfdir}/systemd/network/25-wlan0.network \
    ${sysconfdir}/hostapd/vantageos-hostapd.conf \
    ${sysconfdir}/vantageos-routing-installed \
    ${bindir}/vantageos-routing-setup \
    ${systemd_system_unitdir}/vantageos-hostapd.service \
    ${systemd_system_unitdir}/vantageos-router.service \
"

RDEPENDS:${PN} = " \
    iproute2 \
    hostapd \
    iptables \
"

SYSTEMD_SERVICE:${PN} = "vantageos-hostapd.service vantageos-router.service"
