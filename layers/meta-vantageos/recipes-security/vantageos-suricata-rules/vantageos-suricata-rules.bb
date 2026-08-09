DESCRIPTION = "Curated Suricata ruleset for VantageOS, hand-maintained and vendored at build time"
LICENSE = "CLOSED"

FILESEXTRAPATHS:prepend := "${THISDIR}/files:"

# Hand-curated, edited outside the build as research turns up new IoT/IPS
# signatures -- bump PR whenever files/suricata.rules changes so a rebuild
# actually picks it up.
PR = "r1"

SRC_URI = "file://suricata.rules"

# Using S = ${WORKDIR} is no longer supported
# S = "${WORKDIR}"

do_install() {
    install -d ${D}${sysconfdir}/suricata/rules
    install -m 0644 ${UNPACKDIR}/suricata.rules ${D}${sysconfdir}/suricata/rules/suricata.rules
}

FILES:${PN} = "${sysconfdir}/suricata/rules/suricata.rules"

RDEPENDS:${PN} = "suricata"
