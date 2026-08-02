# CONFIG_FULL_DYNAMIC_VLAN is required for hostapd's dynamic_vlan= directive
# (per-station VLAN assignment via wpa_psk_file) used by the SecurityHub
# segmentation model -- it is not set in meta-openembedded's defconfig.
# CONFIG_IEEE80211W enables the ieee80211w= (PMF) directive -- also unset
# upstream, needed for the onboarding BSS per the SecurityHub design doc.
do_configure:append() {
    echo "CONFIG_FULL_DYNAMIC_VLAN=y" >> ${B}/hostapd/.config
    echo "CONFIG_IEEE80211W=y" >> ${B}/hostapd/.config
}
