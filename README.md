# ImmoDataTUI

Explorer les ventes immobilières françaises depuis le terminal, pour **décider
d'un achat** : ce qui s'est vendu, à quel prix, dans quelle rue — et ce qui est
à vendre aujourd'hui, comparé à ces ventes.

Les données sont celles de **DVF** (Demandes de valeurs foncières), publiées
par la DGFiP : toutes les ventes immobilières de France depuis six ans, avec
leur prix, leurs surfaces et leur parcelle. ImmoDataTUI les télécharge à la
demande et les met en cache.

```
carte de France ──▶ région ──▶ département ──▶ commune ──▶ statistiques ──▶ fiche d'une vente
                                                              │
                                    « / » adresse ────────────┤
                                    « a » biens en vente ─────┘
```

## Fonctionnalités

- **Carte interactive** de la France, des régions et des départements, au
  clavier ou à la souris.
- **Statistiques par commune** : prix médian, cœur du marché (Q1–Q3), €/m², par
  type de bien, par surface, par nombre de pièces, évolution d'une année sur
  l'autre.
- **Toutes les ventes**, triables, avec l'écart de chacune au prix médian, et
  leur **fiche complète** — jusqu'aux parcelles cadastrales.
- **Recherche d'adresse** : les ventes à ce numéro, dans la rue, dans la
  commune.
- **Voir le bien** sur une carte, sur le cadastre ou en vue de la rue.
- **Biens actuellement en vente** (Bien'ici), filtrables, et comparés un par
  un à leurs ventes similaires.

## Installation

**Linux et macOS**

```bash
curl -fsSL https://raw.githubusercontent.com/BicolasNastien/ImmoDataTUI/main/install.sh | sh
```

**Windows** (PowerShell)

```powershell
irm https://raw.githubusercontent.com/BicolasNastien/ImmoDataTUI/main/install.ps1 | iex
```

Puis lancer `immodata` dans un nouveau terminal. Les archives sont aussi
téléchargeables à la main sur la page
[Releases](https://github.com/BicolasNastien/ImmoDataTUI/releases/latest)
(Linux, macOS, Windows ; processeurs Intel/AMD et ARM).

Options des scripts, installation manuelle pas à pas et compilation depuis les
sources :
**[docs/installation.md](docs/installation.md)**.

## Documentation

| Page | Contenu |
|---|---|
| [Installation](docs/installation.md) | Linux, macOS, Windows, depuis les sources, mise à jour, désinstallation, dépannage |
| [Utilisation](docs/utilisation.md) | parcours, raccourcis clavier, statistiques, fiche d'une vente, adresses, biens en vente |
| [Sources de données](docs/sources.md) | DVF, BAN, contours, Bien'ici, emplacements du cache, fraîcheur, couverture, licences |

## Raccourcis essentiels

| Touche | Action |
|---|---|
| `↑` `↓` / `↵` / `esc` | se déplacer / ouvrir / revenir |
| `tab` | onglet suivant |
| `/` | filtrer les communes, ou rechercher une adresse |
| `a` | biens actuellement en vente |
| `s` | changer le tri |
| `v` `c` `V` | voir le bien : carte, cadastre, vue de la rue |
| `q` | quitter |

Tous les raccourcis : [docs/utilisation.md](docs/utilisation.md#raccourcis-clavier).

## À savoir

Les prix DVF sont des **prix nets vendeur**, hors frais de notaire. Les données
sont publiées deux fois par an avec plusieurs mois de retard, et ne couvrent ni
l'Alsace-Moselle ni Mayotte. Les annonces sont des **prix demandés**, jamais
mélangés aux ventes.
