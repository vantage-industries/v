DESCRIPTION = "Curated Suricata ruleset for VantageOS, hand-maintained and vendored at build time"
LICENSE = "CLOSED"

FILESEXTRAPATHS:prepend := "${THISDIR}/files:"

# Hand-curated, edited outside the build as research turns up new IoT/IPS
# signatures -- bump PR whenever files/suricata.rules changes so a rebuild
# actually picks it up.
PR = "r2"

SRC_URI = "file://suricata.rules file://local.rules"

# Using S = ${WORKDIR} is no longer supported
# S = "${WORKDIR}"

do_install() {
    install -d ${D}${sysconfdir}/suricata/rules
    install -m 0644 ${UNPACKDIR}/suricata.rules ${D}${sysconfdir}/suricata/rules/suricata.rules
    install -m 0644 ${UNPACKDIR}/local.rules ${D}${sysconfdir}/suricata/rules/local.rules
}

FILES:${PN} = "${sysconfdir}/suricata/rules/suricata.rules ${sysconfdir}/suricata/rules/local.rules"

RDEPENDS:${PN} = "suricata"
