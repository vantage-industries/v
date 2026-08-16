> **Notatka robocza.** Uporządkowana, aktualna dokumentacja jest w
> [`DOKUMENTACJA.md`](DOKUMENTACJA.md) (sekcja "Rozwiązywanie problemów"). Ten plik
> zostaje jako zapis procesu nauki.

# Wprowadzenie
Podczas pierwszego builda Yocto nie miałem problemów, ale bezmyślnie jechałem po kolei i nie rozumiałem, w jaki sposób ten proces powinien przebiegać. Za drugim razem nie obyło się bez błędów, więc zostawiam to dla potomnych.

# Q&A

## Folder bitbake jest pusty
Submodule nie zainstalowały się poprawnie. Dzieje się tak, gdy robimy zwykłe `git clone`.

Użyj:
```bash
git submodule update --init --recursive
```

## Problem z pierwszym buildem
Pierwszą rzeczą, jaką należy zrobić, jest dodanie swojego użytkownika do grupy `docker`.
Robimy to dlatego, że w skrypcie bashowym dołączonym do projektu pobieramy UID z ENV.
Jeżeli użyjemy `sudo` albo będziemy zalogowani jako `root`, system będzie próbował użyć roota (a to nie jest OK, bo bierze UID 0, który jest już używany).

## Permission denied podczas bitbake
Jeżeli pojawia się taki błąd:
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
To znaczy, że `bitbake-builds` NIE ISTNIAŁ przed uruchomieniem Dockera, a Docker utworzył ten katalog jako `root`.

Wystarczy ręcznie utworzyć folder i nadać mu poprawnego właściciela.