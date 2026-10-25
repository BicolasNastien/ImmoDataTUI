package main

// Évolution millésime par millésime, affichée quand la période choisie les
// fusionne tous.
//
// Une année de DVF sur une commune moyenne, c'est quelques dizaines de ventes
// par type de bien. Découpée par tranche de surface ou par nombre de pièces,
// une médiane porte alors sur trois transactions et ne veut plus rien dire.
// Fusionner les millésimes redonne des effectifs — et permet la seule question
// qu'une année isolée ne peut pas poser : le marché monte-t-il ?

import (
	"fmt"
	"strings"
)

// effectifPlancher : en dessous, une médiane annuelle décrit les quelques biens
// qui sont passés cette année-là, pas un marché.
const effectifPlancher = 5

// evolution rend un tableau année par année pour un type de bien.
func evolution(b *strings.Builder, titre string, lignes []LigneAnnee, terrain bool) {
	if len(lignes) < 1 {
		return
	}
	b.WriteString("\n  " + styleSection.Render(titre) + "\n\n")

	libSurface := "Surface méd."
	if terrain {
		libSurface = "Terrain méd."
	}
	b.WriteString(styleDim.Render(fmt.Sprintf("  %-8s %7s %14s %15s %14s   %s",
		"Année", "Ventes", "Prix médian", "€/m² médian", libSurface, "Volume")) + "\n")

	maxNb := 0
	for _, l := range lignes {
		if l.Nb > maxNb {
			maxNb = l.Nb
		}
	}
	for _, l := range lignes {
		barre := ""
		if maxNb > 0 {
			barre = strings.Repeat("█", l.Nb*18/maxNb)
		}
		b.WriteString(fmt.Sprintf("  %-8s %7d %14s %15s %14s   %s\n",
			l.Annee, l.Nb, euros(l.PrixMed), eurosM2(l.PrixM2Med), m2(l.SurfMed),
			styleDim.Render(barre)))
	}

	// Tendance : évolution du €/m² médian entre le premier et le dernier
	// millésime. Deux médianes ne font pas une série temporelle — et sur cinq
	// ventes par an, l'écart mesure surtout quels biens sont passés cette
	// année-là. On ne l'affiche donc qu'au-delà d'un effectif plancher, et on
	// dit pourquoi quand on s'abstient : une tendance absente vaut mieux qu'une
	// tendance fausse.
	prem, dern := lignes[0], lignes[len(lignes)-1]
	if len(lignes) < 2 || prem.PrixM2Med <= 0 || dern.PrixM2Med <= 0 {
		return
	}
	if prem.Nb < effectifPlancher || dern.Nb < effectifPlancher {
		b.WriteString("  " + styleDim.Render(fmt.Sprintf(
			"pas de tendance affichée : %s et %s comptent %d et %d ventes, trop peu pour comparer",
			prem.Annee, dern.Annee, prem.Nb, dern.Nb)) + "\n")
		return
	}
	ecart, _ := EcartRelatif(dern.PrixM2Med, prem.PrixM2Med)
	annees := float64(len(lignes) - 1)
	b.WriteString("  " + styleDim.Render(fmt.Sprintf(
		"tendance %s → %s : %s au total, soit %s par an en moyenne (%d ventes cumulées)",
		prem.Annee, dern.Annee, pourcent(ecart, true), pourcent(ecart/annees, true),
		totalVentes(lignes))) + "\n")
}

func totalVentes(lignes []LigneAnnee) int {
	n := 0
	for _, l := range lignes {
		n += l.Nb
	}
	return n
}

// ecrireEvolution ajoute les tableaux d'évolution au résumé, quand le jeu
// affiché fusionne plusieurs millésimes. C'est la seule statistique du
// programme qui réponde à « le marché monte-t-il ? » : aucune médiane calculée
// sur une seule année ne peut le dire.
func (m *model) ecrireEvolution(b *strings.Builder) {
	if len(m.parAnnee) < 2 {
		return
	}
	evolution(b, "ÉVOLUTION — MAISONS", EvolutionParAnnee(m.parAnnee, "Maison"), false)
	evolution(b, "ÉVOLUTION — APPARTEMENTS", EvolutionParAnnee(m.parAnnee, "Appartement"), false)
	evolution(b, "ÉVOLUTION — TERRAINS NUS", EvolutionParAnnee(m.parAnnee, "Terrain"), true)

	b.WriteString("\n  " + styleDim.Render(
		"Fusionner les millésimes suppose le marché comparable sur la période :") + "\n")
	b.WriteString("  " + styleDim.Render(
		"les tableaux ci-dessus disent si cette hypothèse tient.") + "\n")
}
