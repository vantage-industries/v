package store

import (
	"fmt"
	"os"
	"strings"
)

const (
	DefaultChannel    = "36"
	CountryCode       = "US"
	MainSubnetCIDR    = "192.168.1.1/24"
	MainSubnetDNS     = "192.168.1.1"
	HostapdConfigPath = "/etc/hostapd/vantageos-hostapd.conf"
	WpaPskFilePath    = "/var/lib/vantageos/wpa_psk_file"
	NetworkdDir       = "/etc/systemd/network"
	Wlan0NetworkdPath = NetworkdDir + "/25-wlan0.network"
	DnsmasqConfigPath = "/etc/dnsmasq.d/vantageos.conf"
)

func SetupSSIDSuffix() string {
	data, err := os.ReadFile("/sys/class/net/wlan0/address")
	if err != nil {
		return "0000"
	}
	mac := strings.TrimSpace(string(data))
	if len(mac) < 17 {
		return "0000"
	}
	return mac[12:14] + mac[15:17]
}

func defaultOperationalSSID() string {
	return "VantageOS-" + SetupSSIDSuffix()
}

func channelFromNetworks(networks []Network) string {
	for _, n := range networks {
		if n.Band == "5ghz" {
			return DefaultChannel
		}
	}
	return DefaultChannel
}

func GenerateSingleSSIDConfig(ssid string, networks []Network) string {
	ch := channelFromNetworks(networks)

	return fmt.Sprintf(`interface=wlan0
driver=nl80211
ssid=%s
hw_mode=a
channel=%s
wpa=2
wpa_key_mgmt=WPA-PSK
rsn_pairwise=CCMP
wpa_psk_file=%s
country_code=%s
ieee80211n=1
ieee80211ac=1
wmm_enabled=1
ieee80211d=1
ieee80211h=1
`, ssid, ch, WpaPskFilePath, CountryCode)
}

func GenerateWpaPskFileEntry(passphrase string) string {
	return fmt.Sprintf("00:00:00:00:00:00 %s\n", passphrase)
}

func GenerateWpaPskFile(psk string, credentials []Credential) string {
	var b strings.Builder
	seen := map[string]struct{}{}
	add := func(secret string) {
		secret = strings.TrimSpace(secret)
		if secret == "" {
			return
		}
		if len(secret) < 8 || len(secret) > 63 {
			return
		}
		if _, exists := seen[secret]; exists {
			return
		}
		seen[secret] = struct{}{}
		b.WriteString(GenerateWpaPskFileEntry(secret))
	}

	add(psk)
	for _, cred := range credentials {
		if cred.Active {
			add(cred.Secret)
		}
	}
	return b.String()
}

func GenerateWlan0NetworkConfigSubnet(cidr string) string {
	return fmt.Sprintf(`[Match]
Name=wlan0

[Network]
Address=%s
DHCPServer=yes
IPForward=yes
LinkLocalAddressing=no

[DHCPServer]
PoolOffset=10
PoolSize=50
EmitDNS=yes
DNS=%s
`, cidr, dnsFromCIDR(cidr))
}

func GenerateDnsmasqConfigOperational() string {
	return fmt.Sprintf(`interface=wlan0
bind-interfaces
listen-address=%s
local-service
no-resolv
no-hosts
domain-needed
bogus-priv
cache-size=0
address=/#/%s
address=/vantageos.local/%s
`, MainSubnetDNS, MainSubnetDNS, MainSubnetDNS)
}

func (s *Store) GetOrCreateSetupPSK() (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.doc.SetupPSK != "" {
		return s.doc.SetupPSK, nil
	}

	psk, err := randomString(12, defaultSecureAlphabet)
	if err != nil {
		return "", err
	}
	s.doc.SetupPSK = psk
	suffix := SetupSSIDSuffix()
	s.doc.SetupSSID = "VantageOS-Setup-" + suffix
	if err := s.saveLocked(); err != nil {
		return "", err
	}
	return psk, nil
}

func (s *Store) SetupSSID() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.doc.SetupSSID != "" {
		return s.doc.SetupSSID
	}
	return "VantageOS-Setup-" + SetupSSIDSuffix()
}

func (s *Store) MainSSID() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, n := range s.doc.Networks {
		if n.Slug == "main" && n.SSID != "" {
			return n.SSID
		}
	}
	return defaultOperationalSSID()
}

func dnsFromCIDR(cidr string) string {
	idx := strings.LastIndexByte(cidr, '/')
	if idx < 0 {
		return cidr
	}
	return cidr[:idx]
}
