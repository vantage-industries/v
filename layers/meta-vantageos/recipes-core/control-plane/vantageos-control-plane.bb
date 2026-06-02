SUMMARY = "VantageOS control plane UI and backend"
DESCRIPTION = "Static SPA frontend plus local Go API for VantageOS"
LICENSE = "MIT"
LIC_FILES_CHKSUM = "file://LICENSE;md5=d462c0efd0687ee7aa138bac25146313"

inherit externalsrc systemd

CONTROL_PLANE_SRC = "${@os.path.abspath(os.path.join(d.getVar('TOPDIR'), '../../../control-plane'))}"
EXTERNALSRC = "${CONTROL_PLANE_SRC}"
EXTERNALSRC_BUILD = "${WORKDIR}/control-plane-build"

DEPENDS = "nodejs-native go-native"

RDEPENDS:${PN} = " \
    nginx \
    openssl \
"

SYSTEMD_SERVICE:${PN} = "vantageos-api.service vantageos-resetd.service vantageos-certgen.service"
SYSTEMD_AUTO_ENABLE:${PN} = "enable"

do_compile() {
    export HOME=${WORKDIR}
    export CI=true
    export GOCACHE=${WORKDIR}/go-cache
    export GOMODCACHE=${WORKDIR}/go-mod-cache
    export COREPACK_ENABLE_DOWNLOAD_PROMPT=0
    export PNPM_HOME=${WORKDIR}/pnpm-home
    export PATH="${STAGING_BINDIR_NATIVE}:${PNPM_HOME}:$PATH"

    cd ${S}/frontend
    corepack enable
    corepack prepare pnpm@11.1.2 --activate
    pnpm install --no-frozen-lockfile
    pnpm exec vite build --outDir ${B}/frontend-dist

    cd ${S}/backend
    go build -trimpath -o ${B}/vantageos-api ./cmd/vantageos-api
    go build -trimpath -o ${B}/vantageos-resetd ./cmd/vantageos-resetd
}

do_install() {
    install -d ${D}${datadir}/vantageos-ui
    cp -R ${B}/frontend-dist/. ${D}${datadir}/vantageos-ui/

    install -d ${D}${bindir}
    install -m 0755 ${B}/vantageos-api ${D}${bindir}/vantageos-api
    install -m 0755 ${B}/vantageos-resetd ${D}${bindir}/vantageos-resetd

    install -d ${D}${sysconfdir}/nginx/conf.d
    install -m 0644 ${S}/deploy/nginx/vantageos.conf ${D}${sysconfdir}/nginx/conf.d/vantageos.conf

    install -d ${D}${systemd_system_unitdir}
    install -m 0644 ${S}/deploy/systemd/vantageos-api.service ${D}${systemd_system_unitdir}/vantageos-api.service
    install -m 0644 ${S}/deploy/systemd/vantageos-resetd.service ${D}${systemd_system_unitdir}/vantageos-resetd.service
    install -m 0644 ${S}/deploy/systemd/vantageos-certgen.service ${D}${systemd_system_unitdir}/vantageos-certgen.service
}

FILES:${PN} = " \
    ${datadir}/vantageos-ui \
    ${bindir}/vantageos-api \
    ${bindir}/vantageos-resetd \
    ${sysconfdir}/nginx/conf.d/vantageos.conf \
    ${systemd_system_unitdir}/vantageos-api.service \
    ${systemd_system_unitdir}/vantageos-resetd.service \
    ${systemd_system_unitdir}/vantageos-certgen.service \
"

# The backend is a statically linked Go binary, so GNU_HASH does not apply.
INSANE_SKIP:${PN} += "ldflags"
