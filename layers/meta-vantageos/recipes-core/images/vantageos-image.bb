SUMMARY = "VantageOS base image"
DESCRIPTION = "Custom Linux image for VantageOS devices"
LICENSE = "MIT"

inherit core-image
inherit extrausers

DEPENDS:append = " openssl-native "

# Reproducible root password configuration
PASSWD_SALT = "vantsalt"
ROOT_PASSWORD = "vantageos"

MACHINE_FEATURES:remove = "alsa"

IMAGE_INSTALL = " \
    packagegroup-core-boot \
    packagegroup-base-extended \
    vantageos-routing-config \
    vantageos-suricata-config \
    dropbear \
    iptables \
    suricata \
    hostapd \
    kernel-modules \
    udev-rules-rpi \
    linux-firmware-rpidistro-bcm43455 \
    linux-firmware-rpidistro-bcm43456 \
    iw \
"
# packagegroup-base-extended

DISTRO = "vantageos"
export IMAGE_BASENAME = "vantageos-image"

# Enable root login (dropbear is installed manually below)
IMAGE_FEATURES:append = "allow-root-login serial-autologin-root"

# Set root password reproducibly during rootfs creation
vantageos_set_root_password() {
    local hash
    hash=$(openssl passwd -1 -salt ${PASSWD_SALT} ${ROOT_PASSWORD})
    sed -i "s|^root:[^:]*:|root:${hash}:|" ${IMAGE_ROOTFS}${sysconfdir}/shadow
}
ROOTFS_POSTPROCESS_COMMAND += "vantageos_set_root_password;"

# Install SSH authorized key for root dropbear access
vantageos_install_ssh_key() {
    install -d -m 0700 ${IMAGE_ROOTFS}${sysconfdir}/dropbear
    install -m 0600 /dev/null ${IMAGE_ROOTFS}${sysconfdir}/dropbear/authorized_keys
    cat <<'EOF' > ${IMAGE_ROOTFS}${sysconfdir}/dropbear/authorized_keys
ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIG90D8qYbIN08D5ctMiiKkln2w75FrD2MLvCQmWTpSqQ user@pc
EOF
}
ROOTFS_POSTPROCESS_COMMAND += "vantageos_install_ssh_key;"

# Manually enable dropbear service (don't use ssh-server-dropbear to avoid openssh deps)
SYSTEMD_AUTO_ENABLE:dropbear = "enable"
SYSTEMD_AUTO_ENABLE:suricata = "enable"
SYSTEMD_AUTO_ENABLE:hostapd = "enable"
SYSTEMD_AUTO_ENABLE:wpa-supplicant = "disable"
INITSCRIPT_PACKAGES:append = " dropbear"
INITSCRIPT_NAME:dropbear = "dropbear"
INITSCRIPT_PARAMS:dropbear = "defaults 10"
