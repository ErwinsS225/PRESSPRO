# Phase 0 — Relevé de validation du 12 octobre 2026

Ce relevé concerne le code existant de `7e432a4` (arbre suivi identique à `main` au SHA `ff15988`).
Aucun test nouveau ajouté. Voir [le rapport d’audit](depot-phase0-audit.md) pour les limites.

## Environnement et commandes

Répertoire frontend : `/home/user/PRESSPRO` ; backend : `/home/user/PRESSPRO/backend`.
Node `v22.22.3`, npm `10.9.8`. Les commandes npm ci-dessous ont réellement été exécutées.
Les sorties complètes sont conservées hors Git dans `/home/user/audit-presspro-phase0/` ; les extraits ci-dessous sont nettoyés des codes ANSI uniquement.
Les empreintes SHA-256 portent sur les logs bruts complets.

## `npm ci --legacy-peer-deps`

Code de sortie : **0**. Log : `npm-ci.log`.

SHA-256 : `9c75d41b811362264cbeb11f85c4c24b72c5480faa992a4a2cccce47e5804b58`.

```text
npm warn deprecated node-domexception@1.0.0: Use your platform's native DOMException instead
npm warn deprecated eslint@9.39.5: This version is no longer supported. Please see https://eslint.org/version-support for other options.

added 492 packages, and audited 493 packages in 11s

97 packages are looking for funding
  run `npm fund` for details

found 0 vulnerabilities
npm notice
npm notice New major version of npm available! 10.9.8 -> 12.2.0
npm notice Changelog: https://github.com/npm/cli/releases/tag/v12.2.0
npm notice To update run: npm install -g npm@12.2.0
npm notice
```

## `npm test`

Code de sortie : **0**. Log : `npm-test.log`.

SHA-256 : `5a1fbf2933924dabde9ff793ea0c5c7549a58f24083c09b2b53b24c613f58d7b`.

```text
✓ src/lib/escpos.test.ts (16 tests) 37ms
 Test Files  25 passed (25)
      Tests  196 passed (196)
   Duration  49.05s (environment 49%, tests 31%, import 9%, setup 7%, transform 3%, worker 1%)
```

## `npm run lint`

Code de sortie : **0**. Log : `lint.log`.

SHA-256 : `a6b21efd55cdba93c857a8b986c6aa67f41a33d644c5aa05f06a0a0ec4bcb73d`.

```text
> react-example@0.0.0 lint
> tsc --noEmit
```

## `npm run lint:eslint`

Code de sortie : **0**. Log : `lint-eslint.log`.

SHA-256 : `0063d9200852cdda613509449a47f4a30880233b439f3ab60a3915e24ef513f4`.

```text
✖ 48 problems (0 errors, 48 warnings)
```

## `npm run format:check`

Code de sortie : **0**. Log : `format-check.log`.

SHA-256 : `bf17f6b1fbefad6c43acddb65ebca906ba04d59ef453da418e1b9ca219f9a3e9`.

```text
> react-example@0.0.0 format:check
> prettier --check .

Checking formatting...
All matched files use Prettier code style!
```

## `npm run build`

Code de sortie : **0**. Log : `build.log`.

SHA-256 : `dd9b33d434bfd9b87e2410d524549d5601d2196711d65e92d79da7fcdbf9e183`.

```text
> react-example@0.0.0 build
> vite build

vite v8.3.4 building client environment for production...
transforming...
✓ 2769 modules transformed.
rendering chunks...
computing gzip size...
dist/index.html                             1.37 kB │ gzip:   0.53 kB
dist/assets/index-DTWgn-Pc.css            102.37 kB │ gzip:  15.93 kB
dist/assets/rolldown-runtime-B0Z9INg1.js    0.90 kB │ gzip:   0.51 kB
dist/assets/vendor-icons-B-g4UwOb.js       20.62 kB │ gzip:   6.99 kB
dist/assets/vendor-motion-gGZcMBMQ.js     124.88 kB │ gzip:  40.70 kB
dist/assets/vendor-react-BCKGYVch.js      218.76 kB │ gzip:  68.27 kB
dist/assets/index-C4oUnPYL.js             366.05 kB │ gzip:  72.73 kB
dist/assets/vendor-lY7hst1K.js            576.73 kB │ gzip: 169.06 kB

[plugin builtin:vite-reporter]
(!) Some chunks are larger than 500 kB after minification. Consider:
- Using dynamic import() to code-split the application
- Use build.rolldownOptions.output.codeSplitting to improve chunking: https://rolldown.rs/reference/OutputOptions.codeSplitting
- Adjust chunk size limit for this warning via build.chunkSizeWarningLimit.
✓ built in 1.51s
```

## Backend local : échec de lancement, pas échec de test métier

Commande exécutée depuis `backend` :

```sh
go test -race -cover ./...
```

Code de sortie **127** :

```text
/bin/bash: line 1: go: command not found
```

`go version` a également échoué. `command -v psql` et `command -v docker` n'ont trouvé aucun exécutable.
Aucun test Go n'a donc démarré localement. Aucune migration exécutée et aucun résultat de test SQL local annoncé.

## Vérification Git et CI existante

Commandes exécutées :

```sh
git status --short
git branch --show-current
git log -3 --oneline
git ls-remote --heads origin main
git fetch origin main
git diff --stat HEAD origin/main
git rev-list --left-right --count HEAD...origin/main
gh pr list --state open --json number,title,headRefName,baseRefName
gh run list --branch main --limit 3 --json databaseId,headSha,status,conclusion,url
gh run view 38216977355 --json headSha,conclusion,event,jobs,url
```

Résultats pertinents :

```text
branche : arena/4127deca-presspro
HEAD : 7e432a4f3661df8bb26a08371e8f9512fd688079
main : ff15988da596fbb9866e3a61619e98f9ab812dd5
diff HEAD origin/main : vide
rev-list HEAD...origin/main : 0 1
PR ouvertes : []
run 38216977355 : event=push, status=completed, conclusion=success
Backend Go (tests + PostgreSQL) : success
Frontend React (types + build) : success
```

[Run existant vérifié sur GitHub](https://github.com/ErwinsS225/PRESSPRO/actions/runs/38216977355).
Les étapes backend `go vet`, tests avec détection de courses, migrations up et intégration PostgreSQL sont indiquées en succès.
Ce run précède cet audit : il ne remplace pas les tests de régression à écrire et ne constitue pas une nouvelle CI lancée par cette phase.

## Contrôle documentaire final

`git diff --check` et un contrôle Python des liens relatifs, cases de la phase 0 et cases des phases 1–9 ont été exécutés après rédaction. Les résultats sont consignés dans le journal de `MiseJourClient.md`.
Le premier contrôle Python a détecté des espaces finaux dans les extraits de logs de ce relevé. Après suppression de ces espaces, le second contrôle a réussi pour les trois documents : liens locaux valides, 44 tâches d’audit cochées, aucune tâche cochée des phases 1–9. `git diff --check` a également réussi ; les nouveaux fichiers non suivis ont été vérifiés séparément par Python.

Les fichiers Markdown ne sont pas validés par `npm run format:check` (exclusion `*.md` dans `.prettierignore`).

## Non exécuté

- Build/vet Go local et tests d'intégration locaux.
- Migrations up/down sur base jetable et reproduction du risque d'initialisation Compose.
- Parcours réel client → dépôt → encaissement sur API/PostgreSQL.
- Test navigateur/PDF d'impression 80 mm ou test matériel WebUSB/imprimante.

La suite frontend réussie ne prouve pas ces opérations.
