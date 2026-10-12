# Mise à jour PRESSPRO — Gestion des clients, du linge et des dépôts

## Objet et mode d'emploi

Ce document transforme les instructions du chantier en todolist de référence. Il ne constitue ni un audit déjà réalisé, ni une preuve de fonctionnement. Les cases sont cochées uniquement avec des éléments vérifiables. Une case d’audit cochée signifie que l’examen est réalisé, pas que la fonctionnalité auditée est conforme.

- Dépôt : https://github.com/ErwinsS225/PRESSPRO
- Branche cible d'intégration : `main`, uniquement après revue d'une Pull Request et autorisation de fusion.
- Branche de travail de cette session : `arena/4127deca-presspro`.
- Produit : ERP SaaS multitenant pour les pressings, notamment en Côte d'Ivoire.
- Objectif : compléter l'existant sans réécriture arbitraire ni système parallèle.
- Compétences attendues : React, TypeScript, Go, API HTTP, PostgreSQL, migrations, transactions, sécurité multitenant, clients, commandes, textiles, paiements, tests, CI et impression thermique 80 mm.

**Source incomplète :** la section 12 du prompt fourni s'interrompt après « mais les consultations et ». Ne pas inventer sa suite. Faire préciser la règle manquante avant de prendre une décision qui en dépend.

## Règles transversales et points de contrôle

- [ ] Inspecter avant de modifier ; rechercher les implémentations dans le frontend, le backend, les routes, les migrations et les tests avant de déclarer une fonction absente.
- [ ] Améliorer l'application existante, réutiliser les composants adaptés et préserver les fonctionnalités en place.
- [ ] Expliquer les décisions techniques et métier, ainsi que les dépendances entre modules.
- [ ] Découper le chantier en phases ; ne pas tout coder en une seule demande.
- [ ] Terminer et présenter l'audit de phase 0 avant toute modification du code métier ou réécriture.
- [ ] Vérifier backend, paiements et isolation multitenant avant de considérer l'interface comme terminée.
- [ ] Ne prendre aucune décision financière implicite : documenter et faire valider les règles non établies.
- [ ] Ne pas changer arbitrairement les formats de données, les prix, les plans ni les droits existants.
- [ ] Ne jamais confondre impression et enregistrement : ticket officiel uniquement à partir d'un dépôt persisté ; tout aperçu non enregistré doit être clairement provisoire.
- [ ] Ne jamais masquer un échec serveur par un succès local simulé.
- [ ] Ne pas ajouter de secrets au code, aux logs ou aux réponses API.
- [ ] Fournir à chaque phase les fichiers modifiés, commandes exécutées, résultats, échecs et limites de validation.
- [ ] Ne jamais annoncer un test réussi sans l'avoir exécuté et sans résultat vérifiable.
- [ ] Soumettre les changements à une Pull Request ; ne pas fusionner automatiquement dans `main`.

## Phase 0 — Audit factuel obligatoire, sans modification métier

**État au 12 octobre 2026 : audit statique et baseline frontend réalisés.** [Rapport factuel](docs/depot-phase0-audit.md) et [preuves de validation](docs/depot-phase0-validation.md). Tests Go locaux bloqués (`go` absent), PostgreSQL et impression non exécutés ; aucune conformité métier globale déclarée. Les cases ci-dessous portent sur l’examen et son compte rendu.

### 0.1 — Accès, état du dépôt et conventions

- [x] Vérifier l'accès effectif au dépôt GitHub.
- [x] Identifier la branche courante, la branche principale et les modifications non validées.
- [x] Examiner l'arborescence réelle du projet.
- [x] Rechercher et lire les `AGENTS.md`, README, documents d'architecture et conventions de développement existants.
- [x] Identifier les environnements, dépendances, scripts de validation et prérequis de tests.

### 0.2 — Frontend et parcours actuel

- [x] Examiner les composants de commandes, clients, paiements et tickets.
- [x] Examiner `src/components/NewOrderModal.tsx` ou son équivalent réel.
- [x] Examiner `src/components/ThermalTicketModal.tsx` ou son équivalent réel.
- [x] Examiner `src/lib/ordersApi.ts` et `src/lib/customersApi.ts` ou leurs équivalents.
- [x] Examiner `src/App.tsx`, notamment la soumission, la gestion des erreurs et la mise à jour de la liste des commandes.
- [x] Retracer le fonctionnement actuel de création d'une commande et de son éventuel paiement initial.
- [x] Vérifier si un acompte de 50 % est imposé et identifier son origine et sa justification éventuelle.
- [x] Identifier les données réelles, les estimations, les données simulées et les valeurs générées dans le navigateur.

### 0.3 — Backend, persistance et base de données

- [x] Examiner les routes API, handlers, services, modèles et couches de persistance.
- [x] Examiner `backend/internal/order/model.go`.
- [x] Examiner `backend/internal/order/service.go`.
- [x] Examiner `backend/internal/order/handler.go`.
- [x] Examiner `backend/internal/order/persist.go`.
- [x] Examiner `backend/internal/http/router.go`.
- [x] Rechercher les chemins réels si les fichiers ont été déplacés ; ne pas limiter l'audit aux points de départ ci-dessus.
- [x] Examiner les modules clients, catalogue, prestations, tarifs, paiements, caisse, statuts et configuration du pressing.
- [x] Examiner `backend/migrations/`, les contraintes SQL, index, clés étrangères et transactions.
- [x] Vérifier les conventions monétaires, limites numériques, arrondis et formats d'API existants.
- [x] Vérifier la numérotation des dépôts, sa persistance et son comportement concurrent.
- [x] Vérifier les dates, sources temporelles et fuseaux horaires existants.
- [x] Retracer le tenant authentifié et les autorisations sur chaque opération concernée.

### 0.4 — Abonnements et validation existante

- [x] Examiner `backend/internal/billing/` et les modules d'entitlements associés.
- [x] Examiner les plans, fonctionnalités autorisées, middlewares, quotas et matrice de droits réelle.
- [x] Examiner le cycle d'essai, l'expiration, le renouvellement et la rétrogradation.
- [x] Examiner les migrations et tests d'abonnements.
- [x] Examiner les tests existants, scripts de validation et configuration CI.
- [x] Exécuter les validations initiales disponibles ; consigner commandes, résultats et blocages sans les extrapoler.

### 0.5 — Rapport obligatoire de l'audit

- [x] Décrire l'architecture réellement constatée.
- [x] Décrire les flux frontend → API → base de données et les réponses canoniques utilisées par l'interface.
- [x] Lister les fonctionnalités déjà implémentées et réutilisables.
- [x] Lister les fonctionnalités partiellement implémentées.
- [x] Lister les fonctionnalités manquantes après recherche effective.
- [x] Décrire les risques de sécurité, d'intégrité des données et de régression.
- [x] Identifier les dépendances entre modules.
- [x] Lister les fichiers à modifier et les migrations éventuellement nécessaires.
- [x] Lister les tests à ajouter.
- [x] Associer chaque constat important à un fichier et à un symbole, une fonction ou une route.
- [x] Fournir un plan d'exécution ordonné et les décisions métier restant à faire valider.
- [x] Présenter ce rapport avant d'entamer l'implémentation.

## Phase 1 — Cadrage métier, architecture et migrations

**Décisions confirmées le 12 octobre 2026 :** téléphone obligatoire, numéro partagé autorisé sans fusion, paiement intégral au dépôt, calcul de monnaie à rendre (excédent non comptabilisé comme paiement), restitution uniquement à solde nul. Voir [le contrat de phase 1](docs/depot-phase1-decisions.md). Les règles financières sont approuvées mais restent à implémenter ; elles remplacent les possibilités conditionnelles du cahier initial pour les nouveaux dépôts.

**Lot clients :** [rapport d’implémentation et preuves](docs/depot-lot1-clients.md). Go/PostgreSQL locaux désormais disponibles pour les validations ; le blocage décrit en phase 0 est levé. Les formulaires complets, la caisse et les tickets ne sont pas terminés.

- [x] Confirmer les règles réelles concernant le téléphone facultatif ou obligatoire.
- [ ] Confirmer la représentation monétaire existante et les limites de quantité et de prix.
- [x] Confirmer les moyens de paiement supportés par le backend ou la configuration.
- [x] Confirmer les règles de paiement initial nul, partiel et intégral.
- [x] Définir explicitement le traitement d'un paiement supérieur au solde, sans autorisation implicite de trop-perçu.
- [x] Vérifier si une règle d'acompte de 50 % est explicitement configurée et approuvée ; sinon prévoir sa suppression.
- [x] Vérifier et faire valider toute règle imposant ou non le règlement du solde avant restitution.
- [ ] Confirmer les statuts opérationnels existants et les transitions autorisées.
- [ ] Définir la politique temporelle : source fiable, stockage et affichage des dates et heures.
- [ ] Confirmer la règle de date prévue de restitution ; ne pas inventer une échéance.
- [x] Évaluer l'atomicité client + dépôt + paiement initial, adaptée à l'architecture existante.
- [ ] Évaluer une stratégie d'idempotence pour les doubles soumissions et résultats réseau incertains.
- [ ] Identifier les changements de schéma strictement nécessaires, compatibles avec les données existantes.
- [ ] Prévoir contraintes SQL, index et clés étrangères assurant l'intégrité et l'isolation des relations.
- [ ] Documenter la migration des données existantes et les risques de déploiement, sans changement arbitraire de format.
- [ ] Faire préciser la fin manquante de la consigne d'abonnement si elle conditionne l'implémentation.

## Phase 2 — Backend des clients et des dépôts

### 2.1 — Clients

- [ ] Permettre la recherche d'un client par nom, prénom ou téléphone.
- [ ] Permettre la sélection d'un client existant du tenant courant.
- [ ] Permettre la création d'un client depuis le parcours de dépôt.
- [ ] Prévoir une saisie séparée du nom, du ou des prénoms et du téléphone, compatible avec les données existantes.
- [ ] Vérifier de bout en bout le téléphone obligatoire (décision D1) ; aucun téléphone fictif ni client sans téléphone nouvellement créé.
- [ ] Autoriser la modification des coordonnées uniquement selon les permissions existantes.
- [ ] Détecter les correspondances possibles et permettre leur vérification sans fusion automatique.
- [ ] Ne jamais fusionner deux clients sur la seule base d'un nom ou d'un téléphone.
- [ ] Favoriser la sélection d'un client existant pour limiter les doublons.
- [ ] Évaluer les créations partielles si client et dépôt sont deux opérations séparées.
- [ ] Privilégier une opération backend atomique lorsque compatible avec l'architecture ; documenter les garanties et limites retenues.

### 2.2 — Articles, prestations et tarifs

- [ ] Autoriser plusieurs lignes d'articles par dépôt.
- [ ] Gérer, selon pertinence, catégorie/type, désignation précise et prestation demandée.
- [ ] Gérer quantité, prix unitaire appliqué et sous-total.
- [ ] Gérer couleur, état à réception, observations et consignes particulières lorsque nécessaires.
- [ ] Réutiliser les mécanismes de configuration des prestations et tarifs propres au pressing.
- [ ] Ne pas imposer de liste fermée sans validation du modèle et des besoins de configuration.
- [ ] Couvrir les articles usuels sans en faire une énumération obligatoire : chemise, pantalon, costume, robe, drap, couverture, rideau, veste.
- [ ] Valider côté serveur une quantité positive et bornée ainsi qu'un prix unitaire valide.
- [ ] Rejeter les quantités nulles, négatives ou hors limites.
- [ ] Rejeter les lignes vides ou incohérentes.
- [ ] Vérifier que chaque article, prestation et tarif est autorisé pour le tenant concerné.
- [ ] Calculer les sous-totaux et le total à partir des lignes validées côté backend.
- [ ] Limiter le frontend à des estimations ; le backend reste l'autorité financière.
- [ ] Conserver le prix appliqué comme instantané historique indépendant des modifications ultérieures du catalogue.
- [ ] Éviter les erreurs de précision des flottants en réutilisant les conventions monétaires du projet.

### 2.3 — Numéro et dates du dépôt

- [ ] Générer le numéro officiel côté backend selon le mécanisme existant ou une solution persistante compatible.
- [ ] Garantir son unicité dans le périmètre métier retenu.
- [ ] Garantir une numérotation fiable sous requêtes simultanées.
- [ ] Garantir sa conservation après rechargement.
- [ ] Ne pas utiliser d'aléatoire navigateur ni d'identifiant temporaire frontend comme numéro officiel.
- [ ] Produire la date de réception depuis une source fiable avec politique de fuseau explicite.
- [ ] Saisir ou calculer la date prévue de restitution selon une règle métier existante, jamais arbitrairement.
- [ ] Distinguer création, réception, restitution prévue et remise réelle lorsque le modèle métier le nécessite.

## Phase 3 — Paiements et caisse : validation avant finalisation de l'interface

### 3.1 — Encaissement et données financières

- [ ] Enregistrer le montant réellement reçu, distinct du montant proposé par défaut dans l'interface.
- [x] Imposer le paiement intégral aux nouveaux dépôts ; conserver les règlements des commandes historiques partiellement payées sans réécriture de leur historique.
- [x] Refuser les nouveaux dépôts non soldés (décision D2), sans supprimer les anciennes commandes impayées.
- [ ] Prendre en charge plusieurs paiements successifs sur un même dépôt.
- [ ] Afficher le total, le cumul des paiements confirmés et le solde restant.
- [ ] Conserver et rendre consultable l'historique des règlements.
- [ ] Conserver le mode de paiement, la date, l'heure et l'utilisateur ayant enregistré chaque paiement.
- [ ] Utiliser exclusivement les moyens de paiement réellement pris en charge.
- [ ] Calculer le restant dû : total du dépôt − cumul des paiements valides.
- [ ] Supprimer l'acompte automatique de 50 % sauf règle explicitement configurée et approuvée.

### 3.2 — Garanties financières et corrections

- [ ] Valider les montants côté backend ; rejeter les valeurs négatives, invalides ou incohérentes.
- [ ] Appliquer la règle explicite de dépassement du solde retenue et validée.
- [ ] Vérifier l'appartenance du paiement au dépôt et au tenant courant.
- [ ] Garantir qu'une concurrence d'encaissements ne produit pas de solde incorrect.
- [ ] Garantir les transactions de création du dépôt et de son paiement initial selon l'architecture retenue.
- [ ] Ne jamais modifier ni supprimer silencieusement un paiement validé.
- [ ] Privilégier une contrepassation ou annulation traçable pour les corrections, adaptée au modèle existant.
- [ ] Distinguer, si nécessaire, les droits de consultation, encaissement, annulation et correction.
- [ ] Séparer statut financier et statut opérationnel du linge.
- [ ] Réutiliser ou définir les statuts financiers cohérents avec les règles retenues : non payé, partiellement payé, payé ; trop-perçu uniquement s'il est autorisé.
- [ ] Valider ces garanties par des tests backend avant de considérer le parcours caisse comme terminé.

## Phase 4 — Suivi du linge et historique

- [ ] Étudier et réutiliser les statuts existants lorsqu'ils répondent au besoin.
- [ ] Rendre clair le parcours cible : déposé/réceptionné → en attente → en cours → prêt → restitué.
- [ ] Adapter les libellés à l'architecture sans perdre leur sens métier.
- [ ] Valider chaque transition côté backend et appliquer les autorisations.
- [ ] Empêcher les transitions impossibles ou non autorisées.
- [ ] Conserver, lorsque compatible, ancien statut, nouveau statut, utilisateur, date, heure et observation éventuelle.
- [ ] Conserver l'historique, les articles et les informations financières après restitution.
- [x] Imposer côté backend un solde nul avant toute nouvelle restitution (validation métier obtenue le 12 octobre 2026).

## Phase 5 — Isolation multitenant et abonnements : exigences bloquantes

### 5.1 — Sécurité et intégrité

- [ ] Identifier le tenant depuis le contexte d'authentification fiable pour chaque opération.
- [ ] Ne jamais faire confiance à un `tenant_id` librement fourni par le navigateur.
- [ ] Vérifier l'appartenance du client au tenant courant.
- [ ] Vérifier l'appartenance du dépôt au tenant courant.
- [ ] Vérifier que paiements et articles sont liés à des ressources du même tenant.
- [ ] Protéger lectures, créations, modifications, annulations et transitions de statut.
- [ ] Appliquer les autorisations serveur indépendamment des contrôles d'interface.
- [ ] Empêcher l'accès intertenant par modification d'un identifiant dans l'URL ou le corps de requête.
- [ ] Vérifier les contraintes SQL, index et clés étrangères appropriés.
- [ ] Ajouter des tests négatifs démontrant le refus d'accès aux ressources d'un autre tenant.
- [ ] Préserver les données existantes et éviter les changements de schéma incompatibles.

### 5.2 — Réutilisation des entitlements

- [ ] Réutiliser le système existant d'abonnements et d'entitlements ; ne pas en créer un second.
- [ ] Protéger dépôts, paiements, tickets et suivi selon la matrice réelle des plans.
- [ ] Réutiliser les middlewares et mécanismes de quota existants.
- [ ] Ne pas inventer de droits ni de tarifs supplémentaires.
- [ ] Ne modifier aucun prix existant sans autorisation explicite.
- [ ] Retourner une erreur métier explicite et exploitable par le frontend lorsqu'une limite est atteinte.
- [ ] Ne supprimer aucune donnée historique lors d'une rétrogradation.
- [ ] Bloquer les nouvelles opérations dépassant une limite uniquement selon les règles d'abonnement existantes.
- [ ] Faire confirmer la règle tronquée concernant les consultations et la suite de la section 12 avant toute modification dépendante.

## Phase 6 — Intégration frontend fiable

### 6.1 — Saisie du dépôt et du client

- [ ] Permettre une saisie libre du client sans imposer une liste fermée.
- [ ] Intégrer recherche, sélection et création de client au formulaire de dépôt.
- [ ] Saisir séparément nom, prénoms et téléphone conformément aux règles validées.
- [ ] Afficher un avertissement de correspondance possible et laisser l'utilisateur vérifier son choix.
- [ ] Préserver les articles et autres données déjà saisis lors de la création ou modification du client.
- [ ] Intégrer toutes les données utiles des lignes d'articles et afficher les estimations de montant.
- [ ] Permettre au caissier de saisir le montant réellement reçu et le mode de paiement autorisé.
- [ ] Afficher clairement total, payé, solde, statut financier et statut opérationnel distincts.
- [ ] Rendre accessibles historique des règlements et suivi du linge selon les droits.

### 6.2 — Soumission, erreurs et nouvelle tentative

- [ ] Attendre le résultat réel de l'API avant d'ajouter définitivement une commande à la liste.
- [ ] Mettre à jour la liste depuis les données persistées ou la réponse canonique du serveur.
- [ ] Distinguer erreurs de validation, d'autorisation, de réseau et de serveur.
- [ ] Afficher un message compréhensible pour chaque catégorie, y compris les limites d'abonnement.
- [ ] Préserver les données saisies en cas d'échec.
- [ ] Empêcher les doubles soumissions accidentelles.
- [ ] Prévoir une nouvelle tentative contrôlée.
- [ ] Traiter un délai réseau avec résultat incertain selon la stratégie d'idempotence étudiée, sans créer de doublon.
- [ ] Ne pas générer de faux numéros, dates ou paiements.
- [ ] Ne pas ouvrir un ticket officiel comme si l'enregistrement ou le paiement était confirmé après un échec API.

## Phase 7 — Ticket thermique 80 mm

### 7.1 — Données du ticket

- [ ] Réutiliser le composant existant lorsqu'il convient.
- [ ] Alimenter le ticket officiel uniquement avec les données réellement enregistrées par le backend.
- [ ] Afficher le nom du pressing et ses coordonnées si configurées.
- [ ] Afficher le numéro officiel du dépôt et la date/heure de réception.
- [ ] Afficher l'identité du client et son téléphone lorsqu'il existe.
- [ ] Afficher le détail des articles, quantités, prix unitaires et sous-totaux.
- [ ] Afficher total, cumul payé et solde restant.
- [ ] Afficher la date prévue de restitution lorsqu'elle est renseignée.
- [ ] Afficher les observations utiles.
- [ ] Afficher les conditions de conservation ou de restitution lorsqu'elles sont configurées.
- [ ] Prévoir un message de remerciement facultatif.
- [ ] Ne jamais présenter un dépôt non enregistré comme ayant un numéro officiel et un paiement confirmé.

### 7.2 — Mise en page et impression

- [ ] Adapter la mise en page au papier 80 mm et à sa largeur réellement imprimable.
- [ ] Utiliser des tailles de texte lisibles et des marges maîtrisées.
- [ ] Prévoir un tableau compact, des retours à la ligne corrects et des textes longs sans débordement.
- [ ] Exclure de l'impression boutons, menus et autres éléments de l'application.
- [ ] Tester le rendu d'impression navigateur ou par les moyens disponibles.
- [ ] Conserver une preuve du rendu testé et indiquer les limites constatées.
- [ ] Tester sur une imprimante physique si disponible ; sinon signaler explicitement que cette validation reste à effectuer.
- [ ] Ne pas revendiquer de compatibilité universelle sans tests des modèles ou protocoles concernés.

## Phase 8 — Tests et preuves de validation

Les cas ci-dessous sont à adapter aux outils réellement constatés pendant l'audit, sans créer un système de validation parallèle inutile.

### 8.1 — Backend, base et sécurité

- [ ] Tester recherche et sélection client, création depuis le dépôt, téléphone absent si autorisé et correspondances possibles sans fusion automatique.
- [ ] Tester l'absence de création partielle selon les garanties transactionnelles retenues.
- [ ] Tester plusieurs lignes, rejets des lignes incohérentes et bornes de quantité/prix.
- [ ] Tester le calcul serveur, la représentation monétaire et l'instantané historique des prix.
- [ ] Tester l'unicité concurrente et la persistance du numéro officiel.
- [ ] Tester les dates et la distinction réception/restitution prévue/remise réelle selon le modèle.
- [ ] Tester paiements nul si autorisé, partiel, intégral et successifs.
- [ ] Tester montants invalides, dépassement du solde selon la règle validée et encaissements concurrents.
- [ ] Tester la transaction dépôt + paiement initial et son comportement en cas d'échec.
- [ ] Tester traçabilité et droits des annulations/corrections retenues.
- [ ] Tester séparation des statuts financiers et opérationnels.
- [ ] Tester transitions valides, impossibles et non autorisées, et conservation de l'historique après restitution.
- [ ] Tester négativement l'accès intertenant aux clients, dépôts, articles, tarifs et paiements, en lecture comme en écriture.
- [ ] Tester les permissions serveur indépendamment de l'interface.
- [ ] Tester quotas, fonctionnalités de plan, essai/expiration et conservation des données après rétrogradation selon les règles existantes.
- [ ] Tester l'application des migrations sur un état compatible avec les données existantes.

### 8.2 — Frontend, intégration et non-régression

- [ ] Tester la conservation de la saisie lors de la création client et après erreur API.
- [ ] Tester l'absence de succès local simulé après erreur de validation, autorisation, réseau ou serveur.
- [ ] Tester doubles clics, nouvelles tentatives et résultat réseau incertain selon la stratégie retenue.
- [ ] Tester que la liste et le ticket utilisent la réponse persistée, sans faux numéro ni faux paiement.
- [ ] Tester le ticket avec téléphone absent, date prévue absente, textes longs et plusieurs articles.
- [ ] Tester l'impression sans éléments d'interface parasites.
- [ ] Exécuter les suites existantes et nouveaux tests ciblés avec leurs commandes exactes.
- [ ] Exécuter les contrôles de formatage, lint, typage et build disponibles dans le dépôt.
- [ ] Vérifier les résultats CI et distinguer les tests locaux des contrôles distants.
- [ ] Documenter les régressions, échecs, tests non exécutés et leurs raisons.

## Phase 9 — Recette caissier, livraison et revue

### Parcours attendu à démontrer

- [ ] 1. Rechercher ou créer le client.
- [ ] 2. Enregistrer les articles et leurs quantités.
- [ ] 3. Vérifier le total et encaisser le montant réellement reçu.
- [ ] 4. Enregistrer le dépôt puis imprimer son ticket 80 mm depuis les données confirmées.
- [ ] 5. Suivre le linge jusqu'à sa restitution, sans perte d'historique ni confusion financière.

### Conditions de livraison

- [ ] Mettre à jour ce document au fil des phases avec des preuves, pas avec un simple « tout est terminé ».
- [ ] Fournir la liste des fichiers modifiés et la justification des changements.
- [ ] Fournir les migrations, leur compatibilité et les instructions de déploiement nécessaires.
- [ ] Fournir les commandes exécutées et leurs résultats vérifiables.
- [ ] Signaler explicitement les échecs, blocages, validations manquantes et décisions métier en attente.
- [ ] Documenter les risques de régression et les limites restantes, notamment l'impression physique non testée le cas échéant.
- [ ] Ouvrir une Pull Request depuis la branche de travail vers `main` pour contrôle avant intégration.
- [ ] Ne fusionner qu'après revue et autorisation explicite ; aucune fusion automatique.

## Journal d'exécution à compléter

Une entrée est attendue pour chaque phase effectivement travaillée. La création de cette checklist ne valide aucune fonctionnalité.

| Phase / date | Constats et références (fichier, symbole, route) | Fichiers modifiés | Commandes exécutées | Résultats et preuves | Échecs / limites / décisions en attente |
| --- | --- | --- | --- | --- | --- |
| Phase 0 / 2026-10-12 | Voir `docs/depot-phase0-audit.md`, références par fonction et route | `MiseJourClient.md`, `docs/depot-phase0-audit.md`, `docs/depot-phase0-validation.md` uniquement | `npm ci --legacy-peer-deps`, `npm test`, `npm run lint`, `npm run lint:eslint`, `npm run format:check`, `npm run build`, tentative `go test -race -cover ./...`, contrôles Git/GitHub, `git diff --check` et contrôle documentaire Python | 196 tests frontend / 25 fichiers ; types, format et build OK ; ESLint 0 erreur / 48 avertissements ; CI antérieure de main vérifiée en succès ; liens documentaires et périmètre des cases vérifiés | Go absent (code 127), PostgreSQL/Docker absents ; pas de validation SQL ni d’impression locale ; décisions D1–D8 du rapport en attente. Aucun code métier modifié, aucune fusion. |

### Premier lot backend clients — suivi détaillé (2026-10-12)

- [x] Consigner les décisions du propriétaire sans annoncer les règles financières comme déjà implémentées.
- [x] Exiger le téléphone dans le contrat de création client, sans valeur anonyme fabriquée au dépôt.
- [x] Résoudre le client du dépôt par ID et tenant, sans écrasement de coordonnées.
- [x] Créer un client structuré dans la transaction existante du dépôt ; ne pas fusionner par téléphone.
- [x] Autoriser les numéros partagés dans le schéma et protéger le lien tenant-client par FK composite.
- [x] Refuser l’ambiguïté du contrat historique avec un 409 exploitable.
- [x] Transmettre l’ID dans le sélecteur existant et le client HTTP pour préserver les dépôts de clients existants.
- [x] Tester les refus intertenant, le rollback et les migrations sur PostgreSQL 16.2.
- [ ] Construire la recherche et la création inline avec avertissement de numéro partagé dans le formulaire complet.
- [x] Livrer paiement intégral, somme remise, monnaie et refus de restitution impayée (lot 2 finances).
- [ ] Supprimer les faux succès locaux dans `App.tsx` et terminer l’idempotence avant recette caisse.

| Phase / date | Constats et références | Fichiers modifiés | Commandes exécutées | Résultats et preuves | Échecs / limites |
| --- | --- | --- | --- | --- | --- |
| Phase 1 + lot clients / 2026-10-12 | `docs/depot-phase1-decisions.md`, `docs/depot-lot1-clients.md` | Liste exhaustive dans le rapport du lot | Go race, intégration PostgreSQL, vet/build, migrations up/down, Vitest, tsc, ESLint, Prettier, build Vite | 172 tests Go de premier niveau + 73 sous-tests réussis, 1 sous-test ignoré ; 200 tests frontend au second passage ; migration validée | 5 tests de régression échouaient avant correction ; premier Vitest : échec du test de notification existant, réussi au second passage ; détails et erreurs d’outillage dans le rapport. Caisse/interface complète non livrées, aucune fusion. |
| Lot 2 finances & caisse / 2026-10-12 | `docs/depot-lot2-finances.md` | `backend/internal/order/finance*.go`, `persist.go`, `service.go`, `000019_*.sql`, `NewOrderModal.tsx`, `ordersApi.ts` | Go tests race/integration PostgreSQL, Vitest 201 tests, tsc, eslint, prettier, build | 178 tests Go top + 73 subtests réussis, 201 tests frontend réussis, calcul monnaie exact, rejet restitution impayée | Aucun blocage résiduel sur ce lot. Le ticket 80mm et la recherche approfondie client restent au programme des lots suivants. |
