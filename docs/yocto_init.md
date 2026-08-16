> **Notatka robocza.** Uporządkowana, aktualna dokumentacja jest w
> [`DOKUMENTACJA.md`](DOKUMENTACJA.md) (sekcja "Szybki start"). Ten plik zostaje jako
> zapis procesu nauki przy pierwszym buildzie.

# inicjalizacja projektu yocto
zaczynamy od https://docs.yoctoproject.org/brief-yoctoprojectqs/index.html

## ogólne przemyślenia

### build server
będziemy musieli mieć maszyne do buildowania.
jakiegoś vpsa który będzie nam budował nasz system
- aby mieć automatyczne buildy gdy nowego gh releasea damy
- aby być niezależnym od mocy obliczeniowej naszych komputerków
- abyśmy łatwiej wspólnie mogli testować systemy i je budować
- wtedy każdy korzysta z tego samego build cache

### pakiety openwrt
możemy integrować pakiety z openwrt za pomocą
https://layers.openembedded.org/layerindex/branch/master/layer/meta-openwrt/


## odpalamy sobie wszystko w dockerze 
- aby każdy miał ten sam dev setup 
- aby łatwo wszędzie odpalić builda
- aby mieć czysty setup do buildu czyli żadne konfiguracje naszego systemu nie będą wpływały na konfiguracje wewnątrz kontenera
- aby jedynym wymaganiem była instalacja dockera

więc musisz mieć na systemie zainstalowany docker oraz bash (aby odpalić skrypt pomocniczy), to wszystko.


## skrypt pomocniczy
przeczytaj sobie skrypt aby wiedziec dokładnie co on robi

przy pierwszym razie odpalenia kontenera:
```
./yocto-docker.sh build
```
potem możemy wejść w powłoke w kontenerze za pomocą:
```
./yocto-docker.sh shell
```

pozostałe komendy wykonujemy w kontenerze:


## decyzje przy ./bitbake/bin/bitbake-setup init

### konfiguracje wybieramy oe-nodistro-whinlatter
w oe-nodistro wybieramy dokładne komponenty systemu, bez żadnych domyślnych komponentów.
wersje master to takie najnowsze "bety" z nowymi fajnymi rzeczami ale nie potrzebujemy ich więc wybieramy whinlatter czyli aktualny release.

poky natomiast to taki zbiór testowych pakietów yocto które tworzą pewną dystrybucje tak jak powiedzmy ubuntu buduje na debianie to poky buduje na yocto projekcie: https://docs.yoctoproject.org/ref-manual/terms.html#term-Poky

### target wybieramy machine/qemuarm64
ponieważ rpi to komputer arm64, i aby przetestować na jak najpodobniejszym środowisku wybieramy vmke z tą samą architekturą

## inicjalizujemy środowisko
```bash
cd bitbake-builds
source vantageos/build/init-build-env
```
załaduje to nam komende bitbake do środowiska tak abyśmy mogli ją uruchomić

## dodajemy fragmenty
fragmenty to zbiory konfiguracji oraz pakietów(programów)

- core/yocto/sstate-mirror-cdn - aby zamiast budować wszystko od 0 móc skorzystać z gotowo zbudowanych pakietów z serwerów yocto
- core/yocto/root-login-with-empty-password - abyśmy mogli się łatwo zalogować w vmce bez hasła

```bash
bitbake-config-build enable-fragment <nazwa fragmentu>
```

## budujemy!

używamy bitbake aby zbudować obraz systemu najbardziej minimalny,
by build był szybki, ale z konsolą by móc sobie poklikać, by 
czyli wybieramy target "core-image-base"

> [!NOTE]
> zobacz sobie layers/openembedded-core/meta/recipes-core/images/core-image-base.bb
> oraz inne definicje obrazów w tym folderze aby zrozumieć jak jest zdefiniowany ten obraz systemu
> są to poprostu dodatkowe reguły które ustalają jaki ten obraz ma być, 
> dodatkowo do naszej gównej konfiguracji nad którą pózniej będziemy pracować

```bash
bitbake core-image-base
```

na mojej maszynce build pierwszy zajął około 30min
użyliśmy sstate-mirror-cdn, więc dużo pobrano gotowców zamiast budować wszystko od 0
następne buildy już będą szybsze
