SUMMARY = "VantageOS base image"
DESCRIPTION = "Custom Linux image for VantageOS devices"
LICENSE = "MIT"

inherit core-image

IMAGE_INSTALL:append = " \
    packagegroup-core-boot \
    packagegroup-base-extended \
    vantageos-routing-config \
    dropbear \
"

export IMAGE_BASENAME = "vantageos-image"
