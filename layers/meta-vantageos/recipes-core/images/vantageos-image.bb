SUMMARY = "VantageOS base image"
DESCRIPTION = "Custom Linux image for VantageOS devices"
LICENSE = "MIT"

inherit core-image
inherit extrausers

# # Set root password to 'vantageos' using openssl generated hash
# # Hash generated with: openssl passwd -1 vantageos
# EXTRA_USERS_PARAMS = "usermod -p '$1$xyz$6EhO.Bv5VwzYpK4q1F8fW0' root;"

IMAGE_INSTALL:append = " \
    packagegroup-core-boot \
    packagegroup-base-extended \
    vantageos-routing-config \
    dropbear \
    iptables \
"

DISTRO = "vantageos"
export IMAGE_BASENAME = "vantageos-image"

# Enable root login (dropbear is installed manually below)
IMAGE_FEATURES:append = "empty-root-password allow-root-login serial-autologin-root"

# Manually enable dropbear service (don't use ssh-server-dropbear to avoid openssh deps)
SYSTEMD_AUTO_ENABLE:dropbear = "enable"
INITSCRIPT_PACKAGES:append = " dropbear"
INITSCRIPT_NAME:dropbear = "dropbear"
INITSCRIPT_PARAMS:dropbear = "defaults 10"
