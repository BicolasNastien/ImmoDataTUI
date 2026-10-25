package main

// Écran de recherche d'adresse, ouvert par « / » depuis la carte.
//
// L'enchaînement est entièrement asynchrone : la requête à la BAN, puis le
// téléchargement des millésimes du département, puis l'extraction de la
// commune, puis l'affichage. Chaque étape est un message ; aucune ne bloque
// l'interface, et échap annule ce qui est en vol.

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

const nbResultats = 8

// msgAdresses porte le résultat d'une recherche.
type msgAdresses struct {
	requete   string
	resultats []Adresse
	err       error
}

func cmdChercher(requete string) tea.Cmd {
	return func() tea.Msg {
		res, err := ChercherAdresse(requete, nbResultats)
		return msgAdresses{requete: requete, resultats: res, err: err}
	}
}

func nouvelleSaisie() textinput.Model {
	ti := textinput.New()
	ti.Placeholder = "12 rue des Lilas, Troyes"
	ti.Prompt = "  ▸ "
	ti.CharLimit = 120
	return ti
}

// ouvrirRecherche bascule sur l'écran de saisie.
func (m model) ouvrirRecherche() (tea.Model, tea.Cmd) {
	m.ecran = ecranRecherche
	m.alerte = ""
	m.saisie.Focus()
	m.saisie.CursorEnd()
	return m, textinput.Blink
}

func (m model) appliquerAdresses(msg msgAdresses) (tea.Model, tea.Cmd) {
	if m.ecran != ecranRecherche {
		return m, nil // réponse arrivée après qu'on a quitté l'écran
	}
	m.chercheEnCours = false
	m.requeteFaite = msg.requete
	if msg.err != nil {
		m.alerte = msg.err.Error()
		m.resultats = nil
		return m, nil
	}
	m.resultats = msg.resultats
	m.curResultat = 0
	if len(m.resultats) == 0 {
		m.alerte = "Aucune adresse trouvée pour « " + msg.requete + " »."
	} else {
		m.alerte = ""
	}
	return m, nil
}

// toucheRecherche : tout ce qui n'est pas navigation part dans le champ de
// saisie, sinon on ne pourrait pas taper « q » dans un nom de rue.
func (m model) toucheRecherche(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {

	case "ctrl+c":
		return m, tea.Quit

	case "esc":
		m.ecran = ecranCarte
		m.saisie.Blur()
		m.alerte = ""
		return m, nil

	case "up":
		m.curResultat = borner(m.curResultat-1, len(m.resultats))
		return m, nil

	case "down":
		m.curResultat = borner(m.curResultat+1, len(m.resultats))
		return m, nil

	case "enter":
		// Un premier ↵ lance la recherche, un second ouvre le résultat
		// sélectionné : la requête n'a pas changé entre-temps, l'intention
		// n'est donc pas ambiguë.
		requete := strings.TrimSpace(m.saisie.Value())
		if requete == "" {
			return m, nil
		}
		if requete == m.requeteFaite && len(m.resultats) > 0 {
			return m.ouvrirAdresse(m.resultats[m.curResultat])
		}
		m.chercheEnCours = true
		m.alerte = ""
		return m, tea.Batch(m.spinner.Tick, cmdChercher(requete))
	}

	var cmd tea.Cmd
	m.saisie, cmd = m.saisie.Update(msg)
	return m, cmd
}

// ouvrirAdresse enchaîne sur le téléchargement du département de l'adresse.
// La suite est câblée dans appliquerTelechargement et appliquerExtraction :
// quand une adresse est en cours, on court-circuite la carte des communes.
func (m model) ouvrirAdresse(a Adresse) (tea.Model, tea.Cmd) {
	dep, ok := DepartementParCode(a.Departement())
	if !ok {
		m.alerte = "Département " + a.Departement() + " inconnu pour « " + a.Label + " »."
		return m, nil
	}
	m.saisie.Blur()
	m.adresse = &a
	m.commune = Zone{Code: a.CodeInsee, Nom: a.Commune}
	m.sourceLocale = false
	return m.telecharger(dep, false)
}

func (m model) vueRecherche() string {
	var b strings.Builder
	b.WriteString("\n  " + styleTitre.Render("DVF — rechercher une adresse") + "\n\n")
	b.WriteString(m.saisie.View() + "\n\n")

	switch {
	case m.chercheEnCours:
		b.WriteString(fmt.Sprintf("  %s recherche…\n", m.spinner.View()))
	case len(m.resultats) == 0:
		b.WriteString("  " + styleDim.Render(
			"Numéro, voie et commune : « 12 rue des Lilas, Troyes ». ↵ pour chercher.") + "\n")
	default:
		for i, a := range m.resultats {
			details := a.CodeInsee
			if a.Contexte != "" {
				details += " · " + a.Contexte
			}
			if i == m.curResultat {
				b.WriteString("  " + styleSel.Render(" ▸ "+a.Label+" ") + "  " +
					styleDim.Render(details) + "\n")
			} else {
				b.WriteString("    " + a.Label + "  " + styleDim.Render(details) + "\n")
			}
		}
	}

	if m.alerte != "" {
		b.WriteString("\n  " + styleWarn.Render(m.alerte) + "\n")
	}

	aide := "↵ chercher · esc retour à la carte"
	if len(m.resultats) > 0 && strings.TrimSpace(m.saisie.Value()) == m.requeteFaite {
		aide = "↑/↓ choisir · ↵ ouvrir l'adresse · esc retour à la carte"
	}
	b.WriteString("\n  " + styleAide.Render(aide) + "\n")
	b.WriteString("  " + styleDim.Render(
		"Les millésimes du département sont téléchargés au besoin, puis agrégés.") + "\n")
	return b.String()
}
