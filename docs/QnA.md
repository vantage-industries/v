# Wprowadzenie
Stworzłem to Q&A ze wzgędu na kilka rzeczy ktore nie poszly idealnie


jeżeli pojawiły się błędy radzę dopisać do skryptu buildowego flagę —no-cache
# Pierwszy build
## Błąd przy pierwszym uruchomieniu skryptu  
Pierwszą rzeczą jaką należy zapewnić jest dodać swojego użytkownika do grupy docker
robimy to ze względu na to iż w skrypcie bashowym dołączonym do projektu pobieramy UID z env.
Jeżeli użyjemy sudo albo będziey zlogowani jako root system będzie próbował zalterować roota (a to jest nie okej)

następnie ważnym elementem jest sprawdzenie czy submodules się poprawnie zainstalowały
git submodule update --init --recursive
Jeżeli pojawia się error:

```

A common site.conf file will be created, please check it is correct before running builds
    /home/builder/bitbake-builds/site.conf

Traceback (most recent call last):
  File "/home/builder/./bitbake/bin/bitbake-setup", line 1166, in <module>
    main()
  File "/home/builder/./bitbake/bin/bitbake-setup", line 1156, in main
    init_config(top_dir, all_settings, args)
  File "/home/builder/./bitbake/bin/bitbake-setup", line 626, in init_config
    create_siteconf(top_dir, args.non_interactive, settings)
  File "/home/builder/./bitbake/bin/bitbake-setup", line 900, in create_siteconf
    with open(siteconfpath, 'w') as siteconffile:
         ^^^^^^^^^^^^^^^^^^^^^^^
PermissionError: [Errno 13] Permission denied: '/home/builder/bitbake-builds/site.conf'
```

To bitbake-builds NIE ISTNIAŁ przed sworzeniem dokcera

# Dodanie layaru do bitbake
Dodalem layer do config ale nic sie nie zmienilo >:(

Spoko byczq a zobacz czy wgl sa ladowane 
```
/yocto-docker.sh shell
source ~/bitbake-builds/vantageos/build/init-build-env
bitbake-layers show-layers
```