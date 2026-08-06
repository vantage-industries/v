SUMMARY = "VantageOS base image"
DESCRIPTION = "Custom Linux image for VantageOS devices"
LICENSE = "MIT"

inherit core-image
inherit extrausers

DEPENDS:append = " openssl-native "

# Dev access is opt-in so production images do not ship with
# a known root password, serial autologin, or baked SSH keys.
VANTAGEOS_ENABLE_DEV_ACCESS ?= "0"
VANTAGEOS_ROOT_PASSWORD ?= ""
VANTAGEOS_SSH_PUBLIC_KEY ?= ""
# Tailscale is opt-in and ships with no auth key baked in -- joining the
# tailnet is a runtime action (interactive `tailscale up` or a key supplied
# out-of-band), never a build-time secret.
VANTAGEOS_ENABLE_TAILSCALE ?= "0"
VANTAGEOS_ENABLE_CAPTIVE_PORTAL ?= "1"
VANTAGEOS_PORTAL_HOSTNAME ?= "vantageos.local"
VANTAGEOS_PORTAL_IP ?= "192.168.88.1"

MACHINE_FEATURES:remove = "alsa"

IMAGE_INSTALL = " \
    packagegroup-core-boot \
    packagegroup-base-extended \
    vantageos-routing-config \
    vantageos-suricata-config \
    dropbear \
    dnsmasq \
    iptables \
    vantageos-control-plane \
    suricata \
    hostapd \
    kernel-modules \
    udev-rules-rpi \
    linux-firmware-rpidistro-bcm43455 \
    linux-firmware-rpidistro-bcm43456 \
    linux-firmware-rtl8852 \
    rtw89-usb \
    iw \
    net-snmp-server-snmpd \
    net-snmp-client \
    openssl \
"
# packagegroup-base-extended

DISTRO = "vantageos"
export IMAGE_BASENAME = "vantageos-image"

IMAGE_FEATURES:append = "${@bb.utils.contains('VANTAGEOS_ENABLE_DEV_ACCESS', '1', ' allow-root-login serial-autologin-root', '', d)}"
IMAGE_INSTALL:append = "${@bb.utils.contains('VANTAGEOS_ENABLE_TAILSCALE', '1', ' tailscale tailscaled', '', d)}"

vantageos_configure_dev_access() {
    if [ "${VANTAGEOS_ENABLE_DEV_ACCESS}" != "1" ]; then
        return 0
    fi

    if [ -n "${VANTAGEOS_ROOT_PASSWORD}" ]; then
        hash=$(openssl passwd -1 -salt vantsalt "${VANTAGEOS_ROOT_PASSWORD}")
        sed -i "s|^root:[^:]*:|root:${hash}:|" ${IMAGE_ROOTFS}${sysconfdir}/shadow
    fi

    if [ -n "${VANTAGEOS_SSH_PUBLIC_KEY}" ]; then
        install -d -m 0700 ${IMAGE_ROOTFS}/root/.ssh
        printf '%s\n' "${VANTAGEOS_SSH_PUBLIC_KEY}" > ${IMAGE_ROOTFS}/root/.ssh/authorized_keys
        chmod 0600 ${IMAGE_ROOTFS}/root/.ssh/authorized_keys
    fi
}
ROOTFS_POSTPROCESS_COMMAND += "vantageos_configure_dev_access;"

vantageos_configure_portal() {
    if [ "${VANTAGEOS_ENABLE_CAPTIVE_PORTAL}" != "1" ]; then
        return 0
    fi

    rm -f ${IMAGE_ROOTFS}${sysconfdir}/nginx/sites-enabled/default_server

    install -d ${IMAGE_ROOTFS}${sysconfdir}/nginx/certs
    openssl_conf=${WORKDIR}/vantageos-portal-openssl.cnf
    cat > ${openssl_conf} <<EOF
[ req ]
default_bits = 2048
prompt = no
default_md = sha256
distinguished_name = req_distinguished_name
x509_extensions = v3_req

[ req_distinguished_name ]
CN = ${VANTAGEOS_PORTAL_HOSTNAME}

[ v3_req ]
subjectAltName = @alt_names
basicConstraints = critical,CA:FALSE
keyUsage = critical,digitalSignature,keyEncipherment
extendedKeyUsage = serverAuth

[ alt_names ]
DNS.1 = ${VANTAGEOS_PORTAL_HOSTNAME}
DNS.2 = localhost
IP.1 = ${VANTAGEOS_PORTAL_IP}
IP.2 = 127.0.0.1
EOF

    openssl req -x509 -nodes -newkey rsa:2048 -days 3650 \
        -keyout ${IMAGE_ROOTFS}${sysconfdir}/nginx/certs/vantageos.key \
        -out ${IMAGE_ROOTFS}${sysconfdir}/nginx/certs/vantageos.crt \
        -config ${openssl_conf} -extensions v3_req >/dev/null 2>&1
    chmod 0600 ${IMAGE_ROOTFS}${sysconfdir}/nginx/certs/vantageos.key
    chmod 0644 ${IMAGE_ROOTFS}${sysconfdir}/nginx/certs/vantageos.crt
}

ROOTFS_POSTPROCESS_COMMAND += "vantageos_configure_portal;"

# Manually enable dropbear service (don't use ssh-server-dropbear to avoid openssh deps)
SYSTEMD_AUTO_ENABLE:dropbear = "${@bb.utils.contains('VANTAGEOS_ENABLE_DEV_ACCESS', '1', 'enable', 'disable', d)}"
SYSTEMD_AUTO_ENABLE:dnsmasq = "enable"
SYSTEMD_AUTO_ENABLE:suricata = "enable"
SYSTEMD_AUTO_ENABLE:hostapd = "disable"
SYSTEMD_AUTO_ENABLE:wpa-supplicant = "disable"
SYSTEMD_AUTO_ENABLE:nginx = "enable"
SYSTEMD_AUTO_ENABLE:tailscaled = "${@bb.utils.contains('VANTAGEOS_ENABLE_TAILSCALE', '1', 'enable', 'disable', d)}"
INITSCRIPT_PACKAGES:append = " dropbear"
INITSCRIPT_NAME:dropbear = "dropbear"
INITSCRIPT_PARAMS:dropbear = "defaults 10"
