# The stock dnsmasq.conf ships with "conf-dir=/etc/dnsmasq.d" commented out,
# so vantageos-routing-config's /etc/dnsmasq.d/vantageos.conf is never read.
# Enable the drop-in dir so that file actually takes effect.
do_install:append() {
    echo "conf-dir=/etc/dnsmasq.d/,*.conf" >> ${D}${sysconfdir}/dnsmasq.conf
}
