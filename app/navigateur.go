package main

// Ouvrir une vue de la rue dans le navigateur du système.
//
// DVF donne l'adresse et, le plus souvent, les coordonnées de la parcelle :
// assez pour aller regarder le bien. Voir la façade répond en trois secondes à
// ce qu'aucune statistique ne dira — l'état du bâti, la rue, le vis-à-vis.

import (
	"fmt"
	"net/url"
	"os/exec"
	"runtime"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// VueExterne est une façon de regarder le bien : une touche, un libellé, et de
// quoi construire l'adresse web.
//
// Il en faut plusieurs parce qu'elles ne répondent pas à la même question. La
// vue de la rue montre la façade mais dépose sur le panorama le plus proche,
// sans dire lequel des bâtiments est le bon. La carte pose un repère exactement
// sur le point, et le cadastre dessine les limites de la parcelle vendue :
// c'est là qu'on identifie le bien, avant d'aller voir sa façade.
type VueExterne struct {
	Touche string
	Nom    string
	Detail string
	URL    func(Sale) string
}

// VuesExternes, dans l'ordre où elles répondent : où est-ce, quelle parcelle,
// à quoi ça ressemble.
var VuesExternes = []VueExterne{
	{
		Touche: "v", Nom: "carte avec repère",
		Detail: "un repère est posé sur le point du bien",
		URL:    urlCarte,
	},
	{
		Touche: "c", Nom: "photo aérienne et parcelles cadastrales",
		Detail: "les limites de la parcelle vendue sont dessinées",
		URL:    urlCadastre,
	},
	{
		Touche: "V", Nom: "vue de la rue",
		Detail: "dépose sur le panorama le plus proche, orientation quelconque",
		URL:    urlRue,
	},
}

// VuePourTouche retrouve une vue par sa touche.
func VuePourTouche(touche string) (VueExterne, bool) {
	for _, v := range VuesExternes {
		if v.Touche == touche {
			return v, true
		}
	}
	return VueExterne{}, false
}

// urlCarte pose un repère sur les coordonnées de la parcelle — c'est la seule
// des trois qui désigne le bien sans ambiguïté. À défaut de coordonnées, la
// recherche sur l'adresse pose elle aussi un repère, moins précis.
func urlCarte(s Sale) string {
	return "https://www.google.com/maps/search/?api=1&query=" + url.QueryEscape(pointOuAdresse(s))
}

// urlRue ouvre la vue de la rue au plus près du bien.
func urlRue(s Sale) string {
	if lat, lon, ok := s.Position(); ok {
		return fmt.Sprintf(
			"https://www.google.com/maps/@?api=1&map_action=pano&viewpoint=%.6f,%.6f",
			lat, lon)
	}
	return "https://www.google.com/maps/search/?api=1&query=" + url.QueryEscape(adresseTextuelle(s))
}

// urlCadastre superpose les parcelles cadastrales à la photo aérienne sur le
// Géoportail. Le Géoportail attend la longitude avant la latitude.
func urlCadastre(s Sale) string {
	lat, lon, ok := s.Position()
	if !ok {
		return ""
	}
	return fmt.Sprintf("https://www.geoportail.gouv.fr/carte?c=%.6f,%.6f&z=19"+
		"&l0=ORTHOIMAGERY.ORTHOPHOTOS::GEOPORTAIL:OGC:WMTS(1)"+
		"&l1=CADASTRALPARCELS.PARCELLAIRE_EXPRESS::GEOPORTAIL:OGC:WMTS(1)"+
		"&permalink=yes", lon, lat)
}

// pointOuAdresse : les coordonnées si on les a, le libellé sinon. Une recherche
// textuelle sur « 12 AV DU PORT » retombe parfois sur une autre commune.
func pointOuAdresse(s Sale) string {
	if lat, lon, ok := s.Position(); ok {
		return fmt.Sprintf("%.6f,%.6f", lat, lon)
	}
	return adresseTextuelle(s)
}

func adresseTextuelle(s Sale) string {
	return strings.TrimSpace(s.Adresse() + " " + s.Ville())
}

// ouvrirURL est indirecte pour que les tests puissent l'intercepter : lancer un
// vrai navigateur pendant `go test` serait pour le moins discourtois.
var ouvrirURL = ouvrirDansNavigateur

// ouvrirDansNavigateur délègue au système : chaque plateforme a son lanceur, et
// aucun d'eux n'attend la fermeture du navigateur.
func ouvrirDansNavigateur(lien string) error {
	var c *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		c = exec.Command("open", lien)
	case "windows":
		c = exec.Command("rundll32", "url.dll,FileProtocolHandler", lien)
	default:
		c = exec.Command("xdg-open", lien)
	}
	if err := c.Start(); err != nil {
		return fmt.Errorf("impossible d'ouvrir le navigateur : %w", err)
	}
	// Le processus est détaché, mais il faut tout de même le récupérer pour ne
	// pas laisser de zombie derrière soi.
	go c.Wait()
	return nil
}

// msgNavigateur rend compte de l'ouverture.
type msgNavigateur struct {
	lien string
	err  error
}

func cmdOuvrirVue(lien string) tea.Cmd {
	return func() tea.Msg {
		return msgNavigateur{lien: lien, err: ouvrirURL(lien)}
	}
}

// ouvrirVue ouvre une vue de la vente sélectionnée. Disponible depuis la fiche
// comme depuis le tableau : c'est la même vente désignée.
func (m model) ouvrirVue(vue VueExterne) (tea.Model, tea.Cmd) {
	v, ok := m.venteCourante()
	if !ok {
		return m, nil
	}
	lien := vue.URL(v)
	if lien == "" {
		m.noteVue = "Pas de " + vue.Nom + " : la parcelle n'est pas géolocalisée."
		return m, nil
	}
	m.noteVue = "Ouverture — " + vue.Nom + "…"
	return m, cmdOuvrirVue(lien)
}

func (m model) appliquerNavigateur(msg msgNavigateur) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		// Sur une machine sans environnement graphique (session SSH), c'est le
		// cas normal : on affiche le lien pour qu'il reste copiable.
		m.noteVue = msg.err.Error() + " — lien : " + msg.lien
	} else {
		m.noteVue = "Vue ouverte dans le navigateur."
	}
	if m.ecran == ecranVente && m.pretVP {
		m.vp.SetContent(m.contenuVente())
	}
	return m, nil
}
