# Bygg och publicera Wishlist

GitHub Actions testar appen, bygger containern och publicerar den till GitHub Container Registry (GHCR). Upplägget följer Annovia: kontroller först och publicering från `main` när alla kontroller går igenom.

Compose, DNS och Nginx konfigureras på servern när den första imagen finns i registret.

## Byggpipan

Workflowet finns i `.github/workflows/build.yaml`.

Vid en pull request mot `main` körs:

- Go-tester med både SQLite och en tillfällig PostgreSQL 18.7- och 19.0-databas.
- `go vet`, formatkontroll och kontroll av `go.mod` och `go.sum`.
- Containerbygge för `linux/amd64` och `linux/arm64`.

En pull request publicerar ingen image och behöver inga serverhemligheter.

Vid en push till `main` körs samma kontroller. När de går igenom publiceras:

- `ghcr.io/aekman/wishlist:0.0.1` vid första publiceringen.
- Samma version med `v`-prefix, till exempel `ghcr.io/aekman/wishlist:v0.0.1`.
- `ghcr.io/aekman/wishlist:main`.
- `ghcr.io/aekman/wishlist:sha-<fullständig-commit>`.

Första versionen är alltid `0.0.1`. Därefter bestämmer Conventional Commits sedan senaste releasen nästa version:

- `fix: ...` och övriga godkända typer ger en patchversion, till exempel `0.0.1` till `0.0.2`.
- `feat: ...` ger en minorversion, till exempel `0.0.2` till `0.1.0`.
- `feat!: ...` eller `BREAKING CHANGE:` i commit-meddelandet ger en majorversion, till exempel `0.1.0` till `1.0.0`.

Den största ändringen styr om flera commits ingår. Godkända typer är `feat`, `fix`, `perf`, `refactor`, `docs`, `test`, `build`, `ci`, `chore` och `revert`. Efter första releasen avbryts publiceringen om ett nytt commit-meddelande inte följer formatet.

Använd Conventional Commits i direkta commits till `main`. Använd **Squash and merge** för pull requests och behåll den kontrollerade PR-titeln som commit-titel. Workflowet **Conventional PR title** kontrollerar titeln. Vanliga merge-commits följer inte detta format.

Efter publiceringen skapas en Git-tagg, till exempel `v0.0.1`, och en GitHub Release med referens till imagen. En omkörning återanvänder redan publicerade bytes och slutför en avbruten release. En versionstagg flyttas inte till annan källkod.

`main` följer den senast publicerade versionen. Använd en versionstagg eller en image med digest på servern när du vill köra en bestämd version.

Du kan också starta workflowet manuellt från fliken **Actions**. Välj `main` för att publicera. En manuell körning på en annan gren kör kontrollerna utan publicering.

Workflowet använder GitHubs automatiska `GITHUB_TOKEN`. Publiceringsjobbet har `packages: write` för imagen och `contents: write` för Git-taggar och GitHub Releases. Du behöver inte lägga in ett personligt token eller produktionsdatabasens uppgifter i GitHub. Uppgifterna i workflowet gäller bara den tillfälliga testdatabasen.

## Första publiceringen

1. Kontrollera de lokala ändringarna med `git status` och `git diff`.
2. Skapa en commit med de ändringar du vill publicera. `.env` och databasfiler ska inte ingå.
3. Pusha ändringarna till GitHub. En pull request behöver mergas till `main` för att publicera imagen.
4. Öppna repositoryts flik **Actions** och välj workflowet **Build**.
5. Kontrollera att alla jobb går igenom.
6. Öppna paketet **wishlist** under ditt GitHub-konto och kontrollera att taggen `main` finns.

Efter publiceringen visas imagen med dess digest i workflowets sammanfattning.

GHCR-paket kan vara privata. Kontrollera paketets synlighet och läsrättigheter innan du konfigurerar servern. En publik image kan hämtas utan inloggning:

```sh
docker pull ghcr.io/aekman/wishlist:main
```

För ett privat paket behöver Docker på servern vara inloggat mot `ghcr.io` med rättighet att läsa paketet. Se [GitHubs GHCR-dokumentation](https://docs.github.com/en/packages/working-with-a-github-packages-registry/working-with-the-container-registry).

## PostgreSQL på servern

Skapa en egen databas och en egen användare för Wishlist på din befintliga PostgreSQL-instans. Användaren behöver kunna skapa och ändra appens tabeller i databasens applikationsschema.

Ange anslutningen vid containerstart med `DATABASE_URL`, till exempel:

```text
postgres://wishlist:<url-kodat-lösenord>@<databasvärd>:5432/wishlist?sslmode=<valt-tls-läge>
```

Välj TLS-läge efter serverns databasanslutning och certifikat. Appen ändrar inte ditt val. Ange de verkliga uppgifterna i serverns lokala konfiguration.

När `DATABASE_URL` är satt används PostgreSQL. Appen avbryter starten om anslutningen eller migreringen misslyckas. Den växlar inte över till SQLite. Om `DATABASE_URL` saknas används befintligt SQLite-stöd via `DATABASE_PATH`.

Appen använder [Goose](https://pressly.github.io/goose/documentation/provider/) och kör väntande migreringar vid start. PostgreSQL-migreringarna finns i `internal/store/migrations/postgres/` och byggs in i binären. Ingen separat Goose-installation behövs i containern.

Varje PostgreSQL-migrering körs i en transaktion. Goose låser migreringarna mot parallella appstarter och sparar historiken i `goose_db_version`.

Även SQLite använder Goose. Dess första Go-migrering läser den äldre `PRAGMA user_version` och uppgraderar befintliga filer till baslinjen utan att ta bort data. Vid senare starter styr Goose-historiken vilka migreringar som körs.

Den första PostgreSQL-migreringen kan ta över databasen från den tidigare egna migreringskoden. Befintliga data och delningslänkar behålls. Den gamla tabellen `wishlist_schema_migrations` ersätts av Goose-historiken.

Lägg framtida PostgreSQL-ändringar i en ny numrerad SQL-fil med `-- +goose Up` och `-- +goose Down`. Ändra inte en migrering som redan har publicerats.

Bilder, önskelistor, rangordning, delningslänkar, köpmarkeringar och inloggningssessioner lagras i databasen. Ingen separat bildvolym behövs när PostgreSQL används. Ta backup av hela Wishlist-databasen.

Befintliga SQLite-data kopieras inte automatiskt till PostgreSQL. Den första PostgreSQL-databasen är tom. Behåll SQLite-filen om du senare vill flytta dess data.

## Lokal verifiering

Vanliga tester körs med:

```sh
go test ./...
```

För PostgreSQL-tester behöver du en separat testdatabas. Testerna skapar ett unikt schema per test och tar bort det efteråt. Databasanvändaren behöver rättighet att skapa scheman. Använd inte produktionsdatabasen.

```sh
WISHLIST_TEST_DATABASE_URL='postgres://<testanvändare>:<testlösenord>@127.0.0.1:5432/wishlist_test?sslmode=disable' go test ./... -count=1
```

Utan `WISHLIST_TEST_DATABASE_URL` hoppas PostgreSQL-testerna över. I GitHub Actions är variabeln alltid satt och PostgreSQL-testerna körs.

## Nästa steg

När imagen finns i GHCR kan du lägga till tjänsten i din befintliga Compose-fil. Den behöver:

- `DATABASE_URL` till Wishlist-databasen.
- `FAMILY_PIN` med familjens PIN-kod.
- `BASE_URL=https://wish.andreasekman.com`.
- `LISTEN_ADDR=:8080`.

Anslut containern till databasens nätverk. Om Nginx kör direkt på servern, publicera appens port på serverns loopback-adress.

Den här byggpipan installerar inte appen på servern och ändrar inte DNS eller Nginx.
