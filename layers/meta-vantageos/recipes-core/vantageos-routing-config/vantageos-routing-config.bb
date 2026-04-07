DESCRIPTION = "VantageOS routing configuration"
LICENSE = "MIT"
LIC_FILES_CHKSUM = "file://${COMMON_LICENSE_DIR}/MIT;md5=0835ade698e0bcf8506ecda2f7b4f302"

FILESEXTRAPATHS:prepend := "${THISDIR}/files:"

# Bump PR to force rebuild
PR = "r1"

SRC_URI = " \
    file://10-vantageos-routing.network \
    file://routing-setup.sh \
"

do_install() {
    # Install systemd-networkd routing config
    install -d ${D}${sysconfdir}/systemd/network
    install -m 0644 ${UNPACKDIR}/10-vantageos-routing.network ${D}${sysconfdir}/systemd/network/
    
    # Install routing setup script
    install -d ${D}${bindir}
    install -m 0755 ${UNPACKDIR}/routing-setup.sh ${D}${bindir}/vantageos-routing-setup
    
    # Create marker file to verify installation
    echo "VantageOS routing config installed" > ${D}${sysconfdir}/vantageos-routing-installed
}

pkg_postinst_ontarget:${PN}() {
    # Enable IP forwarding on first boot
    echo "net.ipv4.ip_forward=1" >> /etc/sysctl.conf
    echo "net.ipv6.conf.all.forwarding=1" >> /etc/sysctl.conf
    sysctl -p || true
}

FILES:${PN} = " \
    ${sysconfdir}/systemd/network/10-vantageos-routing.network \
    ${sysconfdir}/vantageos-routing-installed \
    ${bindir}/vantageos-routing-setup \
"

RDEPENDS:${PN} = " \
    iproute2 \
"
