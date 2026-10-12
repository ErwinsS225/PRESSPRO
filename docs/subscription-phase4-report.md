# PRESSPRO — Rapport de la phase 4 (couverture des routes, matrice d'isolation multitenant)

Suite de `docs/subscription-phase3-report.md`. Périmètre : matrice de toutes les routes enregistrées,
tests négatifs d'isolation et de rôles, contrôle de suspension, de formule et de rétrogradation,
routes d'administration. Aucun tarif modifié, aucun paiement ou webhook simulé, aucune migration.
La matrice détaillée est dans `docs/subscription-phase4-matrix.md`.

---

## Phase 4 — Couverture des routes et matrice d'isolation multitenant

**État :** terminé dans le périmètre de la phase 4 (code, tests, CI verte). Les tests ont été
exécutés en mode mémoire **et** sur PostgreSQL réel. Les limites du mode mémoire sont listées
ci-dessous et ne sont pas présentées comme couvertes. La phase 4 modifie le comportement sur deux
points (401 → 403, validation des identifiants d'administration) : voir Décisions.

**Constat (mesuré par la première exécution de la matrice, avant correctif) :**

| Constat | Preuve | Cause | Traitement |
|---|---|---|---|
| Superadmin sans établissement reçoit **401** au lieu de 403 sur 21 routes métier (`services`, `orders`, `customers`, `users` dont la garde est dans `auth/handler.go`, `branches`, `expenses`, `notifications`, `reports`) | `TestMatrice_SuperAdminSansEtablissement` | Les handlers répondent 401 quand le contexte d'établissement est vide ; le middleware et les tests de rôles disent 403 | Corrigé : 403 (voir Décisions D-P4-1) |
| Identifiant d'établissement mal formé sur `/admin/tenants/{id}/…` : 404 en mémoire, transmis tel quel au service | `TestAdmin_RoutesPlateforme` (mémoire) | Aucune validation de format avant le service | Corrigé : 400 (D-P4-2). Comportement avant correctif **en base non mesuré** : le premier passage ne couvrait que le mémoire |
| Mode mémoire : `GET` et `PUT /users/{id}/branches` acceptent un compte d'un autre établissement | `TestIsolation_RessourcesDUnAutreEtablissement` (mémoire) : 200 puis 400 au lieu de 404 | Le service des sites ne connaît pas les comptes ; la vérification d'appartenance est fail-open en mode dégradé | **Non corrigé.** Contrôle sauté en mémoire, en base il passe. Risque R-P4-2 |
| Mode mémoire : `POST /customers/` répond 201 sans identifiant, rien n'est relisible | `TestIsolation_…` (mémoire) : « identifiant absent de la réponse » | Le service client mémoire ne conserve pas les clients | **Non corrigé.** Contrôle client exécuté en base uniquement. Risque R-P4-1 |
| Mode mémoire : les plafonds de grilles, d'équipe et d'établissements ne sont pas appliqués | `TestPlan_FormuleFree_…` (mémoire) : 2e grille acceptée (201) en FREE | Les compteurs mémoire renvoient 0 pour ces ressources | **Non corrigé.** Contrôle de plafond exécuté en base uniquement. Risque R-P4-3 |
| Expense : catégorie de test invalide (`electricite`) | `TestPlan_Downgrade_…` : 400 | Erreur de test (le référentiel contient `energie_eau`) | Test corrigé |
| Commande de test avec le même téléphone qu'un client créé ensuite | `TestIsolation_…` (base) : 409 | Erreur de test : la création de commande crée le client | Test corrigé (téléphone distinct) |

**Résultat sur l'isolation en base :** aucun défaut trouvé. Les 17 tentatives d'accès par identifiant
d'un autre établissement (lecture, modification, suppression, clients, grilles, commandes, paiements,
tickets, sites, comptes, affectations, dépenses) répondent 404 ; les listes ne contiennent aucun
identifiant étranger ; les données de l'établissement cible sont inchangées après les tentatives.

**Décisions d'implémentation :**

- **D-P4-1 — Contexte d'établissement absent avec un jeton valide : 403, plus 401.** Appliqué dans
  `catalog`, `customer`, `order`, `expense`, `notification`, `report`, `branch` et les trois gardes de
  `auth/handler.go` (liste des utilisateurs, création, statut). La branche « jeton absent » (401) reste
  assurée par `AuthenticatorMiddleware` ; la branche `claims == nil` dans ces handlers n'est pas
  atteignable derrière ce middleware. Effet pour le frontend : un superadmin sans établissement n'est
  plus déconnecté par un 401 ; il reçoit un 403 explicite (à vérifier en phase 5).
- **D-P4-2 — Routes `/admin/tenants/{id}/…` : identifiant mal formé rejeté en 400.** Le format est
  vérifié par `looksLikeUUID` (même garde que le middleware d'accès). Effet connu en mode mémoire : les
  établissements créés en mémoire ont des identifiants non-UUID et sont donc refusés (400) par ces
  routes. Accepté : le mode mémoire n'est actif que sans base, et ses données ne survivent pas à un
  redémarrage. Les établissements réels (PostgreSQL) sont des UUID.
- **D-P4-3 — Le contrôle de suspension est exécuté en PostgreSQL uniquement.** Il dépend de
  l'identifiant UUID de D-P4-2. Le sous-test mémoire est sauté et le journal l'indique.
- **D-P4-4 — La matrice s'exécute dans deux modes.** `pourChaqueMode` lance chaque test en « mémoire »
  et en « PostgreSQL » (fichier `route_matrix_integration_test.go`, tag `integration`). Les
  établissements de test sont créés par inscription, non par les comptes de démonstration ; ils sont
  supprimés au nettoyage (`DeleteTenant`). Les contrôles impossibles en mémoire sont sautés
  explicitement, jamais validés à tort.
- **D-P4-5 — CI.** L'étape « Tests d'intégration (PostgreSQL réel) » couvre désormais
  `./internal/http/...`. Coût : le paquet `http` prend environ 2 min 30 sous `-race`, surtout à cause
  du hachage des mots de passe lors des connexions de test.

**Fichiers modifiés :**
- `backend/internal/http/route_matrix_test.go` — nouveau : matrice des 48 routes, helpers, 9 fonctions de test, exécutées dans chaque mode.
- `backend/internal/http/route_matrix_integration_test.go` — nouveau (tag `integration`) : environnement PostgreSQL câblé comme `cmd/api`, formule fixée par SQL, nettoyage.
- `backend/internal/http/admin_handler.go` — validation de l'identifiant (`looksLikeUUID`) sur `PATCH` et `DELETE` `/admin/tenants/{id}`.
- `backend/internal/auth/handler.go` — 403 pour contexte d'établissement absent (3 gardes).
- `backend/internal/{catalog,customer,order,expense,notification,report,branch}/handler.go` — 401 → 403 pour contexte d'établissement absent.
- `.github/workflows/ci.yml` — `./internal/http/...` ajouté à l'étape d'intégration.
- `docs/subscription-phase4-matrix.md` — nouveau : matrice générée depuis les tests, limites du mode mémoire, commandes d'exécution.
- `docs/subscription-phase4-report.md` — ce rapport.

**Tests :**
- `gofmt -l .` : vide. `go vet ./...` et `go vet -tags=integration ./...` : sans erreur.
- `go test -race -count=1 ./...` (mémoire) : **ok** pour tous les paquets.
- `go test -race -tags=integration -count=1 ./...` (PostgreSQL réel, migrations 000001–000016) : **ok** pour tous les paquets.
- Stabilité : `go test -race -tags=integration -count=3` sur les tests de la matrice : **ok**.
- Mutation (sensibilité du test) : garde `X-Tenant-ID` désactivée dans `auth_middleware.go` ⇒
  `TestIsolation_XTenantIDUsurpe` échoue (200, 400 ou 404 là où 403 est attendu, sur les routes métier),
  preuve que le test détecte la fuite. Fichier restauré, diff vide.
- Décompte : 48 routes dans la matrice, égal aux routes enregistrées (`TestMatrice_CouvertureDesRoutes`).
  Sur les 42 routes non publiques, 401 sans jeton (mémoire et base).
- Non couvert par ces tests : interface frontend, performances, comportement sous charge.

**Sécurité et multitenant :**

| Classe d'attaque | Contrôle | Résultat (base) | Résultat (mémoire) |
|---|---|---|---|
| Accès anonyme aux routes protégées | 401 | OK (42 routes) | OK |
| Caissier ou opérateur sur une route à rôles | 403 | OK | OK |
| Propriétaire sur `/admin/*` | 403 | OK | OK |
| Superadmin sans établissement sur une route métier | 403 | OK | OK |
| `X-Tenant-ID` d'un autre établissement | 403 | OK (toutes les routes métier) | OK |
| Lecture, modification, suppression par identifiant étranger | 404 | OK (17 tentatives) | OK sauf clients et affectations (non persistés en mémoire) |
| Affectation d'un site à un compte étranger | 404 | OK | **Défaut connu (R-P4-2)** |
| Abonnement suspendu | 402 sur modules métier, 200 sur la situation | OK | Non exécuté (D-P4-3) |
| Formule FREE : modules payants | 402 | OK | OK |
| Rétrogradation : données conservées | consultation 200 | OK (grilles, dépenses) | OK |
| Rétrogradation : plafond des créations | 402 | OK | **Non mesuré (R-P4-3)** |

**Risques ouverts (non traités dans cette phase) :**

- **R-P4-1 (mémoire) — clients non persistés.** Degré : faible ; mode dégradé seulement. Chemin : `customer/service.go`.
- **R-P4-2 (mémoire) — affectations de sites sans contrôle du compte cible.** Degré : moyen en mode dégradé (lecture d'informations de personnel d'un autre établissement). Correctif possible : vérifier l'appartenance via le service d'authentification avant l'écriture.
- **R-P4-3 (mémoire) — plafonds non appliqués.** Degré : moyen en mode dégradé (un FREE peut dépasser ses plafonds tant que la base est indisponible). Correctif possible : compteurs mémoire sur les services concernés.
- **R-P4-4 — `/admin/*` protégé seulement par `RequireRole(superadmin)`** à l'intérieur du groupe protégé. Le PLAN (§3.7) demande une protection distincte de la plateforme. Non traité ici : décision à prendre (voir Phase suivante).
- **R-P4-5 — `GET /public/verify/{tenantID}/{orderID}`** renvoie `customer_name` sans jeton, alors que le commentaire du code annonce « sans PII sensible ». Décision (c) en attente.
- **R-P4-6 — `POST /users` accepte le rôle propriétaire pour un gérant** (RBAC : propriétaire, gérant, superadmin ; rôles autorisés : propriétaire, gérant, caissier, opérateur). Décision (a) en attente.
- **Risque hérité de la phase 0 : `ForcePlanForTest`** est exporté depuis du code non-test (`billing/testing_hook.go`). Non modifié ici ; à déplacer dans un fichier de test ou à retirer quand les tests utiliseront la base.

**Décisions du propriétaire (réponses reçues après ce rapport ; toutes au choix recommandé) :**

| Sujet | Décision | Mise en œuvre |
|---|---|---|
| D6 — suppression d'une dépense après rétrogradation | **Blocage conservé** (`DELETE /expenses/{id}` = 402 en FREE). Règle à inscrire dans le PLAN : la rétrogradation ne supprime ni ne rend modifiables les données ; elle bloque les créations au-delà du plafond et les fonctionnalités payantes. | Aucun changement de code. Règle à reprendre dans la mise à jour du PLAN (phase 8). |
| R-P4-2 et R-P4-3 — mode dégradé | **Correction en phase 5** : vérification d'appartenance du compte cible avant écriture des affectations ; compteurs mémoire des plafonds. | Phase 5. Tests mémoire à rendre exécutables (sauts supprimés). |
| Vitrine publique (`customer_name`) | **Retirer `customer_name`** de la réponse publique (ne garder que statut, total, dates et lignes sans PII). | Phase 5, avec vérification du frontend qui affiche le ticket public. |
| Rôle gérant | **Réservé au propriétaire** : création d'un compte propriétaire et verrouillage d'un compte propriétaire. Le gérant garde caissiers et opérateurs. | Phase 5 : restriction RBAC dans `POST /users` et `PATCH /users/{id}/status`, avec tests négatifs de rôles. |

**Décisions à obtenir du propriétaire (historique, remplacées par les réponses ci-dessus) :**

1. **D6 — suppression d'une dépense après rétrogradation.** Observé : `DELETE /expenses/{id}` répond **402** pour un établissement FREE. Le PLAN (§3.3) interdit la suppression de données et ne bloque que les nouvelles créations au-delà du plafond : ce blocage va donc au-delà du PLAN. Option A : autoriser la suppression. Option B : conserver le blocage (données protégées). Recommandation : B, avec une règle explicite dans le PLAN.
2. **Risques R-P4-2 et R-P4-3 (mode dégradé).** Option A : corriger en phase 5 (petit correctif, sans changement pour la production avec base). Option B : accepter et documenter.
3. **(c) Vitrine publique de vérification.** Garder `customer_name` et `tenant_id` dans la réponse publique, ou les retirer ?
4. **(a) et (b) Rôle gérant.** Un gérant peut-il créer un compte propriétaire et verrouiller un compte propriétaire ? Recommandation : non pour les deux ; réservé au propriétaire.

**GitHub :**
- Commit de la phase 4 : `01e2bb9` (« Phase 4 : matrice des routes et isolation multitenant »), poussé sur `arena/a06916de-presspro`.
- CI sur `01e2bb9` (run `38191306427`) : **succès** pour les deux jobs (« Frontend React (types + build) » et « Backend Go (tests + PostgreSQL) »).
- Aucune fusion dans `main`. Aucune pull request ouverte à ce stade. Pas de déploiement.

**Phase suivante :** phase 5 — API et frontend reflétant les droits effectifs du backend
(`/me/entitlements`, libellés de quota, ne plus coder de tarifs ou de limites dans le frontend,
affichage de `BillingView.tsx` et de `permissions.ts`). Elle dépend des décisions 1 à 4 ci-dessus et
d'un accord explicite pour toute modification de comportement.
