# Dokumentacja projektu VantageOS (Yocto)

> Ten dokument jest głównym, aktualnym źródłem wiedzy o warstwie Yocto projektu VantageOS.
> Konsoliduje i porządkuje wcześniejsze, rozrzucone notatki z tego katalogu
> (`yocto_init.md`, `common_issues.md`, `QnA.md`, `multi_ssids.md`) — te pliki zostają jako
> zapis historyczny procesu nauki, ale od teraz **to jest dokument, który należy czytać i
> aktualizować w pierwszej kolejności**.
>
> Dokumentacja aplikacji security-hub (backend/frontend) żyje w jej własnym repozytorium —
> zobacz `v/security-hub/backend/CLAUDE.md` i `SYSTEM-INTEGRATION.md`. Ten dokument opisuje
> tylko warstwę Yocto: jak się buduje obraz, jak jest zorganizowana warstwa `meta-vantageos`
> i jak spinają się ze sobą recepty, sieć i security-hub.

## Spis treści

1. [Czym jest VantageOS](#czym-jest-vantageos)
2. [Struktura repozytorium](#struktura-repozytorium)
3. [Szybki start](#szybki-start)
4. [Konfiguracja dystrybucji i maszyny](#konfiguracja-dystrybucji-i-maszyny)
5. [Obraz systemu — `vantageos-image.bb`](#obraz-systemu--vantageos-imagebb)
6. [Recepty warstwy `meta-vantageos`](#recepty-warstwy-meta-vantageos)
7. [Architektura sieci](#architektura-sieci)
8. [Integracja z security-hub](#integracja-z-security-hub)
9. [IDS/IPS — Suricata inline](#idsips--suricata-inline)
10. [Zasady pracy z repozytorium](#zasady-pracy-z-repozytorium)
11. [Rozwiązywanie problemów](#rozwiązywanie-problemów)
12. [Znane ograniczenia i otwarte tematy](#znane-ograniczenia-i-otwarte-tematy)
13. [Zobacz też](#zobacz-też)

---

## Czym jest VantageOS

VantageOS to niestandardowa dystrybucja Linuksa budowana w Yocto, przeznaczona na
Raspberry Pi 5, która zamienia go w **security appliance**: punkt dostępowy Wi-Fi +
router + IDS/IPS w jednym urządzeniu. Segmentuje sieć domową na wiele VLAN-ów
(zaufane urządzenia, goście, IoT, kamery, chmura, usługi), przypisuje urządzenia do
segmentów na podstawie unikalnego hasła Wi-Fi per-urządzenie, a cały ruch między
segmentami przechodzi przez firewall (`nftables`) i inline IDS/IPS (`suricata`).

Zarządzanie tym wszystkim (dodawanie urządzeń, przydział do VLAN-u, podgląd logów,
apply/rollback konfiguracji) odbywa się przez aplikację **security-hub** — lokalne
API w Go + PWA we React/Vite, serwowane z samego urządzenia przez `nginx`. Warstwa
Yocto opisana w tym dokumencie tylko **pakuje i integruje** tę aplikację z systemem;
sam kod aplikacji żyje w osobnym repozytorium (patrz [Integracja z
security-hub](#integracja-z-security-hub)).

Projekt zastąpił wcześniejszą, prostszą architekturę o nazwie `control-plane`
(2 strefy: Main/IoT, tylko 5 GHz, Flask + SQLite) — ten wcześniejszy projekt jest
opisany historycznie w `architecture.md` i `implementation-checklist.md` w tym samym
katalogu, oznaczonych jako *Superseded*.

## Struktura repozytorium

Repozytorium `v/` (submoduł/checkout `vantage-industries/v`) to cały workspace Yocto:

```
v/
├── bitbake/                  # narzędzie bitbake (submoduł, upstream, nie edytować)
├── layers/
│   ├── meta-openembedded/    # warstwa upstream (submoduł, nie edytować)
│   └── meta-vantageos/       # WARSTWA WŁASNA — tu się edytuje
├── bitbake-builds/vantageos/ # faktyczne drzewo builda (build/, layers/ — symlinki)
├── security-hub/
│   ├── backend/               # submoduł github.com/vantage-industries/security-hub, gałąź dev
│   └── frontend/              # submoduł github.com/vantage-industries/frontend_security_hub
├── config/                   # sources-fixed-revisions.json — zapięte rewizje warstw
├── docs/                     # ten katalog
├── scripts/                  # m.in. flash-image.sh
├── yocto-docker.sh           # skrypt pomocniczy do buildowania w Dockerze
├── docker-compose.yml, Dockerfile
├── vantageos.conf.json       # konfiguracja `bitbake-setup` (źródła, gałęzie, fragmenty)
└── README.md                 # skrócony quick-start
```

Poza `v/` (obok niego, na tym samym hoście) istnieje `security-hub/` — samodzielny,
żywy checkout backendu (`vantage-industries/security-hub`, gałąź `dev`), z tego samego
repo/zdalnego co submoduł `v/security-hub/backend`. To jest miejsce, w którym faktycznie
edytuje się kod backendu (patrz [Zasady pracy z
repozytorium](#zasady-pracy-z-repozytorium)).

### `v/layers/meta-vantageos/` — mapa plików

```
conf/
  distro/vantageos.conf          # definicja dystrybucji "vantageos"
  machine/qemuarm64.conf         # maszyna QEMU ARMv8 (środowisko testowe)
  layer.conf                     # rejestracja warstwy w BitBake

recipes-bsp/bootfiles/
  rpi-config_%.bbappend          # dopisuje dtoverlay=dwc2 (USB peripheral) na RPi5

recipes-connectivity/hostapd/
  hostapd_2.11.bbappend          # włącza CONFIG_FULL_DYNAMIC_VLAN i CONFIG_IEEE80211W

recipes-core/
  images/vantageos-image.bb                    # główna recepta obrazu
  security-hub/vantageos-security-hub.bb        # pakuje security-hub (backend+frontend)
  vantageos-routing-config/vantageos-routing-config.bb  # sieć: networkd/hostapd/dnsmasq/nftables
  vantageos-usb-gadget/vantageos-usb-gadget.bb   # tryb USB-gadget (debug only)

recipes-ids/suricata/
  suricata_%.bbappend            # suricata jako inline IPS (NFQUEUE zamiast AF_PACKET)

recipes-kernel/rtw89-usb/
  rtw89-usb_git.bb               # sterownik dla adaptera Alfa AWUS036AX (wlan1)

recipes-security/vantageos-suricata-rules/
  vantageos-suricata-rules.bb    # własny, ręcznie utrzymywany zestaw reguł IDS/IPS

recipes-support/dnsmasq/
  dnsmasq_%.bbappend             # włącza conf-dir=/etc/dnsmasq.d
```

Pełniejsza, na bieżąco aktualizowana mapa (z dodatkowymi faktami sprzętowymi) jest w
`yocto/PROJECT_MAP.md` (poza tym repozytorium git, lokalny plik nawigacyjny).

## Szybki start

Wymagania: Docker + Docker Compose, ~50 GB wolnego miejsca, ~8 GB RAM.

```bash
# 1. Klon z submodułami
git clone --recursive git@github.com:vantage-industries/v.git
cd v

# 2. Zbuduj obraz Dockera z narzędziami Yocto
./yocto-docker.sh build

# 3. Zainicjalizuj środowisko builda VantageOS (bitbake-setup)
./yocto-docker.sh init

# 4. Zbuduj obraz systemu (pierwszy raz: 30–60 min)
./yocto-docker.sh bitbake vantageos-image

# 5. Uruchom w QEMU
./yocto-docker.sh runqemu
```

Inne przydatne komendy `yocto-docker.sh`: `shell` (wchodzi w powłokę kontenera bez
odpalania bitbake), `setup` (bezpośredni dostęp do `bitbake-setup`).

Domyślna maszyna to `qemuarm64` (QEMU ARM64 — środowisko testowe najbliższe realnemu
RPi5 pod względem architektury). Docelowy sprzęt to `raspberrypi5` (warstwa
`meta-raspberrypi`, patrz `vantageos.conf.json` → `oe-fragments-one-of.machine`). Zmiana
maszyny: edytuj `MACHINE` w `bitbake-builds/vantageos/build/conf/local.conf`.

Flashowanie obrazu na kartę SD: `nix develop` (dostarcza `bmaptool`), potem
`scripts/flash-image.sh /dev/sdX`.

> **Nie buduj obrazu samodzielnie, chyba że użytkownik wyraźnie o to poprosi** — build
> jest kosztowny (RAM/czas) na tym hoście, patrz [Zasady pracy z
> repozytorium](#zasady-pracy-z-repozytorium).

## Konfiguracja dystrybucji i maszyny

`conf/distro/vantageos.conf` definiuje dystrybucję `vantageos`:

- `INIT_MANAGER = "systemd"` — systemd jako init, nie sysvinit.
- `PACKAGE_CLASSES = "package_ipk"` — pakiety w formacie `.ipk`.
- `DISTRO_FEATURES_DEFAULT` obejmuje m.in. `wifi`, `usbhost`, `seccomp`, `security`, `ipv4
  ipv6`, `nfs`, `pci`.
- Niestandardowy `FETCHCMD_wget` z opisowym User-Agentem — crates.io odrzuca domyślny
  UA BitBake HTTP 403 (potrzebne np. przy pobieraniu zależności Rust dla suricaty przez
  `crate://`).
- `LICENSE_FLAGS_ACCEPTED += "synaptics-killswitch"` — akceptacja niestandardowej
  licencji wymaganej przez jeden z pakietów.

`conf/machine/qemuarm64.conf` konfiguruje QEMU jako maszynę ARMv8 (cortex-a57), z
`MACHINE_FEATURES = "bluetooth usbgadget screen vfat"` (bez `alsa` — usuwane też
explicite w `vantageos-image.bb`).

`BBLAYERS` builda (`bitbake-builds/vantageos/build/conf/bblayers.conf`) ładuje, oprócz
`openembedded-core` i `meta-vantageos`: `meta-openembedded` (`meta-oe`, `meta-python`,
`meta-networking`, `meta-webserver`), `meta-security`, `meta-raspberrypi`.

## Obraz systemu — `vantageos-image.bb`

`recipes-core/images/vantageos-image.bb` to główna recepta obrazu. Kluczowe elementy:

- `IMAGE_ROOTFS_EXTRA_SPACE = "10485760"` — 10 GB (w KiB) dodatkowego wolnego miejsca w
  partycji root, nie osobna partycja.
- Pakiety instalowane przez `IMAGE_INSTALL`: `vantageos-routing-config`,
  `vantageos-suricata-rules`, `vantageos-security-hub`, `suricata`, `hostapd`,
  `dnsmasq`, `dropbear`, `iptables`, sterowniki Wi-Fi (`rtw89-usb`,
  `linux-firmware-rpidistro-bcm4345{5,6}`, `linux-firmware-rtl8852`), `iw`,
  `net-snmp-{server-snmpd,client}`, `openssl`/`openssl-bin`, `logrotate`, `tzdata`.
- **Zmienne opt-in** (domyślnie wyłączone, żeby obraz produkcyjny nie wyciekał
  sekretów ani nie miał znanego hasła root):
  - `VANTAGEOS_ENABLE_DEV_ACCESS` (0/1) — włącza `allow-root-login`,
    `serial-autologin-root`, hasłuje `root` (`VANTAGEOS_ROOT_PASSWORD`, openssl
    passwd -1) i wgrywa `VANTAGEOS_SSH_PUBLIC_KEY` do `/root/.ssh/authorized_keys`.
    Włącza też usługę `dropbear` (SSH server).
  - `VANTAGEOS_ENABLE_TAILSCALE` (0/1) — dokłada pakiety `tailscale`/`tailscaled` do
    obrazu; dołączenie do tailnetu to zawsze akcja runtime (`tailscale up` albo klucz
    dostarczony poza buildem), nigdy sekret wpisany na etapie budowania.
  - `VANTAGEOS_ENABLE_CAPTIVE_PORTAL` (domyślnie `1`) — generuje samopodpisany
    certyfikat TLS dla `VANTAGEOS_PORTAL_HOSTNAME` (`vantageos.local`) i
    `VANTAGEOS_PORTAL_IP` (`192.168.88.1`), usuwa domyślny `nginx` `default_server`.
  - `VANTAGEOS_TIMEZONE` (domyślnie `Europe/Warsaw`) — ustawiana przez symlink
    `/etc/localtime`.
- `SYSTEMD_AUTO_ENABLE`: `dnsmasq`, `suricata`, `nginx` — włączone zawsze;
  `hostapd`/`wpa-supplicant` — **wyłączone** (hostapd jest odpalany przez własną
  jednostkę `vantageos-hostapd.service`, patrz niżej); `dropbear`/`tailscaled` —
  warunkowo, w zależności od zmiennych opt-in powyżej.

Wartości `VANTAGEOS_*` ustawiane są zwykle w `local.conf` builda (np.
`VANTAGEOS_ENABLE_DEV_ACCESS = "1"` do celów developerskich).

## Recepty warstwy `meta-vantageos`

### `vantageos-security-hub` (`recipes-core/security-hub/`)

Pakuje frontend (PWA, React/Vite) i backend (Go) security-hub. Oba są siostrzanymi
submodułami pod `v/security-hub/` (nie fetchowane przez bitbake) — recepta korzysta z
`externalsrc` i buduje wprost z tego, co jest wypięte na dysku
(`EXTERNALSRC = SECURITY_HUB_BACKEND_SRC`).

- `do_compile`: `npm ci` + `vite build` dla frontendu (do `frontend-dist`), `go build
  -trimpath` (CGO wyłączone) dla backendu → binarka `security-hub-api`.
- `do_install` instaluje: statyczny frontend do
  `${datadir}/security-hub-ui`, binarkę do `${bindir}/security-hub-api`,
  konfiguracje `nginx` (3 pliki: główny + snippet API + snippet frontend),
  `config.yaml` **wprost z submodułu** (`security-hub/backend/deploy/config.production.yaml`
  — submoduł jest źródłem prawdy, nazwa pliku docelowego musi zostać `config.yaml`, bo
  Viper w backendzie hardkoduje tę nazwę), skrypt generowania certyfikatu,
  smoke-test runner + timer, oraz jednostkę `security-hub-api.service`
  (też wprost z submodułu — `deploy/security-hub-api.service`).
- Konto systemowe `securityhub` (bez logowania, bez home) — API działa jako ten
  użytkownik.
- Jednostki systemd: `security-hub-keygen.service` (generuje klucz urządzenia przy
  pierwszym boocie), `security-hub-api.service`, `security-hub-cert.service`,
  `security-hub-smoke-test.timer` — wszystkie **enabled by default**. API aplikuje
  własny wyrenderowany firewall od pierwszego boota; ma 60s okno na `Confirm()` zanim
  wróci do poprzedniego rulesetu (ale to nie chroni przed błędną regułą admitującą
  interfejs zarządzania — patrz `SYSTEM-INTEGRATION.md` w submodule).

### `vantageos-routing-config` (`recipes-core/vantageos-routing-config/`)

Największa recepta sieciowa — instaluje wszystko co potrzebne, żeby RPi5 działał jako
router/AP: pliki `systemd-networkd` (`.network`/`.link` dla wlan0/wlan1 i 6 mostków
VLAN), `hostapd.conf`/`hostapd-guest.conf`, `dnsmasq.conf` (per-VLAN DHCP),
`vantageos.nft` (firewall/NAT deklaratywny), `routing-setup.sh` (skrypt aplikujący
firewall + limit pasma dla gości), sysctl do forwardowania IP, oraz **pliki-ziarna**
(`wpa_psk`, `reservations`) pod `/var/lib/securityhub/rendered/` — instalowane jako
placeholder, żeby hostapd/dnsmasq miały co czytać na pierwszym boocie, zanim
security-hub-api zdąży je nadpisać (patrz [Integracja z
security-hub](#integracja-z-security-hub)).

Instaluje i włącza własne jednostki `vantageos-hostapd.service` i
`vantageos-router.service` (RDEPENDS: `iproute2`, `iproute2-tc`, `hostapd`, `dnsmasq`,
`nftables`).

### `vantageos-usb-gadget` (`recipes-core/vantageos-usb-gadget/`)

Konfiguracja trybu USB-gadget na RPi5 (bridge `br0`, interfejsy `usb0`/`usb1`,
`kernel-module-libcomposite`). **Wyłącznie do debugowania** (patrz [Znane
ograniczenia](#znane-ograniczenia-i-otwarte-tematy)) — nie jest to funkcja produktowa
launch scope'u.

### `suricata_%.bbappend` (`recipes-ids/suricata/`)

Zamienia domyślny tryb pasywny suricaty (`AF_PACKET` na `eth0`) na **inline IPS przez
NFQUEUE** (`queue num 0` w `vantageos.nft`, zgodnie z flagą `bypass` — jeśli suricata nie
działa, ruch po prostu przechodzi, fail-open, nigdy fail-closed). Ustawia `mode: accept`
+ `fail-open: yes` w `suricata.yaml`, i podmienia listę `rule-files` na ścieżki
bezwzględne do `suricata.rules`/`local.rules` z recepty `vantageos-suricata-rules`
(bezwzględna ścieżka jest konieczna — bare filename rozwiązuje się względem
`default-rule-path`, który dalej wskazuje na `/var/lib/suricata/rules`, gdzie nic nie
jest instalowane; potwierdzone na urządzeniu, że przy błędnej ścieżce suricata cicho
ładuje zero reguł). Doładowuje moduł jądra `nfnetlink_queue` oraz `br_netfilter`
(potrzebny, żeby ruch między urządzeniami na tym samym moście VLAN — normalnie czysto
L2, niewidoczny dla netfiltera — też przechodził przez hook forward).

### `vantageos-suricata-rules` (`recipes-security/vantageos-suricata-rules/`)

Własny, ręcznie utrzymywany zestaw reguł (`suricata.rules` + `local.rules`),
aktualizowany w miarę pojawiania się nowych sygnatur IoT/IPS. `PR` trzeba bumpować
ręcznie przy każdej zmianie plików, żeby rebuild rzeczywiście podniósł nową wersję.

### `rtw89-usb` (`recipes-kernel/rtw89-usb/`)

Backport sterownika `rtw89` (mac80211) z transportem USB dla adapterów RTL8852BU (np.
Alfa AWUS036AX). Wbudowany w jądro sterownik `rtw89` obsługuje tylko PCIe (USB
(`CONFIG_RTW89_8852BU`) trafił do mainline dopiero w Linuksie 6.17, a to jądro to
6.12) — bez tego backportu dostępny byłby tylko wendorowy blob Realtek, bez trybu
monitor i bez `AP_VLAN`. `KERNEL_MODULE_AUTOLOAD` ładuje moduł automatycznie.

### `hostapd_2.11.bbappend` (`recipes-connectivity/hostapd/`)

Dopisuje do configu builda `hostapd`: `CONFIG_FULL_DYNAMIC_VLAN=y` (wymagane przez
`dynamic_vlan=` w `hostapd.conf` — przypisanie VLAN per-stacja na podstawie
`wpa_psk_file`, kluczowe dla modelu segmentacji security-hub) i
`CONFIG_IEEE80211W=y` (Protected Management Frames, `ieee80211w=`).

### `dnsmasq_%.bbappend` (`recipes-support/dnsmasq/`)

Domyślny `dnsmasq.conf` ma `conf-dir=/etc/dnsmasq.d` zakomentowane — bez tej łatki
plik `vantageos-routing-config`'a (`/etc/dnsmasq.d/vantageos.conf`) nigdy nie byłby
wczytywany.

### `rpi-config_%.bbappend` (`recipes-bsp/bootfiles/`)

Dla maszyny `raspberrypi5` dopisuje `dtoverlay=dwc2,dr_mode=peripheral` do
`config.txt` — potrzebne dla trybu USB-gadget.

## Architektura sieci

VantageOS segmentuje sieć na osiem VLAN-ów (jeden, Management/99, celowo nie jest
wdrożony wg dokumentu projektowego):

| VLAN | Nazwa | Podsieć | Sposób podpięcia |
|---|---|---|---|
| 5 | Kwarantanna / onboarding | `192.168.5.0/24` | most `br-vlan5` |
| 10 | Zaufane (trusted) | `192.168.10.0/24` | most `br-vlan10` |
| 20 | Goście (Guest) | `192.168.20.0/24` | **bezpośrednio na `wlan0`**, nie most |
| 30 | Cloud native (np. Xiaomi, TV) | `192.168.30.0/24` | most `br-vlan30` |
| 40 | Kamery (CAM) | `192.168.40.0/24` | most `br-vlan40` |
| 50 | IoT offline / hybryda | `192.168.50.0/24` | most `br-vlan50` |
| 60 | Usługi (Home Assistant, NVR) | `192.168.60.0/24` | most `br-vlan60` |
| 99 | Management | — | **nie wdrożony** |

### Dwa radia, dwie role

Urządzenie ma dwie karty Wi-Fi, każda ogranicza się do **jednego** równoczesnego
interfejsu w roli AP (potwierdzone przez `iw phy` na obu kartach) — stąd konieczność
rozdzielenia SSID między dwie fizyczne karty zamiast dwóch BSS-ów na jednej:

- **wlan1** = zewnętrzny adapter USB Alfa AWUS036AX (sterownik `rtw89_8852bu_git`,
  przypięty do tego konkretnego urządzenia przez `10-alfa-wlan.link`). SSID
  `VantageOS`, `hw_mode=g`, kanał 6 (2.4 GHz). Obsługuje **dynamic VLAN**: każda
  stacja po autentykacji WPA2-PSK dostaje VLAN na podstawie wpisu w
  `wpa_psk_file=/var/lib/securityhub/rendered/wpa-psk` (wpis wildcard → domyślnie
  VLAN 5/kwarantanna). To jedno SSID obsługuje wszystkie VLAN-y oprócz Guest.
- **wlan0** = wbudowane radio RPi5 (`brcmfmac`, rodzina bcm4329-fmac). SSID
  `VantageOS-Guest`, `hw_mode=a`, kanał 36 (5 GHz, non-DFS). **Routowane, nie
  mostkowane** — `brcmfmac` odrzuca dołączenie do mostu (`EOPNOTSUPP`, potwierdzone na
  urządzeniu), więc podsieć gości `192.168.20.0/24` wisi bezpośrednio na `wlan0`.
  Jedno, statyczne hasło WPA2-PSK, bez per-klienckiego VLAN-u (bo tu każda stacja z
  definicji jest gościem) i bez portalu captive.

> Przydział pasm (od 2026-08-09): Alfa/wlan1 na 2.4 GHz, Guest/wlan0 na 5 GHz —
> odwrotnie niż pierwotny plan, bo wsparcie 5 GHz wbudowanego radia RPi5 nie było
> jeszcze zweryfikowane sprzętowo na tym konkretnym egzemplarzu.

### Firewall i routing

`vantageos.nft` to deklaratywny ruleset `nftables` (ładowany w całości przez `nft -f`,
zaczyna się od `flush ruleset` — idempotentny, bezpieczny do ponownego uruchomienia).
`routing-setup.sh` rozwiązuje dwie zmienne runtime (`VANTAGEOS_WAN_IFACE`, domyślnie
`eth0`, i opcjonalny `VANTAGEOS_ADMIN_IP`) do małego pliku `include`, aplikuje pełny
ruleset, włącza IP forwarding (`sysctl`) i ustawia limit pasma dla gości (`tc htb`, 20
Mbit domyślnie, `VANTAGEOS_GUEST_BW_MBIT`) na interfejsie `wlan0`.

DHCP dla każdego VLAN-u obsługuje `dnsmasq` — osobny `dhcp-range`/`listen-address` na
każdym moście (plik `dnsmasq.conf` w recepcie `vantageos-routing-config`), plus
rezerwacje DHCP renderowane w locie przez security-hub (`dhcp-hostsfile=`, czytane
tylko przy starcie — reconciler po każdej zmianie wysyła `dnsmasq` SIGHUP zamiast
restartu).

## Integracja z security-hub

Kod aplikacji (Go backend + React/Vite frontend) żyje w osobnych repozytoriach,
wpiętych jako submoduły w `v/security-hub/{backend,frontend}`. Warstwa Yocto nigdy nie
edytuje submodułów — pakuje tylko to, co jest na dysku (`externalsrc`) i wprost
przenosi pliki dostarczane przez sam backend (`config.production.yaml` →
`config.yaml`, `security-hub-api.service`, `deploy/smoke-test.sh`) zamiast trzymać ich
własne kopie/widelce w recepcie.

Kluczowy mechanizm współdzielenia stanu: pliki
`/var/lib/securityhub/rendered/{wpa-psk,reservations}` mają **dwóch właścicieli w
czasie**:

1. **Build time** — `vantageos-routing-config` instaluje plik-ziarno (statyczny wpis
   wildcard → VLAN 5 dla PSK, pusty plik dla rezerwacji) z uprawnieniami `root:root
   0600`, żeby `hostapd`/`dnsmasq` miały co czytać zanim API w ogóle wystartuje.
2. **Runtime** — po starcie `security-hub-api`, jego reconciler nadpisuje cały plik
   PSK na podstawie żywych rekordów `DevicePSK` przy każdym cyklu konwergencji;
   plik-ziarno spodziewanie zostaje wtedy nadpisany przy pierwszym takim przebiegu.
   `ExecStartPre=+` w jednostce API robi `chgrp`/`chmod` obu plików do
   `securityhub:securityhub 0660` przy każdym starcie (bo `vantageos-routing-config`
   instaluje je jako `root:root 0600`).

Ta sama zasada dotyczy `config.yaml` API — plik jest instalowany wprost z submodułu
(`deploy/config.production.yaml`), nie ma już własnej, rozjeżdżającej się kopii w
recepcie Yocto (stan na 2026-08-13). Nazwa pliku docelowego **musi** zostać
`config.yaml` — Viper w kodzie backendu hardkoduje tę nazwę (`SetConfigName("config")`),
a brak/zła nazwa pliku **nie jest błędem fatalnym** (błąd jest połykany), tylko cichym
fallbackiem do wartości domyślnych (`environment=development`, mock backend) — czyli
błąd w tym miejscu psuje się cicho, nie głośno.

## IDS/IPS — Suricata inline

Suricata działa jako **inline IPS** (nie pasywny sniffer) przez `NFQUEUE` — pakiety na
`queue num 0` w `vantageos.nft` trafiają do suricaty zanim opuszczą router; suricata
decyduje `ACCEPT`/`DROP`. Cały mechanizm jest **fail-open z założenia**: flaga
`bypass` na kolejce nftables i `fail-open: yes` w konfiguracji suricaty gwarantują, że
jeśli demon nie działa albo się zapcha, ruch przechodzi bez filtrowania zamiast zostać
zablokowany — świadoma decyzja projektowa, żeby awaria IDS/IPS nigdy nie odcinała
urządzeniom IoT łączności.

Reguły pochodzą wyłącznie z własnego zestawu (`vantageos-suricata-rules`), nie z
domyślnej listy upstream ET-classic — trzeba pamiętać o bumpowaniu `PR` w tej recepcie
przy każdej zmianie plików `.rules`, inaczej rebuild nie podniesie nowej wersji.

## Zasady pracy z repozytorium

- **Cała praca nad Yocto/bitbake/warstwami/receptami dzieje się w `v/`.** Warstwa do
  edycji to `v/layers/meta-vantageos/` — edytuj **zawsze** tę ścieżkę na hoście, nigdy
  `v/bitbake-builds/vantageos/layers/meta-vantageos` (to symlink w mount kontenera
  Dockera, nie źródło prawdy).
- **Nigdy nie edytuj plików wewnątrz submodułów** (`v/security-hub/backend`,
  `v/security-hub/frontend`, `v/bitbake`, `v/layers/meta-openembedded`). Zmiany w
  kodzie backendu robi się w samodzielnym checkoucie `/home/sonat/pjbl/yocto/security-hub`
  (to samo repo/branch `dev` co submoduł) — edycja, commit, push upstream, a potem
  osobnym krokiem bump wskaźnika submodułu. Dla frontendu obecnie **nie ma** takiego
  samodzielnego checkoutu — trzeba by najpierw sklonować
  `vantage-industries/frontend_security_hub`.
- **Host budujący ma 7,6 GB RAM** — nigdy nie odpalaj dwóch `docker compose run` z
  bitbake równolegle (buildy `qemuarm64` i `raspberrypi5` muszą być sekwencyjne, nie
  równoległe). `BB_NUMBER_THREADS="3"` i `PARALLEL_MAKE="-j 3"` w `local.conf` są
  celowo niskie — przy wyższych wartościach obserwowano OOM/korupcję builda.
- **Nie buduj obrazu samodzielnie**, chyba że użytkownik wyraźnie o to poprosi.

## Rozwiązywanie problemów

Skonsolidowane z `common_issues.md`/`QnA.md` (pierwsze podejścia do builda):

**Katalog `bitbake/` jest pusty** — submoduły nie zostały zainicjalizowane (typowe po
zwykłym `git clone` bez `--recursive`). Napraw:
```bash
git submodule update --init --recursive
```

**Błąd przy pierwszym uruchomieniu skryptu / build jako root** — dodaj swojego
użytkownika do grupy `docker`. Skrypt bashowy pobiera UID ze zmiennej środowiskowej;
uruchomienie przez `sudo` albo jako `root` sprawi, że system spróbuje użyć UID 0, co
koliduje z rzeczywistym rootem hosta.

**`PermissionError: [Errno 13] Permission denied: '.../bitbake-builds/site.conf'`** —
katalog `bitbake-builds` nie istniał przed pierwszym uruchomieniem Dockera, więc Docker
utworzył go jako `root`. Napraw ręcznie tworząc katalog i nadając mu poprawnego
właściciela przed kolejną próbą.

**Dodałem warstwę do configu, ale nic się nie zmieniło** — sprawdź, czy warstwa
faktycznie się ładuje:
```bash
./yocto-docker.sh shell
source ~/bitbake-builds/vantageos/build/init-build-env
bitbake-layers show-layers
```

**Suricata "ładuje" reguły, ale nic nie wykrywa** — sprawdź, czy `rule-files` w
`/etc/suricata/suricata.yaml` wskazuje na ścieżkę bezwzględną
(`${sysconfdir}/suricata/rules/suricata.rules`), nie samą nazwę pliku — bare filename
rozwiązuje się względem `default-rule-path` (`/var/lib/suricata/rules`), gdzie nic nie
jest zainstalowane, i suricata cicho ładuje zero reguł.

**Dostęp administracyjny (SSH/panel) przestaje działać po starcie/zmianie
security-hub-api** — reconciler API renderuje własny ruleset `nftables`, który
dopuszcza ruch `input` tylko z interfejsów wymienionych w
`network.management_interfaces` w `config.production.yaml`. Jeśli interfejs, przez
który się łączysz, nie jest na tej liście, ruch jest po cichu odrzucany, gdy tylko
ruleset renderera zastąpi stary, budowany na etapie instalacji (ten wcześniejszy
ufał `eth0` bezwarunkowo — tylko na potrzeby test-rigów). Napraw dopisując właściwy
interfejs do `network.management_interfaces` w tym pliku.

## Znane ograniczenia i otwarte tematy

- **Jeden SSID na kartę Wi-Fi.** Obie karty (wbudowana `brcmfmac` i zewnętrzna
  `rtw89`) obsługują tylko jeden równoczesny interfejs w roli AP — nie da się
  wystawić dwóch niezależnych SSID z jednej karty. Stąd bieżące rozwiązanie: Guest na
  osobnej karcie (`wlan0`), reszta VLAN-ów jako dynamic VLAN na jednym SSID
  (`wlan1`). Alternatywa rozważana wcześniej: przypisanie VLAN-u po adresie MAC
  zamiast po PSK — niewykorzystana.
- **USB-gadget tylko do debugowania**, nie jest częścią zakresu startowego produktu.
- **2,4 GHz jako fallback ogólny** (nie tylko dla Alfy) jest planowane na później —
  start skupia się na obecnym przydziale pasm.
- **VLAN 99 (Management)** nie jest wdrożony, mimo że jest częścią dokumentu
  projektowego segmentacji.
- Wsparcie 5 GHz wbudowanego radia RPi5 nie było jeszcze zweryfikowane sprzętowo na
  danym egzemplarzu w momencie ustalania przydziału pasm — warto to zweryfikować przy
  kolejnej zmianie układu kanałów.

## Zobacz też

- `yocto/PROJECT_MAP.md` — szczegółowa, na bieżąco aktualizowana mapa plików i faktów
  sprzętowych/sieciowych (poza tym repozytorium git).
- `yocto/CLAUDE.md` — twarde zasady pracy z tym repozytorium.
- `v/README.md` — skrócony quick-start budowania.
- `v/docs/architecture.md`, `v/docs/implementation-checklist.md` — historyczny opis
  zastąpionej architektury `control-plane` (oznaczone *Superseded*).
- `v/docs/yocto_init.md`, `v/docs/common_issues.md`, `v/docs/QnA.md`,
  `v/docs/multi_ssids.md` — oryginalne, robocze notatki (zachowane jako zapis procesu).
- `v/security-hub/backend/CLAUDE.md`, `v/security-hub/backend/SYSTEM-INTEGRATION.md` —
  dokumentacja samej aplikacji security-hub (wewnątrz submodułu).
