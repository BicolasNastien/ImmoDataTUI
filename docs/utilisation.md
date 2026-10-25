# Utilisation

- [Lancer le programme](#lancer-le-programme)
- [Le parcours](#le-parcours)
- [Raccourcis clavier](#raccourcis-clavier)
- [La carte](#la-carte)
- [Les statistiques d'une commune](#les-statistiques-dune-commune)
- [La fiche d'une vente](#la-fiche-dune-vente)
- [Rechercher une adresse](#rechercher-une-adresse)
- [Comparer aux biens en vente](#comparer-aux-biens-en-vente)
- [Chercher des biens en vente](#chercher-des-biens-en-vente)
- [La collection locale](#la-collection-locale)
- [Ce que les chiffres veulent dire](#ce-que-les-chiffres-veulent-dire)

## Lancer le programme

```bash
immodata              # données rangées dans le dossier par défaut du système
immodata ~/immo       # ou dans le dossier indiqué
```

Le dossier passé en argument reçoit les extraits par commune. Sans argument,
voir [Emplacements](sources.md#emplacements). `q` quitte le programme à tout
moment hors des champs de saisie (où `ctrl+c` fonctionne toujours).

## Le parcours

On part d'une carte de France et on descend jusqu'à la commune :

```
carte de France          carte du département
(régions + outre-mer)    (communes)
        │                        │
   région choisie           commune choisie
        │                        │
        ▼                        ▼
 départements ──▶ téléchargement ──▶ millésimes ──▶ statistiques
                       ▲                                 ▲
                       │                                 │
    « / » ou « A » ──▶ recherche d'adresse ──────▶ page de l'adresse
                  (Base Adresse Nationale)    (tous millésimes agrégés)
```

1. **Carte de France** : choisir une région (flèches ou clic), `↵`.
2. **Carte de la région** : choisir un département, `↵`. Ses millésimes sont
   téléchargés la première fois (quelques Mo à quelques dizaines de Mo), puis
   lus depuis le cache.
3. **Carte du département** : choisir une commune — `/` filtre la liste par
   son nom.
4. **Millésimes** : une année précise, ou *Toutes les années* pour cumuler
   toute la période disponible.
5. **Statistiques** : des onglets par type de bien, un tableau de toutes les
   ventes, et la fiche détaillée de chacune.

Deux raccourcis court-circuitent ce parcours : `/` (ou `A`) pour aller
directement à une adresse, et `a` pour chercher des biens actuellement en
vente.

## Raccourcis clavier

### Partout

| Touche | Action |
|---|---|
| `↑` `↓` ou `k` `j` | se déplacer dans une liste, une carte ou un tableau |
| `↵` ou `→` | ouvrir (zone, millésime, vente, annonce) |
| `esc`, `←` ou `h` | revenir en arrière (annule aussi un téléchargement en cours) |
| `pgup` `pgdn` | se déplacer de dix lignes, ou faire défiler une page |
| `q` | quitter |
| `ctrl+c` | quitter, y compris pendant une saisie |

### Souris

| Geste | Action |
|---|---|
| survol | met en évidence la zone de la carte ou la ligne sous le pointeur |
| clic | sélectionne ; un second clic sur la même ligne ou la même zone l'ouvre |
| molette | fait défiler |

### Sur la carte

| Touche | Action |
|---|---|
| `/` | carte d'un département : filtrer les communes ; ailleurs : rechercher une adresse |
| `A` | rechercher une adresse, à tous les niveaux |
| `a` | chercher des biens actuellement en vente |
| `r` | sur un département : forcer le retéléchargement de ses données |
| `l` | ouvrir la collection locale (si elle existe) |

### Filtre des communes (après `/` sur un département)

| Touche | Action |
|---|---|
| lettres, chiffres, espace | réduire la liste (nom ou code INSEE) |
| `backspace` | effacer le dernier caractère (sur un filtre vide : fermer le filtre) |
| `↑` `↓` | parcourir la liste réduite |
| `↵` | ouvrir la commune sélectionnée |
| `esc` | effacer le filtre |

### Recherche d'adresse

| Touche | Action |
|---|---|
| saisie libre | taper l'adresse (toutes les touches vont dans le champ) |
| `↵` | lancer la recherche ; un second `↵` ouvre le résultat sélectionné |
| `↑` `↓` | choisir parmi les résultats |
| `esc` | revenir à la carte |

### Statistiques

| Touche | Action |
|---|---|
| `tab` / `shift+tab` | onglet suivant / précédent |
| `↑` `↓` | déplacer la ligne sélectionnée du tableau des ventes (ou des annonces) |
| `pgup` `pgdn` / `espace` | faire défiler sans changer la sélection |
| `s` | changer le tri du tableau des ventes : date, prix, €/m², surface |
| `↵` | ouvrir la fiche de la vente sélectionnée (onglet En vente : ouvrir l'annonce) |
| `o` | onglet En vente : ouvrir l'annonce sélectionnée dans le navigateur |
| `v` `c` `V` | voir la vente sélectionnée dans le navigateur : carte, cadastre, vue de la rue |

### Fiche d'une vente

| Touche | Action |
|---|---|
| `n` / `p` | vente suivante / précédente du tableau |
| `v` | carte avec un repère sur le bien (Google Maps) |
| `c` | photo aérienne et cadastre (Géoportail) |
| `V` | vue de la rue (Street View) |
| `pgup` `pgdn` / `espace` | faire défiler la fiche |

### Biens en vente : formulaire de critères

| Touche | Action |
|---|---|
| `↑` `↓` ou `tab` `shift+tab` | changer de champ |
| chiffres, `backspace` | saisir ou corriger une valeur numérique |
| `espace` ou `↵` | cocher / décocher (types de bien, exclure le neuf) |
| `↵` sur *Zone* | ouvrir le choix des zones |
| `backspace` sur *Zone* | retirer toutes les zones |
| `↵` sur un autre champ | lancer la recherche |
| `esc` | revenir à la carte |

### Biens en vente : choix des zones

| Touche | Action |
|---|---|
| saisie libre | nom d'une région, d'un département ou d'une commune |
| `↵` | chercher ; un second `↵` ajoute (ou retire) la zone sélectionnée et revient au formulaire |
| `↑` `↓` | choisir parmi les zones trouvées |
| `espace` | ajouter ou retirer la zone sélectionnée, sans quitter l'écran (pour en cumuler plusieurs) |
| `esc` | revenir au formulaire |

### Biens en vente : résultats

| Touche | Action |
|---|---|
| `↑` `↓` | parcourir les annonces |
| `s` | changer le tri : prix, €/m², surface, terrain, date |
| `i` | inverser le sens du tri |
| `↵` ou `o` | ouvrir l'annonce dans le navigateur |
| `r` | relancer la recherche |
| `f`, `esc` ou `←` | revenir aux critères |

## La carte

Chaque caractère du terminal porte deux « pixels » grâce au demi-bloc `▀` :
la moitié haute prend la couleur d'avant-plan, la moitié basse celle du fond.
Deux zones voisines n'ont jamais la même couleur. Codes couleur :

- **vert** : départements déjà téléchargés ;
- **jaune** : zone sélectionnée.

L'outre-mer (Guadeloupe, Martinique, Guyane, La Réunion, Mayotte) est affiché en
encarts. Les contours des régions et départements sont dans le programme : le
premier écran s'affiche sans réseau. Ceux des communes sont téléchargés à
l'ouverture d'un département.

Sur la carte d'un département, le filtre `/` ignore accents, casse et tirets
(`saint jean` trouve Saint-Jean-de-Luz). Les noms qui commencent par la saisie
passent devant ceux qui la contiennent. La carte continue de montrer tout le
département, avec la commune sélectionnée en évidence. Le filtre reste posé au
retour sur la carte : un premier `esc` l'efface, le suivant remonte aux
départements.

## Les statistiques d'une commune

### Choisir la période

La liste des millésimes commence par **Toutes les années**, qui fusionne tous
les millésimes disponibles. C'est souvent le bon choix : une commune moyenne
compte quelques dizaines de ventes par an et par type de bien, trop peu pour
une médiane fiable par tranche de surface. L'hypothèse est alors que le marché
est comparable sur toute la période — l'onglet Résumé permet de la vérifier.

### Onglet Résumé

- nombre de ventes par type de bien, prix médian et €/m² médian ;
- activité mois par mois (saisonnalité) ;
- voies les plus actives et leur prix médian ;
- nombre de mutations écartées (voir
  [Ce que les chiffres veulent dire](#ce-que-les-chiffres-veulent-dire)) ;
- sur une période fusionnée : **évolution millésime par millésime** (ventes,
  prix médian, €/m² médian, surface médiane, volume). La tendance entre la
  première et la dernière année n'est affichée que si chacune compte au moins
  cinq ventes.

### Onglets Maisons et Appartements

- **prix médian** (plutôt que la moyenne, que quelques ventes atypiques
  tirent) ;
- **cœur du marché Q1 → Q3** : la fourchette où se situent la moitié des
  ventes, la plus utile pour caler un budget ;
- minimum et maximum ;
- **€/m² médian** et sa dispersion ;
- surfaces bâtie et de terrain médianes ;
- tableaux **par tranche de surface** et **par nombre de pièces** : nombre de
  ventes, prix médian, €/m² médian. On y voit à partir de quelle surface le
  €/m² décroche.

### Onglet Terrains

Prix médian et **€/m² de terrain nu**. La dispersion est large : la catégorie
mélange terrains constructibles, agricoles et boisés.

### Le tableau des ventes

Sous les statistiques des onglets Maisons, Appartements et Terrains, chaque
vente est listée, la plus récente en tête :

| Colonne | Lecture |
|---|---|
| Date | date de la mutation |
| Prix | valeur foncière |
| Surface | surface bâtie (de terrain pour un terrain nu) |
| P. | pièces principales |
| €/m² | prix au m² de cette vente |
| vs méd. | écart de ce €/m² au €/m² médian du type |
| Terrain | surface de terrain rattachée |
| Adresse | numéro et voie |

`s` change le tri. La colonne **vs méd.** situe chaque vente dans son marché :
une vente à −30 % mérite qu'on ouvre sa fiche, une vente à +45 % aussi, pour
une autre raison.

## La fiche d'une vente

`↵` sur une ligne du tableau (ou un second clic) ouvre le détail complet de la
mutation — aucune colonne du fichier DVF n'est perdue :

- **Prix** : valeur foncière, €/m² bâti et de terrain, écart au prix et au €/m²
  médians de la commune ;
- **Bien** : type de local, surface bâtie, pièces, terrain, lots de
  copropriété et surfaces Carrez, liste des locaux quand la vente en compte
  plusieurs (dépendances…) ;
- **Adresse** : numéro, voie, code postal, commune, code INSEE, coordonnées ;
- **Mutation** : identifiant, date, nature, numéro de disposition ;
- **Parcelles et natures de culture** : chaque subdivision cadastrale et sa
  surface ;
- **Lignes brutes** : les lignes du fichier telles quelles, pour comprendre un
  chiffre qui surprend.

### Voir le bien dans le navigateur

Depuis la fiche, ou directement depuis la ligne sélectionnée du tableau :

| Touche | Vue | Ce qu'elle apporte |
|---|---|---|
| `v` | carte (Google Maps) | un repère sur le point du bien |
| `c` | photo aérienne + cadastre (Géoportail) | les limites de la parcelle vendue |
| `V` | vue de la rue (Street View) | la façade, depuis le panorama le plus proche |

Dans cet ordre : *où est-ce*, *quelle parcelle*, *à quoi ça ressemble*. La vue
de la rue seule ne dit pas lequel des bâtiments est le bon ; la carte et le
cadastre lèvent l'ambiguïté.

Les liens utilisent les coordonnées de la parcelle quand DVF les fournit (cas
général). Sinon, ils se rabattent sur l'adresse, et le cadastre n'est pas
proposé. Sans environnement graphique, le lien complet est affiché pour être
copié.

## Rechercher une adresse

`/` (ou `A` à tous les niveaux) ouvre un champ de saisie. Taper l'adresse, `↵`
pour chercher, choisir un résultat, `↵` pour l'ouvrir. Le département est
téléchargé au besoin et **tous les millésimes de la commune sont agrégés** — un
logement ne se revend qu'une fois tous les dix ans en moyenne.

Un onglet **Adresse** s'ajoute en tête des statistiques :

- l'adresse trouvée : libellé, commune, département, coordonnées, période ;
- **les ventes à ce numéro précis**, sur toute la période. Ne rien trouver est
  le cas normal ;
- **la voie** : ventes par type de bien et écart de son €/m² à celui de la
  commune — ce qui distingue une rue chère d'une rue ordinaire ;
- le **tableau des ventes de la voie**, le curseur posé sur une vente au numéro
  cherché s'il y en a une.

Les noms de voie sont rapprochés malgré leurs écritures différentes (DVF écrit
`AV DU PORT`, la Base Adresse Nationale « Avenue du Port »). Si la voie
n'apparaît dans aucune vente, le tableau porte sur toute la commune, et le dit.

## Comparer aux biens en vente

L'onglet **En vente** des statistiques confronte les biens anciens actuellement
en vente dans la commune aux ventes DVF de la période choisie. Les annonces
sont chargées à la première ouverture de l'onglet.

Comparer les médianes des annonces et des ventes mesurerait surtout la
différence de composition des deux échantillons. **Chaque annonce est donc
comparée à ses propres ventes similaires** :

| | Critère strict | Critère élargi (marqué `≈`) |
|---|---|---|
| Type | identique | identique |
| Surface (habitable, ou de terrain pour un terrain nu) | ±15 % | ±25 % |
| Pièces (6 et plus confondues) | identique | ±1 |
| Terrain d'une maison | entre ½ et 2 fois | entre ⅓ et 3 fois |
| Ventes similaires requises | 5 | 5 |

L'estimation vaut le €/m² médian des ventes similaires multiplié par la surface
de l'annonce. Avant le calcul :

- le prix est ramené au **net vendeur** (hors honoraires d'agence), comme dans
  l'acte notarié ;
- les **viagers** sont écartés (leur prix n'est que le bouquet) ;
- les **doublons** (un bien confié à plusieurs agences) ne comptent qu'une fois.

L'onglet affiche une synthèse par type, puis chaque annonce de la plus
avantageuse à la plus chère ; celles qui n'ont pas pu être comparées (sans
surface, ou moins de cinq ventes similaires) viennent ensuite, marquées `–`.
`↵` ou `o` ouvre l'annonce sélectionnée.

À garder en tête : un prix demandé se négocie, quelques pour cent au-dessus des
ventes sont donc normaux ; et sur plusieurs millésimes, l'écart inclut
l'évolution des prix depuis.

## Chercher des biens en vente

`a` depuis la carte ouvre la recherche d'annonces, indépendante de DVF :

```
carte ──« a »──▶ critères ──« ↵ »──▶ résultats ──« ↵ »──▶ annonce (navigateur)
                    │                    ▲
                    └── zones ───────────┘
                    (région, département, commune, cumulables)
```

| Critère | Portée |
|---|---|
| Zone | région, département ou commune — plusieurs zones se cumulent |
| Rayon max | en km autour du centre de la première zone |
| Maisons, Appartements, Terrains nus | types cochables indépendamment |
| Prix min / max | en euros |
| Surface min / max | surface habitable, en m² |
| Terrain min / max | surface de terrain, en m² |
| Pièces min | pièces principales |
| Exclure le neuf | écarte programmes et biens neufs |
| Nombre max de résultats | 200 par défaut |

Un champ vide ne contraint rien. Le tri par défaut est le prix croissant ; `s`
et `i` le changent instantanément, sans nouvelle requête. Les valeurs absentes
restent toujours en fin de liste.

Ce que ce tableau n'est pas :

- **le marché entier** : seules les premières centaines d'annonces sont
  rapportées, la ligne sous le tableau donne la proportion ;
- **des prix de vente** : ce sont des prix demandés, à ne pas confondre avec
  DVF ;
- **des positions exactes** : les portails floutent l'adresse au quartier ou à
  la commune.

## La collection locale

Des fichiers DVF peuvent aussi être déposés à la main dans le dossier de
données, au format CSV de
[geo-dvf](https://files.data.gouv.fr/geo-dvf/latest/csv/) :

```
<dossier de données>/
└── <Commune>/
    └── <année>/
        └── <n'importe quel nom>.csv
```

C'est exactement l'arborescence que le programme crée lui-même en extrayant
une commune : un fichier déposé et un fichier téléchargé sont
indiscernables. `l` depuis la carte ouvre cette collection.

## Ce que les chiffres veulent dire

**Mutations écartées.** Ne sont pas comptées :

- les mutations autres que des ventes (échanges, expropriations…) ;
- les ventes sans valeur foncière ;
- les **ventes groupées** de plusieurs biens distincts (deux maisons, une maison
  et un appartement…) : leur prix ne peut pas être attribué à un bien.

Un bien réparti sur plusieurs parcelles cadastrales est en revanche normal et
conservé.

**Prix net vendeur.** Les valeurs foncières DVF n'incluent ni les frais de
notaire (environ 7–8 % dans l'ancien, 2–3 % dans le neuf), ni le mobilier. Les
honoraires d'agence n'y figurent que s'ils sont à la charge du vendeur.

**Données provisoires.** Quand le dernier millésime vient du fichier brut de la
DGFiP plutôt que de geo-dvf, l'écran l'indique : les ventes n'ont alors pas de
coordonnées (voir [Sources](sources.md#repli-sur-le-fichier-brut-de-la-dgfip)).

**Délai de publication.** DVF est publié deux fois par an, avec plusieurs mois
de retard sur les ventes : les transactions des derniers mois n'y sont pas
encore.
