SUMMARY = "VantageOS SecurityHub UI and backend"
DESCRIPTION = "SecurityHub PWA frontend plus local Go API for VantageOS -- router, access \
point and IDS/IPS in one appliance. Replaces the legacy vantageos-control-plane."
LICENSE = "MIT"
LIC_FILES_CHKSUM = "file://${COMMON_LICENSE_DIR}/MIT;md5=0835ade698e0bcf8506ecda2f7b4f302"

inherit externalsrc systemd goarch useradd

FILESEXTRAPATHS:prepend := "${THISDIR}/files:"

SRC_URI = " \
    file://config.yaml \
    file://security-hub-keygen.service \
    file://nginx/security-hub.conf \
    file://nginx/security-hub-api.conf \
    file://nginx/security-hub-frontend.conf \
    file://generate-cert.sh \
    file://security-hub-cert.service \
    file://securityhub-tmpfiles.conf \
    file://security-hub-smoke-test-runner.sh \
    file://security-hub-smoke-test.service \
    file://security-hub-smoke-test.timer \
    file://smoke-test.env.example \
"

# security-hub/backend and security-hub/frontend are sibling git submodules (v/.gitmodules)
# pinned by the superproject, not fetched by bitbake. externalsrc just builds off whatever is
# checked out on disk at these paths -- same trick vantageos-control-plane used for
# control-plane/, just pointed at the new location. Neither submodule is ever written to by
# this recipe.
SECURITY_HUB_BACKEND_SRC = "${@os.path.abspath(os.path.join(d.getVar('TOPDIR'), '../../../security-hub/backend'))}"
SECURITY_HUB_FRONTEND_SRC = "${@os.path.abspath(os.path.join(d.getVar('TOPDIR'), '../../../security-hub/frontend'))}"
EXTERNALSRC = "${SECURITY_HUB_BACKEND_SRC}"
EXTERNALSRC_BUILD = "${WORKDIR}/security-hub-build"

DEPENDS = "nodejs-native go-native"

# openssl-bin: security-hub-cert.service's generate-cert.sh shells out to `openssl req`/`openssl
# x509`, not just openssl(1)'s library -- already in IMAGE_INSTALL, but a runtime dependency this
# recipe actually exercises should be declared here rather than borrowed from the image recipe.
RDEPENDS:${PN} = "nginx openssl-bin"

# System account the API runs as (deploy/vantageos-api.service: User=securityhub
# Group=securityhub). No login, no home -- the unit only needs it for capability scoping.
USERADD_PACKAGES = "${PN}"
GROUPADD_PARAM:${PN} = "--system securityhub"
USERADD_PARAM:${PN} = "--system --no-create-home --shell /usr/sbin/nologin --gid securityhub securityhub"

SYSTEMD_SERVICE:${PN} = "security-hub-keygen.service security-hub-api.service security-hub-cert.service security-hub-smoke-test.timer"
# Enabled by default: security-hub-api applies its rendered firewall ruleset unattended from
# the very first boot. Apply() arms a 60s timer (system.firewall_confirm_seconds) that reverts
# to the previous ruleset unless Confirm() cancels it first -- but Confirm() only proves the
# local `nft -f` + DB write succeeded, not that the operator's management path (tailscale0/SSH)
# is still reachable; a ruleset that installs cleanly but wrongly omits the management-admit
# rule is not caught by this timer. The actual safety net is the renderer admitting tailscale0
# first (security-hub's own SYSTEM-INTEGRATION.md §8a), not this window.
#
# security-hub-smoke-test.service is deliberately NOT listed here -- it has no [Install]
# section (it only ever runs Unit=-triggered by the timer above, never by a target), so nothing
# needs to auto-enable it directly. Enabling the timer is what makes the polling start at boot;
# see security-hub-smoke-test-runner.sh for why it still no-ops until the operator bootstraps.
SYSTEMD_AUTO_ENABLE:${PN} = "enable"

do_compile() {
    export HOME=${WORKDIR}
    export CI=true
    export npm_config_cache=${WORKDIR}/npm-cache
    export PATH="${STAGING_BINDIR_NATIVE}:$PATH"

    cd ${SECURITY_HUB_FRONTEND_SRC}
    npm ci
    npx tsc -b
    npx vite build --outDir ${B}/frontend-dist

    cd ${S}
    export GOOS="${TARGET_GOOS}"
    export GOARCH="${TARGET_GOARCH}"
    export CGO_ENABLED="0"
    export GOCACHE=${WORKDIR}/go-cache
    export GOMODCACHE=${WORKDIR}/go-mod-cache
    go build -trimpath -o ${B}/security-hub-api ./cmd/api
}

do_install() {
    install -d ${D}${datadir}/security-hub-ui
    cp -R ${B}/frontend-dist/. ${D}${datadir}/security-hub-ui/

    install -d ${D}${bindir}
    install -m 0755 ${B}/security-hub-api ${D}${bindir}/security-hub-api

    install -d ${D}${sysconfdir}/securityhub
    install -m 0644 ${UNPACKDIR}/config.yaml ${D}${sysconfdir}/securityhub/config.yaml

    install -d ${D}${sysconfdir}/nginx/conf.d ${D}${sysconfdir}/nginx/snippets
    install -m 0644 ${UNPACKDIR}/nginx/security-hub.conf ${D}${sysconfdir}/nginx/conf.d/security-hub.conf
    install -m 0644 ${UNPACKDIR}/nginx/security-hub-api.conf ${D}${sysconfdir}/nginx/snippets/security-hub-api.conf
    install -m 0644 ${UNPACKDIR}/nginx/security-hub-frontend.conf ${D}${sysconfdir}/nginx/snippets/security-hub-frontend.conf

    install -m 0755 ${UNPACKDIR}/generate-cert.sh ${D}${bindir}/security-hub-generate-cert.sh
    install -m 0755 ${UNPACKDIR}/security-hub-smoke-test-runner.sh ${D}${bindir}/security-hub-smoke-test-runner.sh
    # security-hub/backend's own deploy/smoke-test.sh -- vendored from the submodule at build
    # time like vantageos-api.service above, not duplicated into this recipe's files/.
    install -m 0755 ${SECURITY_HUB_BACKEND_SRC}/deploy/smoke-test.sh ${D}${bindir}/security-hub-smoke-test

    install -d ${D}${sysconfdir}/tmpfiles.d
    install -m 0644 ${UNPACKDIR}/securityhub-tmpfiles.conf ${D}${sysconfdir}/tmpfiles.d/security-hub.conf

    install -m 0644 ${UNPACKDIR}/smoke-test.env.example ${D}${sysconfdir}/securityhub/smoke-test.env.example

    install -d ${D}${systemd_system_unitdir}
    install -m 0644 ${UNPACKDIR}/security-hub-keygen.service ${D}${systemd_system_unitdir}/security-hub-keygen.service
    install -m 0644 ${UNPACKDIR}/security-hub-cert.service ${D}${systemd_system_unitdir}/security-hub-cert.service
    install -m 0644 ${UNPACKDIR}/security-hub-smoke-test.service ${D}${systemd_system_unitdir}/security-hub-smoke-test.service
    install -m 0644 ${UNPACKDIR}/security-hub-smoke-test.timer ${D}${systemd_system_unitdir}/security-hub-smoke-test.timer

    # security-hub/backend's own deploy/vantageos-api.service ships ExecStart=/usr/bin/security_hub,
    # which matches neither this recipe's binary name nor the Makefile's own bin/securityhub-api --
    # an upstream inconsistency. Patched here on a copy, at install time, in the working directory
    # only; deploy/vantageos-api.service inside the submodule is never touched.
    # ReadWritePaths patch: ProtectSystem=strict makes the whole filesystem read-only for the
    # unprivileged service process except what's listed here. Upstream's unit only lists
    # /var/lib/securityhub and /tmp -- the reconciler also needs to write dnsmasq reservations
    # (/etc/dnsmasq.d) and the hostapd PSK file (/etc/hostapd), and hostapd_cli (shelled out to
    # by internal/system/linux/hostapd.go) needs to create its own client control socket inside
    # /run/hostapd (ctrl_interface_group=securityhub in hostapd.conf was already set up to allow
    # exactly this at the POSIX layer -- ReadWritePaths= was the missing piece). Without the
    # first two, every reconcile cycle fails with "read-only file system" and the firewall
    # ruleset never gets Confirm()ed, so it flaps apply/revert on a ~90s loop.
    sed \
        -e 's#^ExecStart=.*#ExecStart=${bindir}/security-hub-api#' \
        -e '/^ExecStart=/a ExecStartPre=+/bin/chgrp securityhub /etc/hostapd\nExecStartPre=+/bin/chmod 0775 /etc/hostapd\nExecStartPre=+/bin/chgrp securityhub /etc/hostapd/wpa_psk\nExecStartPre=+/bin/chmod 0660 /etc/hostapd/wpa_psk\nExecStartPre=+/bin/chgrp securityhub /etc/dnsmasq.d\nExecStartPre=+/bin/chmod 0775 /etc/dnsmasq.d\nEnvironmentFile=-/etc/securityhub/device_key.env' \
        -e 's#^Conflicts=vantageos-router.service#After=vantageos-router.service#' \
        -e '/^After=vantageos-router.service/a Requires=security-hub-keygen.service\nAfter=security-hub-keygen.service' \
        -e 's#^ReadWritePaths=.*#ReadWritePaths=/var/lib/securityhub /tmp /etc/dnsmasq.d /etc/hostapd /run/hostapd#' \
        ${SECURITY_HUB_BACKEND_SRC}/deploy/vantageos-api.service > ${WORKDIR}/security-hub-api.service.patched
    install -m 0644 ${WORKDIR}/security-hub-api.service.patched ${D}${systemd_system_unitdir}/security-hub-api.service
}

FILES:${PN} = " \
    ${datadir}/security-hub-ui \
    ${bindir}/security-hub-api \
    ${bindir}/security-hub-generate-cert.sh \
    ${bindir}/security-hub-smoke-test \
    ${bindir}/security-hub-smoke-test-runner.sh \
    ${sysconfdir}/securityhub/config.yaml \
    ${sysconfdir}/securityhub/smoke-test.env.example \
    ${sysconfdir}/nginx/conf.d/security-hub.conf \
    ${sysconfdir}/nginx/snippets/security-hub-api.conf \
    ${sysconfdir}/nginx/snippets/security-hub-frontend.conf \
    ${sysconfdir}/tmpfiles.d/security-hub.conf \
    ${systemd_system_unitdir}/security-hub-api.service \
    ${systemd_system_unitdir}/security-hub-keygen.service \
    ${systemd_system_unitdir}/security-hub-cert.service \
    ${systemd_system_unitdir}/security-hub-smoke-test.service \
    ${systemd_system_unitdir}/security-hub-smoke-test.timer \
"

# The backend is a statically linked (CGO_ENABLED=0) Go binary, so GNU_HASH does not apply.
INSANE_SKIP:${PN} += "ldflags"
