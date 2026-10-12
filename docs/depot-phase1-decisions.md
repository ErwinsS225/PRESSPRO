# Dépôts — décisions de phase 1 et contrat du premier lot

Date : 2026-10-12. Suite à l'audit et aux confirmations du propriétaire.

## Règles métier confirmées (prioritaires sur les propositions de l'audit)

1. **Téléphone obligatoire** pour le client.
2. **Numéro partagé autorisé** entre plusieurs clients distincts du même pressing. Afficher un avertissement de correspondance ; aucune fusion ou modification automatique.
3. **Paiement intégral obligatoire lors d'un nouveau dépôt**. Supprimer le choix d'un acompte automatique de 50 % et ne pas enregistrer un dépôt nouveau non soldé.
4. **Monnaie à rendre** lorsque le montant remis dépasse le total : paiement affecté = total ; monnaie = montant remis − total. Ne pas enregistrer l'excédent comme recette ni comme crédit client.
5. **Restitution interdite tant que le solde n'est pas nul** ; contrôle à réaliser côté backend, pas seulement dans l'interface.

Ces décisions sont confirmées, mais les points financiers 3–5 ne sont **pas encore implémentés dans le lot clients**. La validation des montants, la monnaie rendue et le blocage de restitution constitueront le lot suivant. Distinguer somme remise, paiement affecté et monnaie rendue dans le futur contrat canonique.

Les commandes historiques non soldées ne seront ni effacées ni déclarées payées par migration. Conserver leur historique et leur mécanisme de règlement ; elles devront être soldées avant une future restitution. Ne pas réécrire les restitutions déjà effectuées.

La monnaie à rendre selon le mode d'encaissement (espèces versus paiement électronique), les dérogations de prix, les plafonds/précision monétaires, les rôles de caisse, les transitions détaillées et la date prévue restent à cadrer avant leurs lots respectifs. Aucun prix ni entitlement d'abonnement ne change.

## Lot clients — contrat backend cible

Réutiliser `POST /api/v1/orders`, pas une seconde API de dépôts.

- **Client existant** : `customer_id`. Le serveur relit la fiche du tenant authentifié dans la transaction ; nom/téléphone éventuellement fournis comme anciens champs ne modifient pas la fiche et ne font pas autorité. Un client étranger ou inconnu renvoie 404.
- **Nouveau client** : `new_customer` contenant `first_name`, `last_name`, `phone` et les champs facultatifs du contrat clients existant. Création dans la transaction du dépôt. Aucun upsert par téléphone ; deux personnes partageant le numéro reçoivent des IDs distincts. Le futur frontend doit rechercher et avertir avant ce choix explicite.
- **Ancien contrat** : `customer_name` + `customer_phone` restent acceptés pour un numéro sans correspondance. Si le numéro correspond déjà à une fiche du tenant, répondre 409 et demander un choix explicite (ID ou `new_customer`), plutôt que créer un doublon silencieux ou modifier un client. Une erreur ne doit laisser ni client ni dépôt partiel.
- Refuser `customer_id` + `new_customer` simultanés et un objet structuré mélangé aux anciens champs. Le téléphone vide est refusé ; aucune valeur `anonyme-*` fabriquée.
- Modes mémoire : ne jamais accepter un ID client non vérifiable. L'absence de persistance durable du mode dégradé reste une limite, pas une preuve d'enregistrement officiel.
- `GET /api/v1/customers?q=...` et CRUD clients restent les mécanismes de recherche/gestion. Le frontend viendra après validation backend.

## Migration candidate 000018

- Retirer seulement l'unicité `(tenant_id, phone)` ; garder le téléphone obligatoire.
- Indexer `(tenant_id, phone)` sans unicité pour la recherche exacte de correspondance.
- Aucun renommage, fusion ou suppression de client historique.
- Le retour arrière doit **refuser** si des numéros partagés ont été créés : ne jamais supprimer des fiches pour rétablir l'unicité. Déploiement coordonné : l'ancien backend utilise `ON CONFLICT (tenant_id, phone)` et ne doit pas être remis en service après retrait de cette contrainte.

## Validation prévue du lot

Tests de refus téléphone absent, ambiguïté d'identité, client intertenant/inconnu, sélection sans écrasement, noms/prénoms conservés, numéros partagés distincts, compatibilité legacy sans fusion, rollback client+dépôt et quota/numéro inchangés en cas de refus. Tester migration up/down sans perte et refus explicite du down en présence de doublons.

Aucune interface achevée ni nouvelle règle financière livrée par ce document. Revue par PR, aucune fusion automatique.
