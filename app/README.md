# dvf-tui

Exploration en terminal des données DVF (Demandes de Valeurs Foncières), orientée
**décision d'achat immobilier**.

Les données sont récupérées à la demande : on choisit une région sur une carte
de France dessinée dans le terminal, puis un département, dont les millésimes
sont téléchargés depuis `files.data.gouv.fr`. La carte devient alors celle du
département découpé en communes, et cliquer sur une commune ouvre ses
statistiques.

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

La touche `/` court-circuite les trois échelons : on tape une adresse, le
département est déduit de son code INSEE, ses millésimes sont téléchargés et
l'on arrive directement sur la page de l'adresse.

La touche `a` ouvre l'autre versant du marché : les **biens actuellement en
vente**, interrogés en direct chez les portails d'annonces, avec filtres et tri.

```
carte ──« a »──▶ critères ──« ↵ »──▶ résultats ──« ↵ »──▶ annonce (navigateur)
                    │                    ▲
                    └── zones ───────────┘
                    (région, département, commune, cumulables)
```

## Installation

```bash
go mod tidy
go build -o dvf-tui .
./dvf-tui            # racine des données : data/ (créée si absente)
```

Le dossier passé en argument sert de racine aux données extraites.

## Sources

| Donnée | Source | Récupération |
|---|---|---|
| Transactions DVF | `files.data.gouv.fr/geo-dvf/latest/csv/<année>/departements/<dd>.csv.gz` | à la demande, mise en cache |
| Transactions DVF, millésime absent de geo-dvf | fichier brut de la DGFiP, jeu « Demandes de valeurs foncières » sur `data.gouv.fr` | en repli, mise en cache |
| Contours des communes | `geo.api.gouv.fr` | à la demande, mise en cache |
| Recherche d'adresse | `api-adresse.data.gouv.fr` (Base Adresse Nationale) | à la demande |
| Biens en vente | `bienici.com` (`realEstateAds.json`, `suggest.json`) | en direct, sans cache |
| Contours des régions et départements | embarqués dans le binaire (`geodata/`) | — |

L'API Découpage administratif ne sert pas de géométrie pour les régions et les
départements : ces contours sont produits une fois pour toutes par
`geodata/generer.py` (sources : *france-geojson* pour la métropole, contours des
communes fusionnés pour l'outre-mer), simplifiés puis embarqués. Le premier
écran ne dépend donc d'aucun réseau.

## Emplacements

| Quoi | Où |
|---|---|
| Archives départementales (`.csv.gz`) | `~/.cache/dvf-tui/dvf/<année>/<dd>.csv.gz` |
| Fichiers bruts nationaux de la DGFiP | `~/.cache/dvf-tui/dgfip/valeursfoncieres-<année>[-s1].txt.zip` |
| Contours de communes | `~/.cache/dvf-tui/geo/communes-<dd>.json` |
| Extraits par commune | `<racine>/<Commune>/<année>/<code INSEE>.csv` |

Les extraits adoptent l'arborescence que le programme sait déjà lire : un fichier
téléchargé et un fichier déposé à la main sont indiscernables, et une collection
existante continue de fonctionner sans migration. La touche `l` ouvre cette
collection locale directement.

Les archives sont revérifiées au bout de trois mois — le jeu DVF est republié
deux fois par an, et une publication complète les millésimes passés. La touche
`r` sur un département force le retéléchargement.

### Repli sur le fichier brut de la DGFiP

geo-dvf est un retraitement par Etalab du fichier publié par la DGFiP, et il
sort plusieurs semaines après lui. Entre les deux, le millésime le plus récent
(par exemple le premier semestre 2026, publié par la DGFiP le 12/10/2026)
renvoie un 404 sur geo-dvf. Dans ce cas, le programme :

1. demande au catalogue `data.gouv.fr` les fichiers de l'année. Les URL
   changent à chaque publication. Une année complète remplace ses semestres ;
2. télécharge le fichier national zippé (30 à 90 Mo), une fois par publication ;
3. n'en garde que le département demandé et le réécrit au format geo-dvf :
   colonnes renommées, dates en ISO, point décimal, code INSEE et identifiant
   de parcelle reconstitués, mutations numérotées `<année>-D<n>`.

L'archive obtenue est marquée **provisoire**. Elle n'a ni coordonnées (les
liens Street View et Google Maps passent par l'adresse, le lien Géoportail
disparaît), ni libellé pour les natures de culture spéciales. L'écran de
téléchargement et l'en-tête des statistiques l'indiquent. À chaque ouverture,
une requête `HEAD` vérifie si geo-dvf a publié le millésime. Si oui, l'archive
provisoire est remplacée.

Comparée à geo-dvf sur le millésime 2025 de la Lozère, la conversion retrouve
toutes ses lignes et ses mutations. Les quelques lignes en plus sont des ventes
ajoutées par la publication d'octobre de la DGFiP, que geo-dvf n'a pas encore
reprises. Le fichier brut ne couvre pas la Moselle, le Bas-Rhin, le Haut-Rhin
(régime du livre foncier) ni Mayotte : ces départements restent sans données.

## Navigation

| Touche | Action |
|---|---|
| `↑` `↓` / `k` `j` | Naviguer dans une liste ou une carte, se déplacer dans le tableau des ventes |
| souris | Survoler pour mettre en évidence, cliquer pour sélectionner ou ouvrir, molette pour défiler |
| `↵` / `→` | Ouvrir (une zone, un millésime, la fiche de la vente sélectionnée) |
| `esc` / `←` | Retour (annule un téléchargement en cours) |
| `tab` / `shift+tab` | Changer d'onglet |
| `/` | Filtrer la liste des communes (carte d'un département) ; ailleurs sur la carte, rechercher une adresse |
| `A` | Rechercher une adresse, à tous les niveaux de la carte |
| `a` | Biens actuellement en vente (depuis la carte) |
| `s` | Changer le tri du tableau des ventes (date, prix, €/m², surface) |
| `n` / `p` | Vente suivante / précédente depuis une fiche |
| `o` | Ouvrir l'annonce sélectionnée dans le navigateur (onglet En vente, écran des annonces) |
| `v` `c` `V` | Voir le bien dans le navigateur : carte avec repère, cadastre, vue de la rue |
| `pgup` `pgdn` / `espace` | Défilement rapide |
| `r` | Forcer le retéléchargement d'un département, relancer une recherche d'annonces |
| `i` | Inverser le sens du tri (écran des annonces) |
| `f` | Revenir aux critères depuis les résultats d'annonces |
| `l` | Collection locale |
| `q` | Quitter |

Sur la carte d'un département, `/` ouvre un filtre : chaque lettre tapée réduit
la liste des communes, sans tenir compte des accents, de la casse ni des tirets
(`saint jean` trouve Saint-Jean-de-Luz). Un code INSEE marche aussi. Les noms
qui commencent par la saisie passent devant ceux qui la contiennent. `↑` `↓`
parcourent la liste réduite, `↵` ouvre la commune, `esc` efface le filtre. La
carte, elle, montre toujours tout le département, avec la commune sélectionnée.
Le filtre reste posé au retour sur la carte : un premier `esc` l'efface, le
suivant remonte aux départements.

Dans les onglets Maisons, Appartements et Terrains, `↑` `↓` déplacent la ligne
sélectionnée du tableau des ventes et le contenu défile pour la suivre ; `pgup`
`pgdn` défilent librement sans changer la sélection.

## Lecture de la carte

Chaque cellule du terminal porte deux pixels grâce au demi-bloc `▀` : la couleur
d'avant-plan peint la moitié haute, celle de fond la moitié basse. Les couleurs
sont attribuées par coloriage d'adjacence — deux zones voisines n'ont jamais la
même. En vert : les départements déjà téléchargés. En jaune : la zone
sélectionnée.

## Statistiques calculées

Les indicateurs ont été choisis pour répondre aux questions que se pose
concrètement un acheteur.

### Onglet Résumé

- **Volume de transactions** par type de bien — un marché à 15 ventes par an ne
  se négocie pas comme un marché à 150.
- **Prix médian et prix au m² médian** par type.
- **Activité mois par mois** — révèle la saisonnalité et les périodes où l'offre
  se libère.
- **Voies les plus actives** avec leur prix médian — repère les micro-marchés
  (lotissements récents, quartiers qui tournent).

### Onglets Maisons / Appartements

- **Prix médian** mis en avant plutôt que la moyenne : il résiste aux ventes
  atypiques (biens d'exception, ventes entre proches) qui tirent la moyenne.
- **Cœur du marché (Q1 → Q3)** : la fourchette où se situent 50 % des ventes.
  C'est l'indicateur le plus directement actionnable pour caler un budget.
- **Extrêmes** min/max pour mesurer l'étendue réelle.
- **Prix au m² médian**, avec sa dispersion Q1–Q3.
- **Surface bâtie et terrain médians**, avec dispersion.
- **Tableau par tranche de surface** : nombre de ventes, prix médian et €/m²
  médian par palier. Permet de voir où le €/m² décroche et de cibler la surface
  offrant le meilleur rapport.
- **Tableau par nombre de pièces**, même logique.

### Tableau des ventes (onglets Maisons, Appartements, Terrains)

Sous les statistiques agrégées, **chaque vente** du type de bien est listée,
triée par date décroissante par défaut :

| Colonne | Lecture |
|---|---|
| Date | date de la mutation |
| Prix | valeur foncière |
| Surface | surface bâtie (surface de terrain pour un terrain nu) |
| P. | pièces principales |
| €/m² | prix au m² de cette vente |
| vs méd. | écart de ce €/m² à la médiane du type — situe la vente dans son marché |
| Terrain | surface de terrain rattachée |
| Adresse | numéro et voie |

La touche `s` fait tourner le tri : date, prix, €/m², surface. La colonne
« vs méd. » est ce qui rend le tableau lisible d'un coup d'œil : une vente à
−30 % du €/m² médian mérite qu'on ouvre sa fiche, une vente à +45 % aussi, mais
pas pour la même raison.

### Fiche d'une vente

`↵` sur une ligne (ou un second clic) ouvre le détail complet de la mutation.
**Aucune colonne du fichier DVF n'y est perdue** :

- **Prix** — valeur foncière, prix au m² bâti et au m² de terrain, et l'**écart
  au prix médian et au €/m² médian** de la commune pour ce type de bien.
- **Bien** — type de local et son code, surface réelle bâtie, pièces
  principales, surface de terrain, nombre de lots, lots de copropriété avec
  leur surface Carrez, numéro de volume, et la liste des locaux de la mutation
  lorsqu'elle en compte plusieurs (dépendances, annexes).
- **Adresse** — numéro, suffixe, voie, code voie FANTOIR, code postal, commune
  et son code INSEE, département, ancienne commune le cas échéant, et les
  coordonnées géographiques.
- **Mutation** — identifiant, date, nature, numéro de disposition, nombre de
  lignes dans le fichier.
- **Parcelles et natures de culture** — chaque subdivision cadastrale avec sa
  surface, et leur total : c'est la décomposition exacte de la surface de
  terrain affichée ailleurs.
- **Lignes brutes** — les lignes du CSV telles quelles. Dernier recours quand un
  chiffre surprend : on voit alors ce que la consolidation a fait.

`n` et `p` passent à la vente suivante ou précédente sans repasser par le
tableau.

**Voir le bien dans le navigateur** — depuis la fiche comme depuis la ligne
sélectionnée du tableau. Trois vues, parce qu'elles ne répondent pas à la même
question :

| Touche | Vue | Ce qu'elle apporte |
|---|---|---|
| `v` | Carte (Google Maps) | un **repère** posé sur le point du bien |
| `c` | Photo aérienne + cadastre (Géoportail) | les **limites de la parcelle vendue**, dessinées |
| `V` | Vue de la rue (Street View) | la façade — mais le panorama le plus proche, sans repère |

L'ordre est celui dans lequel on identifie un bien : *où est-ce*, *quelle
parcelle*, *à quoi ça ressemble*. La vue de la rue seule ne suffit pas — elle
dépose sur le panorama le plus proche, orienté au hasard, sans dire lequel des
bâtiments est le bon. La carte et surtout le cadastre lèvent l'ambiguïté ; la
vue de la rue vient ensuite.

Les liens sont construits sur les **coordonnées de la parcelle** quand DVF les
donne (le cas général), plus fiables qu'une recherche sur « 12 AV DU PORT », qui
retombe parfois sur une autre commune. Sans coordonnées, la fiche le signale et
se rabat sur l'adresse ; le cadastre, qui n'a pas de sens sans point, disparaît
alors de la liste.

Sur une session sans environnement graphique, l'échec du lanceur système est
signalé et **le lien complet est affiché**, donc copiable.

Ces touches envoient l'adresse ou les coordonnées du bien à Google Maps ou au
Géoportail ; ce sont les seules actions du programme qui sortent des sources
listées plus haut.

### Choix de la période

La liste des millésimes s'ouvre sur une entrée **« Toutes les années »**, qui
fusionne tous les millésimes disponibles de la commune. La période est un choix
au même titre qu'un millésime : elle se fait sur cet écran, et les onglets qui
suivent restent sur le seul axe du type de bien — quel que soit le choix, ils
décrivent le jeu de données retenu sans savoir lequel c'est.

Fusionner n'est pas un confort d'affichage. Sur une commune moyenne, une année
compte quelques dizaines de ventes par type de bien, et une médiane par tranche
de surface y porte alors sur trois transactions. Cumulés, cinq ou six millésimes
donnent enfin des effectifs exploitables — au prix d'une hypothèse : le marché
est supposé comparable sur toute la période.

Quand la période est fusionnée, l'onglet Résumé ajoute donc les tableaux
d'**évolution millésime par millésime** (maisons, appartements, terrains nus) :
nombre de ventes, prix médian, €/m² médian, surface médiane et le volume en
barres. C'est la seule statistique du programme qui réponde à « le marché
monte-t-il ? », et c'est aussi ce qui permet de vérifier l'hypothèse que la
fusion suppose.

La **tendance** entre le premier et le dernier millésime n'est affichée que si
les deux années comparées comptent au moins cinq ventes. En dessous, l'écart
mesure quels biens sont passés cette année-là, pas le marché : le programme le
dit plutôt que d'afficher un chiffre faux.

### Recherche d'adresse et page d'une adresse

`/` depuis la carte ouvre un champ de saisie. La requête part à la Base Adresse
Nationale ; `↑` `↓` choisissent parmi les résultats et `↵` ouvre l'adresse. Le
code INSEE du résultat donne le département, dont les millésimes sont
téléchargés au besoin, puis la commune est extraite et **tous ses millésimes
sont agrégés** — un logement se vendant une fois par décennie, une page
d'adresse sur un seul millésime n'aurait aucun sens.

L'onglet **Adresse** qui s'ajoute alors en tête présente :

- l'**adresse recherchée** : libellé, commune et code INSEE, département,
  coordonnées, période agrégée ;
- **à cette adresse** : les ventes portant exactement ce numéro dans cette voie,
  sur toute la période. L'absence de vente est le cas normal et le programme le
  dit clairement ;
- **la voie** : synthèse par type de bien des ventes de la voie, avec l'écart de
  son €/m² médian à celui de la commune — c'est ce qui distingue une rue chère
  d'une rue ordinaire ;
- **la commune**, tous millésimes agrégés — les autres onglets décrivent eux
  aussi la période complète, une page d'adresse n'ayant aucun sens sur un seul
  millésime ;
- le **tableau des ventes de la voie**, d'où l'on ouvre la fiche complète de
  n'importe laquelle. Le curseur se pose d'emblée sur une vente au numéro
  cherché s'il en existe une.

Les libellés de voie sont rapprochés malgré leurs écritures différentes : DVF
écrit `AV DU PORT`, la BAN « Avenue du Port ». Les deux sont ramenés à une forme
unique (majuscules sans accents, type de voie développé, particules retirées).
Si la voie n'apparaît dans aucune vente, le tableau porte sur la commune entière
et le dit.

### Onglet Terrains

Prix médian et **prix au m² de terrain nu**, repère pour un projet de
construction. La dispersion Q1–Q3 est affichée car cette statistique mélange
terrains constructibles et parcelles agricoles ou boisées.

### Onglet En vente

Il confronte les biens anciens actuellement en vente dans la commune aux ventes
DVF de la période choisie. Les annonces ne sont demandées au portail qu'à la
première ouverture de l'onglet, puis gardées tant qu'on reste sur la commune.
La commune est retrouvée par son **code INSEE** et non par son nom : il existe
une vingtaine de Saint-Martin.

Comparer la médiane des annonces à celle des ventes mesurerait surtout la
composition des deux échantillons. Si l'offre du moment compte surtout de
grandes maisons, la médiane des annonces est plus haute sans que le marché ait
bougé. **Chaque annonce est donc comparée à ses propres ventes similaires**, et
on n'agrège qu'ensuite les écarts :

| | Critère strict | Critère élargi (marqué `≈`) |
|---|---|---|
| Type | identique | identique |
| Surface (habitable, ou terrain pour un terrain nu) | ±15 % | ±25 % |
| Pièces (6 et plus confondues) | identique | ±1 |
| Terrain d'une maison | entre ½ et 2 fois | entre ⅓ et 3 fois |
| Ventes requises | 5 | 5 |

Un champ que l'annonce ne renseigne pas ne contraint rien. Une annonce sans
surface, ou avec moins de 5 ventes similaires même au critère élargi, n'est pas
comparée et l'onglet le dit. L'estimation vaut le €/m² médian des ventes
similaires multiplié par la surface de l'annonce.

Trois corrections précèdent le calcul :

- **le prix de l'acte**. Quand l'acheteur paie l'agence, le prix affiché inclut
  les honoraires (3 à 7 %), mais l'acte, donc DVF, ne porte que le net vendeur.
  On compare le net vendeur (`priceWithoutFees` chez Bien'ici) ;
- **les viagers sont écartés** : leur prix n'est que le bouquet ;
- **les doublons sont regroupés**. Un même bien confié à plusieurs agences
  apparaît sous plusieurs identifiants. Même type, prix, surface, terrain et
  nombre de pièces : il ne compte qu'une fois.

L'onglet présente une synthèse par type (€/m² demandé, €/m² vendu des
similaires, écart médian, part des annonces plus chères que leurs similaires),
puis chaque annonce, de la plus avantageuse à la plus chère. Les annonces sans
comparaison viennent ensuite, marquées `–`. `↑` `↓` parcourent les annonces de
tous les types, et `↵` ou `o` ouvre celle qui est sélectionnée dans le
navigateur. Un clic sélectionne une annonce, un second clic l'ouvre. Deux
limites restent :
- un prix demandé se négocie, donc quelques pour cent au-dessus des ventes est
  normal ;
- sur une période de plusieurs millésimes, l'écart inclut aussi l'évolution des
  prix depuis.

## Biens en vente

DVF dit ce qui **s'est vendu** ; cet écran dit ce qui **est à vendre**. Les deux
ne se mélangent jamais dans un même tableau : une mutation DVF est un prix payé
et constaté par un notaire, une annonce est un prix demandé par un vendeur,
révisable et parfois jamais atteint. L'onglet En vente des statistiques les
confronte, annonce par annonce, sans les fusionner.

### Critères

| Critère | Portée |
|---|---|
| Zone | région, département ou commune — plusieurs zones se cumulent |
| Rayon max | en kilomètres autour du centre de la première zone retenue |
| Types | maisons, appartements, terrains nus (cochables indépendamment) |
| Prix | minimum et maximum, en euros |
| Surface | surface habitable, minimum et maximum |
| Terrain | surface de terrain, minimum et maximum |
| Pièces min | nombre de pièces principales |
| Exclure le neuf | écarte les programmes et les biens neufs |
| Nombre max de résultats | borne la pagination (200 par défaut) |

Un champ vide vaut « pas de contrainte ». Les régions ne figurent pas dans
l'autocomplétion du portail : elles sont traduites en l'**union de leurs
départements**, à partir des contours déjà embarqués dans le binaire.

### Tri

`s` fait défiler le critère (prix, €/m², surface, terrain, date), `i` inverse le
sens. Le tri est instantané : les annonces restent en mémoire, aucune requête
n'est relancée. Les valeurs absentes partent toujours en fin de liste, quel que
soit le sens — un tri croissant par surface qui commencerait par les annonces
sans surface serait inutilisable.

Le tri par défaut est le prix croissant.

### Ce que le tableau n'est pas

- **Un échantillon, pas le marché.** Le portail annonce souvent plusieurs
  milliers de résultats ; seules les premières centaines sont rapportées. Trier
  par €/m² classe cet échantillon, pas le département. La ligne sous le tableau
  rappelle le rapport entre les deux.
- **Des prix demandés.** À comparer aux ventes DVF, jamais à confondre avec
  elles.
- **Des positions floutées.** Les portails ne publient pas l'adresse exacte : la
  position est celle d'un quartier ou d'une commune. Elle suffit pour un rayon de
  quelques kilomètres, pas pour désigner une parcelle.

### Modularité de la source

Un portail est décrit par l'interface `SourceAnnonces` (nommer, résoudre une
zone, chercher) et s'enregistre au démarrage. Le reste du programme ne connaît
que `Criteres` et `Annonce` : ajouter un portail est un fichier, en retirer un
est une suppression de fichier. Une source en panne n'annule pas les autres —
son erreur est affichée sous le tableau, les résultats des autres restent
exploitables.

Les filtres envoyés au portail sont réappliqués localement au retour. Ce n'est
pas une redondance : Bien'ici accepte des bornes de surface de terrain qu'il
n'applique pas, sans le moindre signal d'erreur. Le rayon, lui, n'est exposé par
aucun portail testé et est entièrement calculé localement.

### Pourquoi pas leboncoin

`www.leboncoin.fr` et `api.leboncoin.fr` répondent **403** à toute requête qui ne
vient pas d'un navigateur, avec une redirection vers un service anti-robot. Et
leur `robots.txt` commence par :

> It's forbidden to use search robots or other automatic methods to access
> Leboncoin.fr. Access is only permitted with special permission from
> Leboncoin.fr.

Ce n'est pas un obstacle technique à contourner, c'est un refus explicite.

## Fiabilité des données

Le programme écarte automatiquement les mutations non exploitables :

- nature de mutation autre que « Vente » (échanges, expropriations) ;
- valeur foncière absente ou nulle ;
- **ventes globales** portant sur plusieurs biens distincts (deux maisons, ou une
  maison + un appartement vendus ensemble) : le prix ne peut pas être rattaché à
  un bien unique et fausserait tous les ratios.

Un même bien réparti sur **plusieurs parcelles cadastrales** n'est pas considéré
comme une anomalie — c'est un cas fréquent et normal. De même, la surface de
terrain somme les subdivisions distinctes d'une parcelle (identifiées par nature
de culture) sans les compter en double, car chaque subdivision est répétée pour
chaque local de la mutation.

Le nombre de mutations écartées est affiché dans l'onglet Résumé.

## Limite importante

Les valeurs foncières DVF sont des **prix nets vendeur** : elles n'incluent pas
les frais de notaire (≈ 7–8 % dans l'ancien, 2–3 % dans le neuf), ni le mobilier.
Les frais d'agence n'y figurent que s'ils sont à la charge du vendeur.
