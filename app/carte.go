package main

import (
	"fmt"
	"math"
	"sort"
	"strings"
)

// ---------------------------------------------------------------------------
// Rendu d'une carte en terminal
// ---------------------------------------------------------------------------
//
// Le terminal n'affiche pas de pixels, mais le demi-bloc « ▀ » en fournit deux
// par cellule : la couleur d'avant-plan peint la moitié haute, celle de fond la
// moitié basse. On raisonne donc sur une grille de L colonnes sur 2×H lignes,
// et une cellule de terminal étant à peu près deux fois plus haute que large,
// ces demi-cellules sont sensiblement carrées — la carte n'est pas déformée.
//
// Chaque pixel mémorise l'indice de la zone qui l'occupe. Cette grille sert à
// la fois au rendu et au pointage à la souris : un clic en (colonne, ligne) se
// résout en une zone par simple lecture, sans repasser par la géométrie.

const pixelVide = -1

// Palette de remplissage : des tons moyens, assez proches pour faire carte et
// assez distincts pour séparer deux zones voisines.
var paletteCarte = []int{109, 144, 174, 108, 139, 180, 110, 151}

const (
	couleurSelection = 220 // jaune vif : la zone sous le curseur
	couleurMarquee   = 42  // vert : données déjà téléchargées
	couleurBordure   = 236
	couleurEtiquette = 231
)

// Carte est une grille de pixels dont chacun porte l'indice de sa zone.
type Carte struct {
	Zones []Zone

	L, H     int // taille en cellules de terminal
	pix      []int16
	couleurs []int             // couleur de remplissage par zone
	surface  []int             // nombre de pixels par zone
	etiq     map[int]etiquette // par indice de cellule
	titrees  map[int]bool      // zones déjà nommées par la légende d'un cartouche
}

type etiquette struct {
	car  rune
	zone int
}

// bloc est une portion rectangulaire de la grille dotée de sa propre
// projection. La métropole en occupe un ; chaque territoire d'outre-mer a le
// sien, sans quoi l'échelle réelle réduirait la France hexagonale à rien.
type bloc struct {
	zones      []int
	x0, y0     int
	l, h       int
	titre      string
	ligneTitre bool
}

// NouvelleCarte projette et rastérise des zones sur une grille de l×h cellules.
// codesMarques désigne les zones à peindre comme « déjà téléchargées ».
func NouvelleCarte(zones []Zone, l, h int, insets bool, codesMarques map[string]bool) *Carte {
	if l < 4 {
		l = 4
	}
	if h < 3 {
		h = 3
	}

	c := &Carte{
		Zones:    zones,
		L:        l,
		H:        h,
		pix:      make([]int16, l*h*2),
		couleurs: make([]int, len(zones)),
		surface:  make([]int, len(zones)),
		etiq:     map[int]etiquette{},
		titrees:  map[int]bool{},
	}
	for i := range c.pix {
		c.pix[i] = pixelVide
	}

	for _, b := range c.decouper(insets) {
		c.rasteriserBloc(b)
	}
	c.recadrer()
	c.colorier(codesMarques)
	c.poserEtiquettes()
	return c
}

// recadrer supprime les colonnes vides de part et d'autre de la carte. Sans ça
// une région étroite laisse un large vide entre elle et la liste, et la carte
// paraît flotter.
func (c *Carte) recadrer() {
	gauche, droite := c.L, -1
	for y := 0; y < c.H*2; y++ {
		for x := 0; x < c.L; x++ {
			if c.pix[y*c.L+x] != pixelVide {
				if x < gauche {
					gauche = x
				}
				if x > droite {
					droite = x
				}
			}
		}
	}
	if droite < 0 || (gauche == 0 && droite == c.L-1) {
		return
	}

	nouvelleL := droite - gauche + 1
	pix := make([]int16, nouvelleL*c.H*2)
	for y := 0; y < c.H*2; y++ {
		copy(pix[y*nouvelleL:(y+1)*nouvelleL], c.pix[y*c.L+gauche:y*c.L+droite+1])
	}

	etiq := make(map[int]etiquette, len(c.etiq))
	for i, e := range c.etiq {
		ligne, col := i/c.L, i%c.L
		if col < gauche || col > droite {
			continue
		}
		etiq[ligne*nouvelleL+col-gauche] = e
	}

	c.pix, c.etiq, c.L = pix, etiq, nouvelleL
}

// decouper répartit les zones en blocs : un pour la métropole, un cartouche par
// territoire d'outre-mer si on en a demandé.
func (c *Carte) decouper(insets bool) []bloc {
	principal := []int{}
	var outreMer []int

	for i, z := range c.Zones {
		code := z.Code
		if len(code) == 3 && strings.HasPrefix(code, "97") { // département d'outre-mer
			outreMer = append(outreMer, i)
			continue
		}
		if EstOutreMer(code) && len(code) == 2 { // région d'outre-mer
			outreMer = append(outreMer, i)
			continue
		}
		principal = append(principal, i)
	}

	if !insets || len(outreMer) == 0 {
		return []bloc{{zones: append(principal, outreMer...), l: c.L, h: c.H * 2}}
	}
	if len(principal) == 0 {
		return []bloc{{zones: outreMer, l: c.L, h: c.H * 2}}
	}

	// Colonne de cartouches à droite, un par territoire, titre compris.
	largeurCol := c.L / 5
	if largeurCol < 10 {
		largeurCol = 10
	}
	if largeurCol > 16 {
		largeurCol = 16
	}
	if largeurCol > c.L/2 {
		largeurCol = c.L / 2
	}

	blocs := []bloc{{zones: principal, l: c.L - largeurCol - 1, h: c.H * 2}}

	hauteurCell := c.H / len(outreMer)
	if hauteurCell < 2 {
		hauteurCell = 2
	}
	x0 := c.L - largeurCol
	for n, idx := range outreMer {
		y0 := n * hauteurCell
		if y0+hauteurCell > c.H {
			break
		}
		blocs = append(blocs, bloc{
			zones:      []int{idx},
			x0:         x0,
			y0:         y0 * 2,
			l:          largeurCol,
			h:          (hauteurCell - 1) * 2,
			titre:      c.Zones[idx].Nom,
			ligneTitre: true,
		})
	}
	return blocs
}

// rasteriserBloc projette les zones d'un bloc puis les remplit.
func (c *Carte) rasteriserBloc(b bloc) {
	if len(b.zones) == 0 || b.l <= 0 || b.h <= 0 {
		return
	}

	var sousZones []Zone
	for _, i := range b.zones {
		sousZones = append(sousZones, c.Zones[i])
	}
	cadre := CadreDe(sousZones)
	if !cadre.valide() {
		return
	}

	// Projection équirectangulaire : à l'échelle d'une région les distorsions
	// sont invisibles, et le facteur cos(latitude) suffit à ne pas étirer la
	// France en largeur.
	_, latCentre := cadre.Centre()
	k := math.Cos(latCentre * math.Pi / 180)
	largeurMonde := (cadre.LonMax - cadre.LonMin) * k
	hauteurMonde := cadre.LatMax - cadre.LatMin

	echelle := math.Min(float64(b.l)/largeurMonde, float64(b.h)/hauteurMonde)
	decX := (float64(b.l) - largeurMonde*echelle) / 2
	decY := (float64(b.h) - hauteurMonde*echelle) / 2

	projeter := func(lon, lat float64) (float64, float64) {
		return float64(b.x0) + decX + (lon-cadre.LonMin)*k*echelle,
			float64(b.y0) + decY + (cadre.LatMax-lat)*echelle
	}

	// Les grandes zones d'abord : les petites, dessinées ensuite, ne peuvent
	// pas être recouvertes et gardent leurs quelques pixels.
	ordre := append([]int(nil), b.zones...)
	sort.Slice(ordre, func(i, j int) bool {
		return etendue(c.Zones[ordre[i]]) > etendue(c.Zones[ordre[j]])
	})

	for _, idx := range ordre {
		var anneauxPix [][]float64
		for _, anneau := range c.Zones[idx].Contours {
			p := make([]float64, len(anneau))
			for i := 0; i+1 < len(anneau); i += 2 {
				p[i], p[i+1] = projeter(anneau[i], anneau[i+1])
			}
			anneauxPix = append(anneauxPix, p)
		}
		c.remplir(anneauxPix, int16(idx), b)
		c.tracerContour(anneauxPix, int16(idx), b)
	}

	// Une commune plus petite qu'un pixel, ou entièrement recouverte par une
	// voisine dessinée après elle, serait dans la liste mais introuvable sur la
	// carte — et donc impossible à cliquer. On lui réserve un pixel près de son
	// centre, en cherchant d'abord un emplacement libre pour ne pas déloger une
	// autre commune à son tour.
	for _, idx := range ordre {
		if c.surface[idx] > 0 {
			continue
		}
		cadreZ := CadreDe([]Zone{c.Zones[idx]})
		lon, lat := cadreZ.Centre()
		x, y := projeter(lon, lat)
		c.poserAuPlusPres(int(math.Round(x)), int(math.Round(y)), int16(idx), b)
	}

	if b.ligneTitre && b.titre != "" {
		c.poserTitre(b)
	}
}

// etendue mesure grossièrement la taille d'une zone (diagonale de son cadre).
func etendue(z Zone) float64 {
	c := CadreDe([]Zone{z})
	if !c.valide() {
		return 0
	}
	return (c.LonMax - c.LonMin) * (c.LatMax - c.LatMin)
}

func (c *Carte) poser(x, y int, zone int16, b bloc) {
	if x < b.x0 || x >= b.x0+b.l || y < b.y0 || y >= b.y0+b.h {
		return
	}
	if x < 0 || x >= c.L || y < 0 || y >= c.H*2 {
		return
	}
	i := y*c.L + x
	if c.pix[i] != pixelVide {
		c.surface[c.pix[i]]--
	}
	c.pix[i] = zone
	c.surface[zone]++
}

// poserAuPlusPres place un pixel de zone au plus près d'un point, en préférant
// une case encore vide dans un petit voisinage.
func (c *Carte) poserAuPlusPres(x, y int, zone int16, b bloc) {
	for rayon := 0; rayon <= 3; rayon++ {
		for dy := -rayon; dy <= rayon; dy++ {
			for dx := -rayon; dx <= rayon; dx++ {
				px, py := x+dx, y+dy
				if px < b.x0 || px >= b.x0+b.l || py < b.y0 || py >= b.y0+b.h {
					continue
				}
				if px < 0 || px >= c.L || py < 0 || py >= c.H*2 {
					continue
				}
				if c.pix[py*c.L+px] == pixelVide {
					c.poser(px, py, zone, b)
					return
				}
			}
		}
	}
	c.poser(x, y, zone, b)
}

// remplir applique la règle pair-impair, ligne de balayage par ligne de
// balayage : les intersections avec les arêtes, triées, se consomment par
// paires. Tous les anneaux d'une zone sont traités ensemble, ce qui gère aussi
// bien les îles que les enclaves.
func (c *Carte) remplir(anneaux [][]float64, zone int16, b bloc) {
	yMin, yMax := math.Inf(1), math.Inf(-1)
	for _, a := range anneaux {
		for i := 1; i < len(a); i += 2 {
			yMin = math.Min(yMin, a[i])
			yMax = math.Max(yMax, a[i])
		}
	}
	if math.IsInf(yMin, 1) {
		return
	}

	debut := int(math.Floor(yMin))
	fin := int(math.Ceil(yMax))
	var croisements []float64

	for y := debut; y <= fin; y++ {
		yc := float64(y) + 0.5
		croisements = croisements[:0]

		for _, a := range anneaux {
			for i := 0; i+3 < len(a); i += 2 {
				x1, y1, x2, y2 := a[i], a[i+1], a[i+2], a[i+3]
				if (y1 <= yc) == (y2 <= yc) {
					continue
				}
				croisements = append(croisements, x1+(yc-y1)/(y2-y1)*(x2-x1))
			}
		}
		if len(croisements) < 2 {
			continue
		}
		sort.Float64s(croisements)
		for i := 0; i+1 < len(croisements); i += 2 {
			for x := int(math.Round(croisements[i])); x <= int(math.Round(croisements[i+1])); x++ {
				c.poser(x, y, zone, b)
			}
		}
	}
}

// tracerContour dessine les arêtes elles-mêmes : sans ça, une commune plus fine
// qu'un pixel disparaîtrait du remplissage.
func (c *Carte) tracerContour(anneaux [][]float64, zone int16, b bloc) {
	for _, a := range anneaux {
		for i := 0; i+3 < len(a); i += 2 {
			c.ligne(a[i], a[i+1], a[i+2], a[i+3], zone, b)
		}
	}
}

func (c *Carte) ligne(x1, y1, x2, y2 float64, zone int16, b bloc) {
	pas := math.Max(math.Abs(x2-x1), math.Abs(y2-y1))
	n := int(pas) + 1
	for i := 0; i <= n; i++ {
		t := float64(i) / float64(n)
		c.poser(int(math.Round(x1+(x2-x1)*t)), int(math.Round(y1+(y2-y1)*t)), zone, b)
	}
}

// ---------------------------------------------------------------------------
// Couleurs
// ---------------------------------------------------------------------------

// colorier attribue les couleurs de remplissage. L'adjacence est déduite de la
// grille rastérisée — deux zones sont voisines si deux de leurs pixels se
// touchent — puis coloriée gloutonnement en commençant par les zones les plus
// contraintes. C'est le théorème des quatre couleurs appliqué à la va-vite :
// avec huit teintes disponibles, deux voisines ne partagent jamais la leur.
func (c *Carte) colorier(codesMarques map[string]bool) {
	voisins := make([]map[int]bool, len(c.Zones))
	for i := range voisins {
		voisins[i] = map[int]bool{}
	}

	lier := func(a, b int16) {
		if a == pixelVide || b == pixelVide || a == b {
			return
		}
		voisins[a][int(b)] = true
		voisins[b][int(a)] = true
	}
	for y := 0; y < c.H*2; y++ {
		for x := 0; x < c.L; x++ {
			p := c.pix[y*c.L+x]
			if x+1 < c.L {
				lier(p, c.pix[y*c.L+x+1])
			}
			if y+1 < c.H*2 {
				lier(p, c.pix[(y+1)*c.L+x])
			}
		}
	}

	ordre := make([]int, len(c.Zones))
	for i := range ordre {
		ordre[i] = i
	}
	sort.Slice(ordre, func(i, j int) bool { return len(voisins[ordre[i]]) > len(voisins[ordre[j]]) })

	for i := range c.couleurs {
		c.couleurs[i] = -1
	}
	for _, z := range ordre {
		prises := map[int]bool{}
		for v := range voisins[z] {
			if c.couleurs[v] >= 0 {
				prises[c.couleurs[v]] = true
			}
		}
		choix := paletteCarte[0]
		for _, p := range paletteCarte {
			if !prises[p] {
				choix = p
				break
			}
		}
		c.couleurs[z] = choix
	}

	for i, z := range c.Zones {
		if codesMarques[z.Code] {
			c.couleurs[i] = couleurMarquee
		}
	}
}

// ---------------------------------------------------------------------------
// Étiquettes
// ---------------------------------------------------------------------------

// poserEtiquettes inscrit le nom des zones les plus grandes au centre de leurs
// pixels, tant qu'il y a la place. Les petites restent muettes : la liste
// latérale les nomme, et une carte surchargée d'étiquettes n'est plus une carte.
func (c *Carte) poserEtiquettes() {
	ordre := make([]int, len(c.Zones))
	for i := range ordre {
		ordre[i] = i
	}
	sort.Slice(ordre, func(i, j int) bool { return c.surface[ordre[i]] > c.surface[ordre[j]] })

	for _, z := range ordre {
		if c.surface[z] < 24 || c.titrees[z] {
			continue
		}
		col, ligne := c.centreCellule(z)
		if col < 0 {
			continue
		}
		for _, texte := range candidatsEtiquette(c.Zones[z].Nom) {
			if c.poserTexte(texte, col-len([]rune(texte))/2, ligne, z, true) {
				break
			}
		}
	}
}

// candidatsEtiquette propose le nom complet puis des replis plus courts.
// L'abréviation garde la première composante et l'initiale des suivantes :
// « Charente-Maritime » devient « Charente-M. » et reste distincte de
// « Charente », là où une simple troncature les confondrait.
func candidatsEtiquette(nom string) []string {
	out := []string{nom}

	if parties := strings.Split(nom, "-"); len(parties) > 1 {
		abrege := parties[0]
		for _, p := range parties[1:] {
			r := []rune(p)
			if len(r) == 0 {
				continue
			}
			abrege += "-" + string(r[0]) + "."
		}
		out = append(out, abrege)
	} else if i := strings.Index(nom, " "); i > 2 {
		out = append(out, nom[:i])
	}

	r := []rune(nom)
	if len(r) > 6 {
		out = append(out, string(r[:5])+".")
	}
	return out
}

// centreCellule renvoie la cellule au centre de masse des pixels d'une zone.
func (c *Carte) centreCellule(zone int) (int, int) {
	var sx, sy, n int
	for y := 0; y < c.H*2; y++ {
		for x := 0; x < c.L; x++ {
			if int(c.pix[y*c.L+x]) == zone {
				sx += x
				sy += y
				n++
			}
		}
	}
	if n == 0 {
		return -1, -1
	}
	return sx / n, sy / n / 2
}

// poserTexte écrit une étiquette si toutes ses cellules sont libres et, quand
// exigeSurZone est vrai, appartiennent bien à la zone.
func (c *Carte) poserTexte(texte string, col, ligne, zone int, exigeSurZone bool) bool {
	r := []rune(texte)
	if col < 0 || ligne < 0 || ligne >= c.H || col+len(r) > c.L {
		return false
	}
	for i := range r {
		idx := ligne*c.L + col + i
		if _, occupe := c.etiq[idx]; occupe {
			return false
		}
		if exigeSurZone {
			haut := c.pix[(ligne*2)*c.L+col+i]
			bas := c.pix[(ligne*2+1)*c.L+col+i]
			if int(haut) != zone && int(bas) != zone {
				return false
			}
		}
	}
	for i, car := range r {
		c.etiq[ligne*c.L+col+i] = etiquette{car: car, zone: zone}
	}
	return true
}

// poserTitre légende un cartouche d'outre-mer, sur la ligne laissée libre sous
// sa géométrie.
func (c *Carte) poserTitre(b bloc) {
	ligne := (b.y0 + b.h) / 2
	if ligne >= c.H {
		return
	}
	zone := b.zones[0]
	for _, texte := range candidatsEtiquette(b.titre) {
		if len([]rune(texte)) > b.l {
			continue
		}
		col := b.x0 + (b.l-len([]rune(texte)))/2
		if c.poserTexte(texte, col, ligne, zone, false) {
			c.titrees[zone] = true
			return
		}
	}
}

// ---------------------------------------------------------------------------
// Interrogation et rendu
// ---------------------------------------------------------------------------

// ZoneEn résout une cellule de terminal en indice de zone, ou -1. C'est tout ce
// qu'il faut pour cliquer sur une carte.
func (c *Carte) ZoneEn(col, ligne int) int {
	if col < 0 || col >= c.L || ligne < 0 || ligne >= c.H {
		return -1
	}
	if e, ok := c.etiq[ligne*c.L+col]; ok {
		return e.zone
	}
	if z := c.pix[(ligne*2)*c.L+col]; z != pixelVide {
		return int(z)
	}
	if z := c.pix[(ligne*2+1)*c.L+col]; z != pixelVide {
		return int(z)
	}
	return -1
}

// Rendu produit la carte, la zone sélectionnée mise en évidence.
func (c *Carte) Rendu(selection int) string {
	var b strings.Builder
	b.Grow(c.L * c.H * 24)

	couleur := func(p int16) int {
		if p == pixelVide {
			return couleurBordure
		}
		if int(p) == selection {
			return couleurSelection
		}
		return c.couleurs[p]
	}

	for ligne := 0; ligne < c.H; ligne++ {
		for col := 0; col < c.L; col++ {
			haut := c.pix[(ligne*2)*c.L+col]
			bas := c.pix[(ligne*2+1)*c.L+col]

			if e, ok := c.etiq[ligne*c.L+col]; ok {
				fond := couleur(int16(e.zone))
				fg := couleurEtiquette
				if e.zone == selection {
					fg = 16
				}
				fmt.Fprintf(&b, "\x1b[38;5;%dm\x1b[48;5;%dm%c", fg, fond, e.car)
				continue
			}
			if haut == pixelVide && bas == pixelVide {
				b.WriteString("\x1b[0m ")
				continue
			}
			fmt.Fprintf(&b, "\x1b[38;5;%dm\x1b[48;5;%dm▀", couleur(haut), couleur(bas))
		}
		b.WriteString("\x1b[0m\n")
	}
	return strings.TrimRight(b.String(), "\n")
}
