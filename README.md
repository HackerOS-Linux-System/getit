# getit

Pobieranie plików, folderów i repozytoriów z GitHub / GitLab / Codeberg.
Wklej link – `getit` sam rozpozna, czy to plik, folder czy repozytorium.

## Użycie

```
getit <link>                        sam wybiera: file / dir / repo
getit file <link>... [-o NAZWA]     pobierz plik(i)
getit dir  <link>    [-o NAZWA]     pobierz folder (albo całe repo bez historii)
getit repo <link> [--clone|--push]  sklonuj repozytorium / wyślij zmiany
getit info <link>                   pokaż, jak getit rozumie link
getit doctor                        sprawdź narzędzia, tokeny i sieć
```

Przykłady:

```
getit https://github.com/user/repo/blob/main/plik.txt     # strona pliku -> pobiera RAW, nie HTML
getit https://github.com/user/repo/tree/main/src          # folder
getit user/repo/tree/main/src                              # skrót = GitHub
getit file https://github.com/user/repo/tree/main/src      # zły podkomenda? getit sam przełączy na `dir`
getit dir  https://github.com/user/repo -o . --merge       # całe repo do bieżącego katalogu
getit dir  https://gitlab.com/group/proj/-/tree/main/docs  # GitLab (przez git sparse-checkout)
getit repo git@github.com:user/repo.git --depth 1
```

## Opcje

| Opcja | Znaczenie |
|-------|-----------|
| `-o NAZWA` | nazwa pliku / folderu wyniku (`-o .` = do bieżącego katalogu) |
| `-d KATALOG` | katalog, w którym ma powstać wynik |
| `-b REF` | gałąź, tag (lub commit przy pobieraniu archiwum) |
| `-f`, `--force` | nadpisz bez pytania (nigdy `/`, `$HOME` ani bieżącego katalogu) |
| `-m`, `--merge` | dołóż pliki do istniejącego folderu |
| `-y`, `--yes` | odpowiadaj „tak” na pytania |
| `-n`, `--dry-run` | pokaż, co zostanie zrobione, bez pobierania |
| `-q`, `--quiet` | tylko błędy; na końcu wypisuje ścieżkę wyniku (do skryptów) |
| `--sparse` | folder przez `git sparse-checkout` (duże repozytoria) |
| `--sha256 SUMA` | sprawdź sumę kontrolną pobranego pliku |
| `--depth N` | głębokość klonu (`repo`) |
| `--no-color` | bez kolorów (też `NO_COLOR`, kolory same wyłączają się przy pipe) |

Stare formy `-clone` i `-push` nadal działają.

## Co getit „się domyśla”

- link do **strony pliku** (`.../blob/...`) → pobiera surowy plik; zapis przez `plik.part`, więc nieudane
  pobieranie nie zostawia śmieci, a strona HTML nigdy nie podszywa się pod plik,
- `file` na linku do folderu → działa jak `dir`, `dir` na linku do pliku → działa jak `file`,
- gałąź ze slashem (`tree/feature/x/src`) rozpoznawana przez `git ls-remote`,
- skrót `user/repo`, adresy `git@host:user/repo.git`, gisty, `raw.githubusercontent.com`,
- istniejący cel: pyta (`nadpisz / scal / anuluj`) albo – bez terminala – podpowiada `--force` / `--merge`,
- błąd → konkretna przyczyna (404, limit zapytań, brak sieci) i podpowiedź, co zrobić.

## Prywatne repozytoria i limity

Ustaw `GITHUB_TOKEN` (lub `GH_TOKEN`), `GITLAB_TOKEN` albo `CODEBERG_TOKEN`.
Token jest wysyłany tylko do serwera, którego dotyczy.

## Wymagania w czasie działania

`curl` (lub `wget`) i `tar`; `git` dla `repo`, `--sparse`, GitLaba/Codeberga i gałęzi ze slashem.
`getit doctor` pokaże, czego brakuje.

## Struktura źródeł

```
src/main.h#       parsowanie argumentów i wybór komendy
src/opts.h#       opcje wiersza poleceń
src/link.h#       rozpoznawanie linków (github / gitlab / gitea / raw / gist / inne)
src/fetch.h#      curl/wget, kody HTTP, archiwa tar.gz, git sparse-checkout, tokeny
src/place.h#      cel zapisu, nadpisywanie / scalanie, bezpieczne przenoszenie
src/cmd_file.h#   getit file
src/cmd_dir.h#    getit dir
src/cmd_repo.h#   getit repo
src/cmd_misc.h#   getit info, getit doctor
src/ui.h#         kolory i komunikaty
src/util.h#       powłoka, napisy, ścieżki
src/help.h#       teksty pomocy
tests/smoke.sh    testy dymne (wymagają sieci)
```

## Budowanie

Wymaga H# (`hacker unpack h#` i `hacker unpack h#-utils`) oraz `bit`.

```
bit build --release
# albo bezpośrednio:
h# compile src/main.h# --release -o build/main
sh tests/smoke.sh build/main
```
