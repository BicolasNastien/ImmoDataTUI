# Sources de données

ImmoDataTUI n'a pas de serveur à lui : il interroge directement des sources
publiques, et ne garde qu'un cache local. Cette page dit d'où vient chaque
chiffre, à quelle fréquence il est récupéré, et où il est stocké.

- [Vue d'ensemble](#vue-densemble)
- [Les transactions : DVF](#les-transactions--dvf)
- [Repli sur le fichier brut de la DGFiP](#repli-sur-le-fichier-brut-de-la-dgfip)
- [Les contours géographiques](#les-contours-géographiques)
- [La recherche d'adresse](#la-recherche-dadresse)
- [Les biens en vente](#les-biens-en-vente)
- [Les liens vers le navigateur](#les-liens-vers-le-navigateur)
- [Emplacements](#emplacements)
- [Fraîcheur et mise à jour](#fraîcheur-et-mise-à-jour)
- [Couverture](#couverture)
- [Conditions d'utilisation](#conditions-dutilisation)

## Vue d'ensemble

| Donnée | Source | Récupération |
|---|---|---|
| Transactions immobilières | [geo-dvf](https://files.data.gouv.fr/geo-dvf/latest/csv/) (Etalab), à partir des « Demandes de valeurs foncières » de la DGFiP | à la demande, par département, mise en cache |
| Transactions, millésime pas encore dans geo-dvf | [fichier brut de la DGFiP](https://www.data.gouv.fr/fr/datasets/demandes-de-valeurs-foncieres/) sur data.gouv.fr | en repli, mise en cache |
| Contours des communes | [API Découpage administratif](https://geo.api.gouv.fr/) (`geo.api.gouv.fr`) | à la demande, par département, mise en cache |
| Contours des régions et départements | [france-geojson](https://github.com/gregoiredavid/france-geojson) pour la métropole, contours des communes fusionnés pour l'outre-mer | embarqués dans le programme |
| Recherche d'adresse | [Base Adresse Nationale](https://adresse.data.gouv.fr/) (`api-adresse.data.gouv.fr`) | à chaque recherche, sans cache |
| Biens en vente | [Bien'ici](https://www.bienici.com/) | à chaque recherche, sans cache |

Aucune autre requête n'est émise, à l'exception des liens que l'on choisit
d'ouvrir dans le navigateur (voir [plus bas](#les-liens-vers-le-navigateur)).

## Les transactions : DVF

La base **Demandes de valeurs foncières** (DVF) est publiée par la Direction
générale des Finances publiques (DGFiP). Elle recense les ventes immobilières
enregistrées par les services de publicité foncière : date, prix, nature de la
mutation, type de bien, surfaces, nombre de pièces, parcelles cadastrales.

Le programme utilise la version **geo-dvf** retraitée par Etalab, qui ajoute
les coordonnées géographiques de chaque parcelle et un format CSV plus régulier
(un fichier par département et par année) :

```
https://files.data.gouv.fr/geo-dvf/latest/csv/<année>/departements/<dd>.csv.gz
```

À l'ouverture d'un département, les **six derniers millésimes** sont
téléchargés (trois à la fois, pour ménager le serveur public), puis la commune
choisie en est extraite. Une archive départementale pèse de quelques Mo à
quelques dizaines de Mo.

Ce que DVF contient, et ne contient pas :

- les ventes de biens immobiliers (maisons, appartements, dépendances, locaux,
  terrains) ;
- le **prix net vendeur**, sans frais de notaire ni mobilier ;
- **aucun nom** d'acheteur ou de vendeur ;
- ni les ventes en Alsace-Moselle ni à Mayotte (voir [Couverture](#couverture)).

## Repli sur le fichier brut de la DGFiP

geo-dvf est publié plusieurs semaines après le fichier de la DGFiP. Entre les
deux, le millésime le plus récent manque sur geo-dvf (réponse 404). Dans ce
cas, le programme :

1. demande au catalogue `data.gouv.fr` la liste des fichiers de l'année (leurs
   adresses changent à chaque publication ; une année complète remplace ses
   semestres) ;
2. télécharge le fichier national zippé (30 à 90 Mo), une seule fois par
   publication ;
3. n'en garde que le département demandé et le convertit au format geo-dvf :
   colonnes renommées, dates au format ISO, point décimal, code INSEE et
   identifiant de parcelle reconstitués.

L'archive obtenue est marquée **provisoire** et l'interface l'indique. Elle n'a
pas de coordonnées : les liens vers la carte et la vue de la rue passent par
l'adresse, et le lien cadastre disparaît. À chaque ouverture, le programme
vérifie (requête `HEAD`) si geo-dvf a publié le millésime ; si oui, l'archive
provisoire est remplacée.

Comparée à geo-dvf sur le millésime 2025 de la Lozère, la conversion retrouve
toutes ses lignes et ses mutations.

## Les contours géographiques

- **Régions et départements** : produits une fois pour toutes par
  `app/geodata/generer.py` (sources : france-geojson pour la métropole,
  contours des communes fusionnés pour l'outre-mer), simplifiés et embarqués
  dans le binaire. Le premier écran s'affiche donc sans réseau.
- **Communes** : demandées à `geo.api.gouv.fr` à l'ouverture d'un département,
  puis mises en cache.

## La recherche d'adresse

Les adresses tapées après `/` ou `A` sont envoyées à l'API de la **Base Adresse
Nationale** (`api-adresse.data.gouv.fr/search/`). Le code INSEE du résultat
désigne la commune et son département ; les coordonnées servent aux liens vers
le navigateur.

## Les biens en vente

Les annonces viennent de **Bien'ici**, interrogé à chaque recherche :

- `res.bienici.com/suggest.json` pour retrouver une commune, un département ou
  une région à partir de son nom ;
- `www.bienici.com/realEstateAds.json` pour les annonces elles-mêmes.

Ces points d'accès sont ceux qu'utilise le site ; ils **ne sont pas
documentés**, peuvent changer sans préavis, et rien ne garantit qu'ils
appliquent tous les filtres envoyés. Le programme réapplique donc chaque
critère au retour (Bien'ici ignore par exemple certaines bornes de surface de
terrain sans le signaler), et calcule lui-même le rayon autour d'une commune.
Les requêtes portent un `User-Agent` qui identifie le programme. Rien n'est
mis en cache : les annonces changent trop vite.

Les annonces ne donnent pas d'adresse exacte : leur position est celle d'un
quartier ou d'une commune.

**Pourquoi pas leboncoin.** `www.leboncoin.fr` et `api.leboncoin.fr` refusent
(403) toute requête qui ne vient pas d'un navigateur, et leur `robots.txt`
interdit explicitement tout accès automatisé sans autorisation. Ce refus est
respecté.

Les portails sont branchés derrière une interface commune (`SourceAnnonces`,
dans `app/annonces.go`) : en ajouter un ne demande qu'un fichier, et une source
en panne n'empêche pas d'afficher les résultats des autres.

## Les liens vers le navigateur

Les touches `v`, `c` et `V` (fiche d'une vente) et `o` (annonce) ouvrent une
page dans le navigateur. Elles transmettent, et seulement à ce moment :

| Touche | Service | Ce qui est envoyé |
|---|---|---|
| `v` | Google Maps | coordonnées de la parcelle, ou adresse à défaut |
| `c` | Géoportail (IGN) | coordonnées de la parcelle |
| `V` | Google Street View | coordonnées de la parcelle, ou adresse à défaut |
| `o` | Bien'ici | identifiant de l'annonce |

## Emplacements

Le programme écrit à deux endroits : un **cache** (archives téléchargées,
reconstituables à tout moment) et un **dossier de données** (extraits par
commune).

### Dossier de données

Dans l'ordre de priorité :

1. le dossier passé en argument : `immodata ~/immo` ;
2. un dossier `data/` présent dans le répertoire depuis lequel on lance le
   programme ;
3. sinon, le dossier standard du système :

| Système | Dossier de données |
|---|---|
| Linux | `$XDG_DATA_HOME/immodata/data`, à défaut `~/.local/share/immodata/data` |
| macOS | `~/Library/Application Support/immodata/data` |
| Windows | `%LocalAppData%\immodata\data` |

On y trouve un dossier par commune : `<Commune>/<année>/<code INSEE>.csv`.

### Cache

| Système | Cache |
|---|---|
| Linux | `$XDG_CACHE_HOME/immodata`, à défaut `~/.cache/immodata` |
| macOS | `~/Library/Caches/immodata` |
| Windows | `%LocalAppData%\immodata` |

| Contenu | Chemin dans le cache |
|---|---|
| Archives départementales geo-dvf | `dvf/<année>/<dd>.csv.gz` |
| Fichiers bruts nationaux de la DGFiP | `dgfip/valeursfoncieres-<année>[-s1].txt.zip` |
| Contours des communes | `geo/communes-<dd>.json` |

Le cache peut être supprimé sans risque : il sera retéléchargé au besoin.

## Fraîcheur et mise à jour

DVF est republié **deux fois par an** (en général au printemps et à
l'automne), et chaque publication complète aussi les millésimes passés. Les
ventes y apparaissent donc avec plusieurs mois de retard.

- Une archive en cache est réutilisée pendant **trois mois**, puis
  retéléchargée à l'ouverture suivante du département.
- `r` sur un département force le retéléchargement immédiat.
- Une archive provisoire (fichier brut DGFiP) est revérifiée à chaque ouverture.

## Couverture

- **Toute la France**, métropole et départements d'outre-mer, **sauf** :
  - la **Moselle**, le **Bas-Rhin** et le **Haut-Rhin**, où la publicité
    foncière relève du livre foncier local et non de la DGFiP ;
  - **Mayotte**.
- **Six derniers millésimes**, selon ce que geo-dvf et la DGFiP publient.
- Les mutations qui ne sont pas des ventes, sans prix, ou qui portent sur
  plusieurs biens distincts à la fois sont écartées des statistiques (voir
  [Utilisation](utilisation.md#ce-que-les-chiffres-veulent-dire)).

## Conditions d'utilisation

- **DVF** et **geo-dvf** sont diffusés sous la
  [Licence Ouverte 2.0](https://www.etalab.gouv.fr/licence-ouverte-open-licence/).
  Leurs conditions générales d'utilisation interdisent en outre toute
  **réidentification des personnes** concernées par les ventes et toute
  indexation des données par les moteurs de recherche. ImmoDataTUI ne fait que
  lire et agréger les données localement ; ce qu'on en fait ensuite relève de
  l'utilisateur.
- La **Base Adresse Nationale** et le **Découpage administratif** sont des
  données publiques ouvertes, diffusées sur `data.gouv.fr`.
- Les **annonces Bien'ici** restent la propriété de leurs auteurs et du
  portail ; ImmoDataTUI les affiche le temps d'une consultation, sans les stocker.
