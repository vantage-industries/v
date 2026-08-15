SUMMARY = "VantageOS SecurityHub UI and backend"
DESCRIPTION = "SecurityHub PWA frontend plus local Go API for VantageOS -- router, access \
point and IDS/IPS in one appliance. Replaces the legacy vantageos-control-plane."
LICENSE = "MIT"
LIC_FILES_CHKSUM = "file://${COMMON_LICENSE_DIR}/MIT;md5=0835ade698e0bcf8506ecda2f7b4f302"

inherit externalsrc systemd goarch useradd

FILESEXTRAPATHS:prepend := "${THISDIR}/files:"

SRC_URI = " \
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

# System account the API runs as (deploy/security-hub-api.service: User=securityhub
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
    # security-hub/backend's own deploy/config.production.yaml, vendored as-is like
    # vantageos-api.service above -- destination filename must stay config.yaml, since
    # internal/config/config.go hardcodes v.SetConfigName("config") (viper looks for
    # <workingdir>/config.yaml; WorkingDirectory=/etc/securityhub in vantageos-api.service).
    # A missing/misnamed file is NOT a fatal error there (ConfigFileNotFoundError is
    # swallowed) -- it silently falls back to defaults (environment=development, mock
    # backend), so getting this filename wrong fails silent, not loud.
    install -m 0644 ${SECURITY_HUB_BACKEND_SRC}/deploy/config.production.yaml ${D}${sysconfdir}/securityhub/config.yaml

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

    # security-hub/backend's own deploy/security-hub-api.service is packaged as-is -- it already
    # carries the correct ExecStart binary name, the hostapd/dnsmasq ExecStartPre ownership
    # fixups, the security-hub-keygen.service ordering, and the ReadWritePaths needed under
    # ProtectSystem=strict. See that repo's deploy/security-hub-api.service for the rationale
    # (previously carried here as a build-time sed patch; moved upstream so the unit under
    # version control matches what actually ships). Renamed from vantageos-api.service upstream
    # in commit f14317b ("Updated config files for embeded yocto packages", 2026-08-14).
    install -m 0644 ${SECURITY_HUB_BACKEND_SRC}/deploy/security-hub-api.service ${D}${systemd_system_unitdir}/security-hub-api.service
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
