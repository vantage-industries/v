DESCRIPTION = "Persistent Suricata configuration for VantageOS"
LICENSE = "MIT"
LIC_FILES_CHKSUM = "file://${COMMON_LICENSE_DIR}/MIT;md5=0835ade698e0bcf8506ecda2f7b4f302"

FILESEXTRAPATHS:prepend := "${THISDIR}/files:"

SRC_URI = " \
    file://vantageos-suricata-config.sh \
    file://vantageos-suricata-config.service \
"

inherit systemd

SYSTEMD_SERVICE:${PN} = "vantageos-suricata-config.service"
SYSTEMD_AUTO_ENABLE:${PN} = "enable"

RDEPENDS:${PN} = " \
    bash \
    coreutils \
"

do_install() {
    install -d ${D}${libexecdir}
    install -m 0755 ${UNPACKDIR}/vantageos-suricata-config.sh ${D}${libexecdir}/vantageos-suricata-config.sh

    install -d ${D}${systemd_system_unitdir}
    install -m 0644 ${UNPACKDIR}/vantageos-suricata-config.service ${D}${systemd_system_unitdir}/vantageos-suricata-config.service
}

FILES:${PN} = " \
    ${libexecdir}/vantageos-suricata-config.sh \
    ${systemd_system_unitdir}/vantageos-suricata-config.service \
"
