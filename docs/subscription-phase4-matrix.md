# PRESSPRO — Matrice des routes et de l'isolation multitenant (phase 4)

Document de référence de la phase 4 (PLAN §3.5 à §3.7). Le tableau ci-dessous est **généré à partir**
de `backend/internal/http/route_matrix_test.go` : `TestMatrice_CouvertureDesRoutes` échoue si une route
est ajoutée ou retirée sans mise à jour de la matrice, dans les deux sens.

## Légende

- **Public** : aucune authentification requise.
- **Métier** : la route est soumise au contrôle d'abonnement (`RequireActiveSubscription`) : abonnement
  suspendu ou inexistant ⇒ 402.
- **Rôles** : rôles autorisés par `RequireRole`. Vide = tout membre actif de l'établissement.
- **Fonctionnalité** : droit du plan vérifié par la route (`RequireFeature`) ⇒ 402 si absent.
- **Quota** : plafond de la formule vérifié à la création (`RequireFeatureQuota` ou garde de commandes).
- **Contrôle complémentaire** : portée établissement, décision ouverte, etc.

Sans jeton, toute route non publique répond **401** (testé sur les 42 routes non publiques).

## Matrice

| Méthode | Route | Public | Métier | Rôles | Fonctionnalité | Quota | Contrôle complémentaire |
|---|---|---|---|---|---|---|---|
| GET | `/health` | oui |  |  |  |  |  |
| GET | `/ready` | oui |  |  |  |  |  |
| GET | `/api/v1/system/info` | oui |  |  |  |  |  |
| POST | `/api/v1/auth/login` | oui |  |  |  |  |  |
| POST | `/api/v1/auth/register-tenant` | oui |  |  |  |  |  |
| GET | `/api/v1/public/verify/{tenantID}/{orderID}` | oui |  |  |  |  | projection publique du ticket (voir décision D7) |
| GET | `/api/v1/auth/me` |  |  | tout membre |  |  |  |
| GET | `/api/v1/tenant/current` |  |  | tout membre |  |  |  |
| GET | `/api/v1/me/entitlements` |  |  | tout membre |  |  |  |
| GET | `/api/v1/services/` |  | oui | tout membre |  |  |  |
| GET | `/api/v1/services/{id}` |  | oui | tout membre |  |  |  |
| POST | `/api/v1/services/` |  | oui | propriétaire, gérant, superadmin |  |  |  |
| PUT | `/api/v1/services/{id}` |  | oui | propriétaire, gérant, superadmin |  |  |  |
| DELETE | `/api/v1/services/{id}` |  | oui | propriétaire, gérant, superadmin |  |  |  |
| GET | `/api/v1/orders/` |  | oui | tout membre |  |  | portée établissement |
| POST | `/api/v1/orders/` |  | oui | tout membre |  | orders.monthly.max (fenêtre 30 j, dans la transaction) | portée établissement |
| GET | `/api/v1/orders/{id}` |  | oui | tout membre |  |  | portée établissement |
| PATCH | `/api/v1/orders/{id}/status` |  | oui | tout membre |  |  | portée établissement |
| POST | `/api/v1/orders/{id}/payments` |  | oui | tout membre |  |  | portée établissement |
| GET | `/api/v1/orders/{id}/ticket` |  | oui | tout membre |  |  | portée établissement |
| GET | `/api/v1/customers/` |  | oui | tout membre |  |  |  |
| GET | `/api/v1/customers/{id}` |  | oui | tout membre |  |  |  |
| POST | `/api/v1/customers/` |  | oui | tout membre |  |  |  |
| PUT | `/api/v1/customers/{id}` |  | oui | propriétaire, gérant, superadmin |  |  |  |
| DELETE | `/api/v1/customers/{id}` |  | oui | propriétaire, gérant, superadmin |  |  |  |
| GET | `/api/v1/price-lists/` |  | oui | tout membre |  |  |  |
| GET | `/api/v1/price-lists/{id}` |  | oui | tout membre |  |  |  |
| POST | `/api/v1/price-lists/` |  | oui | propriétaire, gérant, superadmin |  | price_lists (plafond de la formule) |  |
| POST | `/api/v1/price-lists/{id}/versions` |  | oui | propriétaire, gérant, superadmin | FeaturePriceLists |  |  |
| GET | `/api/v1/users` |  | oui | propriétaire, gérant, superadmin |  |  |  |
| POST | `/api/v1/users` |  | oui | propriétaire, gérant, superadmin |  | users.max (plafond de la formule) |  |
| PATCH | `/api/v1/users/{id}/status` |  | oui | propriétaire, gérant, superadmin |  |  |  |
| GET | `/api/v1/users/{id}/branches` |  | oui | propriétaire, gérant, superadmin |  |  |  |
| PUT | `/api/v1/users/{id}/branches` |  | oui | propriétaire, gérant, superadmin |  |  |  |
| GET | `/api/v1/branches/` |  | oui | tout membre |  |  |  |
| POST | `/api/v1/branches/` |  | oui | propriétaire, gérant, superadmin |  | multi_site (plafond de la formule) |  |
| PATCH | `/api/v1/branches/{id}` |  | oui | propriétaire, gérant, superadmin |  |  |  |
| GET | `/api/v1/expenses/` |  | oui | tout membre |  |  |  |
| POST | `/api/v1/expenses/` |  | oui | propriétaire, gérant, superadmin | FeatureExpensesAdvanced |  |  |
| DELETE | `/api/v1/expenses/{id}` |  | oui | propriétaire, gérant, superadmin | FeatureExpensesAdvanced |  | suppression bloquée par la fonctionnalité (voir décision D6) |
| GET | `/api/v1/notifications/` |  | oui | tout membre |  |  |  |
| POST | `/api/v1/notifications/` |  | oui | tout membre | FeatureNotificationsAdvanced |  |  |
| GET | `/api/v1/reports/summary` |  | oui | propriétaire, gérant, superadmin | FeatureReportsAdvanced |  |  |
| GET | `/api/v1/reports/export` |  | oui | propriétaire, gérant, superadmin | FeatureReportsAdvanced |  |  |
| GET | `/api/v1/admin/stats` |  |  | superadmin |  |  |  |
| GET | `/api/v1/admin/tenants` |  |  | superadmin |  |  |  |
| PATCH | `/api/v1/admin/tenants/{id}/status` |  |  | superadmin |  |  |  |
| DELETE | `/api/v1/admin/tenants/{id}` |  |  | superadmin |  |  |  |

## Contrôles négatifs couverts

| Test | Ce qui est vérifié | Modes |
|---|---|---|
| `TestMatrice_SansJetonRefuse` | 401 sur chaque route non publique sans jeton | mémoire, PostgreSQL |
| `TestMatrice_RolesRefusent` | 403 pour caissier et opérateur sur chaque route à rôles restreints ; 403 pour le propriétaire sur `/admin/*` | mémoire, PostgreSQL |
| `TestMatrice_SuperAdminSansEtablissement` | 403 sur chaque route métier pour le superadmin sans `X-Tenant-ID` | mémoire, PostgreSQL |
| `TestIsolation_RessourcesDUnAutreEtablissement` | 404 pour lire, modifier, supprimer une ressource d'un autre établissement (17 attaques en PostgreSQL, 12 en mémoire) ; listes sans identifiant étranger ; A inchangé après les tentatives | mémoire (sans clients ni affectations de sites), PostgreSQL (complet) |
| `TestIsolation_XTenantIDUsurpe` | 403 sur chaque route métier avec `X-Tenant-ID` d'un autre établissement | mémoire, PostgreSQL |
| `TestSuspension_GeleLesModulesMetier` | 402 sur les modules métier suspendus ; 200 sur la situation du compte ; reprise après réactivation | PostgreSQL |
| `TestPlan_FormuleFree_RefuseLesModulesPayants` | 402 sur dépenses, notifications, rapports en FREE ; plafond de grilles (2e grille refusée) | mémoire (sans plafond), PostgreSQL |
| `TestPlan_Downgrade_ConserveLesDonnees` | après rétrogradation : données historiques consultables (grilles, dépenses) ; nouvelle création au-delà du plafond refusée | mémoire (sans plafond), PostgreSQL |
| `TestAdmin_RoutesPlateforme` | annuaire et statistiques réservés à l'éditeur ; identifiant mal formé ⇒ 400 ; établissement inconnu ⇒ 404 | mémoire, PostgreSQL |

## Limites du mode mémoire (mode dégradé)

Le mode mémoire n'est actif que lorsque PostgreSQL est indisponible au démarrage. Les écarts ci-dessous
sont **connus et mesurés** : les tests sautent le contrôle concerné et l'indiquent dans leur journal.

| Limite | Effet | Traitement |
|---|---|---|
| Clients non persistés | `POST /customers/` répond 201 sans identifiant ; rien n'est relisible | contrôles clients exécutés en PostgreSQL uniquement |
| Compteurs de plafonds non mesurés (grilles, équipe, établissements) | un plafond FREE n'est pas appliqué en mode dégradé | contrôles de plafond exécutés en PostgreSQL uniquement ; **risque R-P4-3** |
| Affectations de sites sans contrôle du compte cible | `GET`/`PUT /users/{id}/branches` accepte un compte d'un autre établissement | contrôles exécutés en PostgreSQL uniquement ; **risque R-P4-2** |
| Identifiants d'établissement non-UUID | la route `/admin/tenants/{id}/…` refuse (400) les établissements créés en mémoire | suspension testée en PostgreSQL uniquement |

## Exécution

Mode mémoire (aucune base requise) :

```
go test -race -count=1 -run 'TestMatrice_|TestIsolation_|TestSuspension_|TestPlan_|TestAdmin_' ./internal/http/...
```

Mode PostgreSQL (sous-tests « PostgreSQL », tag `integration`) :

```
TEST_DATABASE_URL="postgres://…" go test -race -tags=integration -count=1 \
  -run 'TestMatrice_|TestIsolation_|TestSuspension_|TestPlan_|TestAdmin_' ./internal/http/...
```

Sans `TEST_DATABASE_URL`, le sous-test PostgreSQL est **ignoré** (et signalé comme tel, jamais compté
comme réussi). La CI exécute ce mode dans l'étape « Tests d'intégration (PostgreSQL réel) ».
