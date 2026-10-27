package main

// Le suivi des annonces : lesquelles on a déjà croisées, lesquelles on a
// ouvertes.
//
// Deux notions, parce qu'elles répondent à deux questions différentes. Une
// annonce *nouvelle* n'était apparue dans aucune recherche avant cette
// session : c'est ce qui a été mis en vente depuis la dernière fois qu'on a
// regardé. Une annonce *vue* a été ouverte dans le navigateur (ou marquée à la
// main) : c'est ce qu'on a déjà examiné. Une annonce peut être nouvelle et vue
// — ouverte il y a cinq minutes —, ou ni l'une ni l'autre — croisée hier sans
// qu'on l'ouvre.

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"
)

// dureeSuivi : au-delà, une annonce oubliée ne pèse plus dans le fichier. Une
// annonce qui reparaît après un an est de toute façon redevenue une nouveauté.
const dureeSuivi = 365 * 24 * time.Hour

// SuiviAnnonces garde, par annonce, la date de première apparition et celle
// de la dernière consultation. Les clés sont « source:identifiant » : deux
// portails peuvent très bien réutiliser le même identifiant.
type SuiviAnnonces struct {
	Croisees map[string]time.Time `json:"croisees"`
	Vues     map[string]time.Time `json:"vues"`

	chemin string    // vide : suivi en mémoire seulement (tests)
	debut  time.Time // lancement du programme, ce qui définit « nouvelle »
}

func nouveauSuivi(chemin string, debut time.Time) *SuiviAnnonces {
	return &SuiviAnnonces{
		Croisees: map[string]time.Time{},
		Vues:     map[string]time.Time{},
		chemin:   chemin,
		debut:    debut,
	}
}

// ChargerSuivi lit le fichier de suivi. Un fichier absent n'est pas une erreur
// — c'est le premier lancement. Un fichier illisible en est une, mais le suivi
// renvoyé reste utilisable : perdre l'historique ne doit pas empêcher de
// chercher des annonces.
func ChargerSuivi(chemin string, maintenant time.Time) (*SuiviAnnonces, error) {
	s := nouveauSuivi(chemin, maintenant)
	brut, err := os.ReadFile(chemin)
	if errors.Is(err, fs.ErrNotExist) {
		return s, nil
	}
	if err != nil {
		return s, fmt.Errorf("lecture du suivi des annonces : %w", err)
	}
	if err := json.Unmarshal(brut, s); err != nil {
		return nouveauSuivi(chemin, maintenant), fmt.Errorf("suivi des annonces illisible : %w", err)
	}
	if s.Croisees == nil {
		s.Croisees = map[string]time.Time{}
	}
	if s.Vues == nil {
		s.Vues = map[string]time.Time{}
	}
	for _, m := range []map[string]time.Time{s.Croisees, s.Vues} {
		for k, t := range m {
			if maintenant.Sub(t) > dureeSuivi {
				delete(m, k)
			}
		}
	}
	return s, nil
}

func cleSuivi(a Annonce) string { return a.Source + ":" + a.ID }

// Croiser enregistre les annonces d'une liste de résultats. Seules les
// inconnues sont datées : une annonce garde la date de sa première apparition.
func (s *SuiviAnnonces) Croiser(as []Annonce, maintenant time.Time) error {
	ajout := false
	for _, a := range as {
		k := cleSuivi(a)
		if _, ok := s.Croisees[k]; !ok {
			s.Croisees[k] = maintenant
			ajout = true
		}
	}
	if !ajout {
		return nil
	}
	return s.enregistrer()
}

// Nouvelle : apparue pour la première fois depuis le lancement du programme.
// Le critère est la session et non la recherche, sans quoi relancer la même
// recherche effacerait toutes les marques.
func (s *SuiviAnnonces) Nouvelle(a Annonce) bool {
	t, ok := s.Croisees[cleSuivi(a)]
	return !ok || !t.Before(s.debut)
}

func (s *SuiviAnnonces) Vue(a Annonce) bool {
	_, ok := s.Vues[cleSuivi(a)]
	return ok
}

func (s *SuiviAnnonces) MarquerVue(a Annonce, maintenant time.Time) error {
	s.Vues[cleSuivi(a)] = maintenant
	return s.enregistrer()
}

// Basculer inverse la marque « vue » et dit dans quel état l'annonce se
// retrouve.
func (s *SuiviAnnonces) Basculer(a Annonce, maintenant time.Time) (bool, error) {
	k := cleSuivi(a)
	if _, ok := s.Vues[k]; ok {
		delete(s.Vues, k)
		return false, s.enregistrer()
	}
	s.Vues[k] = maintenant
	return true, s.enregistrer()
}

// Marque est la colonne de tête des tableaux d'annonces.
func (s *SuiviAnnonces) Marque(a Annonce) string {
	switch {
	case s.Vue(a):
		return "✓"
	case s.Nouvelle(a):
		return "●"
	default:
		return " "
	}
}

// Compter résume une liste : combien de nouvelles, combien de vues.
func (s *SuiviAnnonces) Compter(as []Annonce) (nouvelles, vues int) {
	for _, a := range as {
		if s.Vue(a) {
			vues++
		} else if s.Nouvelle(a) {
			nouvelles++
		}
	}
	return nouvelles, vues
}

func (s *SuiviAnnonces) enregistrer() error {
	if s.chemin == "" {
		return nil
	}
	brut, err := json.Marshal(s)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(s.chemin), permDossier); err != nil {
		return fmt.Errorf("suivi des annonces : %w", err)
	}
	if err := ecrireAtomique(s.chemin, brut); err != nil {
		return fmt.Errorf("suivi des annonces : %w", err)
	}
	return nil
}
