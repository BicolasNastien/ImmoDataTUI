package main

// ---------------------------------------------------------------------------
// Ce qui est à vendre, comparé à ce qui s'est vendu
// ---------------------------------------------------------------------------
//
// Comparer la médiane des annonces d'une commune à la médiane de ses ventes
// DVF ne dit presque rien : si les annonces du moment sont surtout de grandes
// maisons et les ventes passées surtout des petites, l'écart mesure la
// composition des deux échantillons, pas le marché. C'est le paradoxe de
// Simpson appliqué à l'immobilier.
//
// On compare donc chaque annonce à ses propres ventes similaires — même type,
// surface proche, même nombre de pièces, terrain comparable — et on n'agrège
// qu'ensuite les écarts individuels. Une annonce sans assez de ventes
// similaires n'est pas comparée : on le dit, plutôt que de lui inventer une
// référence.

import (
	"fmt"
	"sort"
	"strings"
)

// NiveauSimilarite dit avec quelle exigence les ventes de référence ont été
// choisies.
type NiveauSimilarite int

const (
	SansReference NiveauSimilarite = iota
	SimilariteElargie
	SimilariteStricte
)

// critereSimilarite : ce qu'on tolère d'écart entre une annonce et une vente
// pour les dire comparables.
type critereSimilarite struct {
	niveau         NiveauSimilarite
	surface        float64 // écart relatif toléré sur la surface de référence
	pieces         int     // écart toléré sur le nombre de pièces
	terrainFacteur float64 // terrain entre 1/f et f fois celui de l'annonce
}

// Du plus strict au plus large. On ne passe au cran suivant que faute de
// ventes : élargir dégrade la comparaison, et l'affichage le signale.
var criteresSimilarite = []critereSimilarite{
	{niveau: SimilariteStricte, surface: 0.15, pieces: 0, terrainFacteur: 2},
	{niveau: SimilariteElargie, surface: 0.25, pieces: 1, terrainFacteur: 3},
}

// minSimilaires : en dessous, une médiane décrit les quelques biens passés,
// pas un prix de marché. C'est le même plancher que pour les tendances
// annuelles.
const minSimilaires = effectifPlancher

// piecesPlafond : au-delà, les annonces comme DVF comptent les pièces de
// manière trop hétérogène (combles, bureaux, vérandas) pour qu'un écart d'une
// pièce ait un sens. Comme dans le tableau par nombre de pièces.
const piecesPlafond = 6

// Comparaison rapproche une annonce des ventes qui lui ressemblent.
type Comparaison struct {
	Annonce    Annonce
	Niveau     NiveauSimilarite
	Similaires []Sale

	PrixM2Vendu float64 // médiane des ventes similaires, au m² de référence
	Q1, Q3      float64 // dispersion de ces mêmes ventes
	Estime      float64 // ce que vaudrait l'annonce au prix médian des similaires
	Ecart       float64 // prix demandé contre estimation, en %
}

// Comparable : une référence existe.
func (c Comparaison) Comparable() bool { return c.Niveau != SansReference }

// typeDVF traduit la nomenclature des annonces vers celle de DVF.
func typeDVF(t TypeBien) string {
	switch t {
	case BienMaison:
		return "Maison"
	case BienAppartement:
		return "Appartement"
	case BienTerrain:
		return "Terrain"
	}
	return ""
}

// surfaceAnnonce : la surface qui fait le prix — habitable pour un bâti,
// celle du terrain pour un terrain nu. La même règle que Sale.SurfaceRef.
func surfaceAnnonce(a Annonce) float64 {
	if a.Type == BienTerrain {
		return a.SurfaceTerrain
	}
	return a.Surface
}

// Comparer cherche les ventes similaires à une annonce, du critère le plus
// strict au plus large.
func Comparer(a Annonce, ventes []Sale) Comparaison {
	out := Comparaison{Annonce: a}
	t, surf := typeDVF(a.Type), surfaceAnnonce(a)
	if t == "" || surf <= 0 || a.PrixCompare() <= 0 || a.Viager {
		return out
	}

	for _, c := range criteresSimilarite {
		var sim []Sale
		for _, s := range ventes {
			if s.Type == t && similaire(a, s, c) {
				sim = append(sim, s)
			}
		}
		if len(sim) < minSimilaires {
			continue
		}

		pm2 := make([]float64, 0, len(sim))
		for _, s := range sim {
			pm2 = append(pm2, s.PrixM2Ref())
		}
		out.Niveau, out.Similaires = c.niveau, sim
		out.PrixM2Vendu = mediane(pm2)
		out.Q1, out.Q3 = quantile(pm2, 0.25), quantile(pm2, 0.75)
		out.Estime = out.PrixM2Vendu * surf
		out.Ecart, _ = EcartRelatif(a.PrixCompare(), out.Estime)
		return out
	}
	return out
}

// similaire applique un critère. Un champ que l'annonce tait (zéro) ne
// contraint rien : mieux vaut une comparaison un peu plus large qu'aucune.
// Un champ que l'annonce renseigne doit en revanche être connu de la vente.
func similaire(a Annonce, s Sale, c critereSimilarite) bool {
	surfA, surfS := surfaceAnnonce(a), s.SurfaceRef()
	if surfS <= 0 || s.PrixM2Ref() <= 0 {
		return false
	}
	if d := surfS/surfA - 1; d < -c.surface || d > c.surface {
		return false
	}

	if a.Type == BienTerrain {
		return true // la surface du terrain est déjà la surface de référence
	}

	if a.Pieces > 0 {
		pa, ps := plafonner(a.Pieces), plafonner(s.Pieces)
		if s.Pieces <= 0 || ecartEntier(pa, ps) > c.pieces {
			return false
		}
	}

	// Le terrain d'une maison pèse lourd dans son prix : 90 m² sur 2 000 m² de
	// jardin et 90 m² en ville ne sont pas le même bien. Un appartement n'en a
	// pas, ou n'en déclare qu'une quote-part de copropriété sans intérêt ici.
	if a.Type == BienMaison && a.SurfaceTerrain > 0 {
		if s.SurfaceTerrain <= 0 {
			return false
		}
		r := s.SurfaceTerrain / a.SurfaceTerrain
		if r < 1/c.terrainFacteur || r > c.terrainFacteur {
			return false
		}
	}
	return true
}

func plafonner(p int) int {
	if p > piecesPlafond {
		return piecesPlafond
	}
	return p
}

func ecartEntier(a, b int) int {
	if a > b {
		return a - b
	}
	return b - a
}

// ComparerTout compare chaque annonce, et classe le résultat : les annonces
// comparables d'abord, de la moins chère à la plus chère relativement à leurs
// similaires ; les autres ensuite, par prix.
func ComparerTout(annonces []Annonce, ventes []Sale) []Comparaison {
	out := make([]Comparaison, 0, len(annonces))
	for _, a := range annonces {
		out = append(out, Comparer(a, ventes))
	}
	sort.SliceStable(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if a.Comparable() != b.Comparable() {
			return a.Comparable()
		}
		if a.Comparable() && a.Ecart != b.Ecart {
			return a.Ecart < b.Ecart
		}
		if a.Annonce.PrixCompare() != b.Annonce.PrixCompare() {
			return a.Annonce.PrixCompare() < b.Annonce.PrixCompare()
		}
		return a.Annonce.ID < b.Annonce.ID
	})
	return out
}

// PreparerMarche écarte ce qui fausserait la comparaison avant tout calcul.
//
// Les viagers : leur prix n'est qu'un bouquet. Les doublons : un même bien
// confié à plusieurs agences paraît plusieurs fois sur le même portail, sous
// des identifiants différents, et compterait double dans les médianes. On ne
// peut pas le prouver, mais même type, même prix, même surface, même terrain et
// même nombre de pièces, au mètre carré près, c'est presque toujours le même
// bien. FiltrerAnnonces ne le fait pas, et ne doit pas le faire : dans une
// liste de résultats, on veut voir les deux agences.
func PreparerMarche(as []Annonce) (gardees []Annonce, viagers, doublons int) {
	vues := map[string]bool{}
	for _, a := range as {
		if a.Viager {
			viagers++
			continue
		}
		cle := fmt.Sprintf("%d|%.0f|%.0f|%.0f|%d", a.Type, a.PrixCompare(), a.Surface, a.SurfaceTerrain, a.Pieces)
		if vues[cle] {
			doublons++
			continue
		}
		vues[cle] = true
		gardees = append(gardees, a)
	}
	return gardees, viagers, doublons
}

// SyntheseMarche résume la comparaison pour un type de bien.
type SyntheseMarche struct {
	Type        TypeBien
	Annonces    int
	Comparees   int
	Elargies    int     // comparées au critère large seulement
	EcartMed    float64 // médiane des écarts individuels
	EcartQ1     float64
	EcartQ3     float64
	AuDessus    int     // annonces plus chères que leurs similaires
	PrixM2Ann   float64 // médiane du €/m² demandé, annonces comparées
	PrixM2Vendu float64 // médiane des références de ces mêmes annonces
}

// Synthetiser agrège des comparaisons par type de bien, dans l'ordre de
// TousTypes. Les médianes ne portent que sur les annonces comparées : mêler
// les autres réintroduirait l'effet de composition qu'on vient d'écarter.
func Synthetiser(cs []Comparaison) []SyntheseMarche {
	var out []SyntheseMarche
	for _, t := range TousTypes() {
		s := SyntheseMarche{Type: t}
		var ecarts, demandes, vendus []float64
		for _, c := range cs {
			if c.Annonce.Type != t {
				continue
			}
			s.Annonces++
			if !c.Comparable() {
				continue
			}
			s.Comparees++
			if c.Niveau == SimilariteElargie {
				s.Elargies++
			}
			if c.Ecart > 0 {
				s.AuDessus++
			}
			ecarts = append(ecarts, c.Ecart)
			demandes = append(demandes, c.Annonce.PrixCompare()/surfaceAnnonce(c.Annonce))
			vendus = append(vendus, c.PrixM2Vendu)
		}
		if s.Annonces == 0 {
			continue
		}
		if s.Comparees > 0 {
			s.EcartMed = mediane(ecarts)
			s.EcartQ1, s.EcartQ3 = quantile(ecarts, 0.25), quantile(ecarts, 0.75)
			s.PrixM2Ann, s.PrixM2Vendu = mediane(demandes), mediane(vendus)
		}
		out = append(out, s)
	}
	return out
}

// CommuneDominante renvoie le code INSEE et le nom qui reviennent le plus dans
// un jeu de ventes. C'est le pont vers les annonces, valable aussi pour une
// collection déposée à la main, dont on ne connaît que le nom de dossier.
func CommuneDominante(ventes []Sale) (code, nom string) {
	compte := map[string]int{}
	noms := map[string]string{}
	for _, s := range ventes {
		if len(s.Lignes) == 0 {
			continue
		}
		c := strings.TrimSpace(s.Lignes[0].CodeCommune)
		if c == "" {
			continue
		}
		compte[c]++
		if noms[c] == "" {
			noms[c] = strings.TrimSpace(s.Commune)
		}
	}
	for c, n := range compte {
		if n > compte[code] || (n == compte[code] && c < code) {
			code = c
		}
	}
	return code, noms[code]
}
