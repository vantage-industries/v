FILESEXTRAPATHS:prepend := "${THISDIR}/files:"

# Inline (IPS) on the IoT VLAN via NFQUEUE instead of passive AF_PACKET on
# eth0. vantageos.nft hands matching packets to queue 0 with the nftables
# `bypass` flag, so if this daemon isn't running the traffic just flows
# unfiltered -- fail-open by design, never fail-closed.
PACKAGECONFIG:append = " nfq"

SRC_URI += " \
    file://vantageos-suricata-modules.conf \
    file://vantageos-suricata-sysctl.conf \
    file://vantageos-suricata-logrotate.conf \
"

do_install:append() {
    # NFQUEUE, not eth0's AF_PACKET -- queue number matches `queue num 0` in
    # vantageos.nft.
    sed -i 's|-i eth0|-q 0|' ${D}${systemd_unitdir}/system/suricata.service

    # mode: accept -- default verdict when no loaded rule matches.
    # fail-open: yes -- mirrors nftables' `bypass` flag one layer down: if
    # the engine itself falls behind/errors internally, packets pass rather
    # than getting silently dropped. Requested explicitly: IoT devices must
    # never lose connectivity because of the IDS/IPS.
    sed -i '/^nfq:$/a\  mode: accept\n  fail-open: yes' ${D}${sysconfdir}/suricata/suricata.yaml

    # Single vendored ruleset (vantageos-suricata-rules) instead of the
    # upstream ET-classic file list, none of which we ship.
    sed -i -e '/^rule-files:/,/^classification-file:/{/^ - /d; /^# - /d}' \
           -e '/^rule-files:/a\ - suricata.rules' \
           ${D}${sysconfdir}/suricata/suricata.yaml

    # nfnetlink_queue: feeds -q 0 above. br_netfilter: makes traffic between
    # two devices on the same VLAN bridge (normally pure L2, invisible to
    # netfilter) traverse the forward hook too -- see the sysctl below and
    # vantageos.nft's device-to-device rule for the other half of this.
    install -d ${D}${sysconfdir}/modules-load.d
    install -m 0644 ${UNPACKDIR}/vantageos-suricata-modules.conf ${D}${sysconfdir}/modules-load.d/vantageos-suricata.conf

    install -d ${D}${sysconfdir}/sysctl.d
    install -m 0644 ${UNPACKDIR}/vantageos-suricata-sysctl.conf ${D}${sysconfdir}/sysctl.d/91-vantageos-suricata.conf

    install -d ${D}${sysconfdir}/logrotate.d
    install -m 0644 ${UNPACKDIR}/vantageos-suricata-logrotate.conf ${D}${sysconfdir}/logrotate.d/suricata
}

FILES:${PN} += " \
    ${sysconfdir}/modules-load.d/vantageos-suricata.conf \
    ${sysconfdir}/sysctl.d/91-vantageos-suricata.conf \
    ${sysconfdir}/logrotate.d/suricata \
"
