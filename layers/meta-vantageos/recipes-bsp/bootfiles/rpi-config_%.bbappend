do_deploy:append:raspberrypi5() {
    echo "dtoverlay=dwc2,dr_mode=peripheral" >> ${DEPLOYDIR}/${BOOTFILES_DIR_NAME}/config.txt
}
