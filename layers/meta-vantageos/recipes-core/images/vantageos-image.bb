SUMMARY = "VantageOS base image"
DESCRIPTION = "Custom Linux image for VantageOS devices"
LICENSE = "MIT"

inherit core-image

IMAGE_INSTALL_append = "packagegroup-core-boot packagegroup-base-extended"

export IMAGE_BASENAME = "vantageos-image"
