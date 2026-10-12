package billing

import "errors"

// Causes métier de refus d'une fonctionnalité ou d'un quota. Elles sont
// distinguées des erreurs techniques (base injoignable, requête invalide) : un
// refus métier répond 402 avec un message compréhensible, une erreur technique
// répond 500 avec un message générique.
var (
	// ErrFeatureNotIncluded : la fonctionnalité n'est pas incluse dans la formule.
	ErrFeatureNotIncluded = errors.New("fonctionnalité non incluse dans la formule")
	// ErrLimitReached : le plafond de la formule est atteint.
	ErrLimitReached = errors.New("plafond de la formule atteint")
	// ErrUnknownFeature : clé de fonctionnalité absente du registre (erreur de configuration).
	ErrUnknownFeature = errors.New("fonctionnalité inconnue")
)

// refusMetier est un refus destiné à l'utilisateur ; son message reste celui du
// moteur (aucun détail technique), sa cause permet de le classer avec errors.Is.
type refusMetier struct {
	cause   error
	message string
}

func (e *refusMetier) Error() string { return e.message }
func (e *refusMetier) Unwrap() error { return e.cause }

// IsRefusMetier indique si l'erreur est un refus de formule (fonctionnalité non
// incluse ou plafond atteint) et non une panne technique.
func IsRefusMetier(err error) bool {
	return errors.Is(err, ErrFeatureNotIncluded) || errors.Is(err, ErrLimitReached)
}
