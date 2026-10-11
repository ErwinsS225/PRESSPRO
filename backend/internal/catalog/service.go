package catalog

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"
)

// Service gère le catalogue et la grille tarifaire en FCFA
// Assure l'isolation multi-tenant et la persistance PostgreSQL
// Structure thread-safe pour accès concurrents
type Service struct {
	mu           sync.RWMutex            // Mutex RW pour protection de la map partagée
	logger       *log.Logger             // Logger structuré pour débogage et traçabilité
	services     map[string]*ServiceItem // Map des services en mémoire (fallback si store PostgreSQL non utilisé)
	store        Store                   // Interface de persistance PostgreSQL
	sequence     int                     // Séquence pour générer des ID uniques par tenant
	tenantSeeded map[string]struct{}     // Tenants déjà initialisés avec la grille par défaut
}

// newService crée un Service prêt à l'emploi (logger, index mémoire, séquence).
// Toute construction d'un Service DOIT passer par cette fabrique ou par
// NewService()/NewStore() : un Service{} nu laisserait le logger à nil et
// provoquerait une panique à la première lecture du catalogue.
func newService() *Service {
	return &Service{
		services:     make(map[string]*ServiceItem),
		logger:       log.Default(),
		sequence:     1,
		tenantSeeded: make(map[string]struct{}),
	}
}

// defaultSeedItems retourne la grille tarifaire par défaut du produit,
// rattachée au tenant fourni (copie indépendante à chaque appel).
func defaultSeedItems(tenantID string) []ServiceItem {
	return []ServiceItem{
		// A. VÊTEMENTS DU QUOTIDIEN (Priorité 1)
		{
			ID:                 "srv-101",
			TenantID:           tenantID,
			Name:               "Chemise / chemisier (Lavage + repassage)",
			Article:            "Chemise / chemisier",
			Treatment:          "Lavage + repassage",
			Category:           CategoryDaily,
			UnitPrice:          500,
			Unit:               "pièce",
			PricingModel:       ModelPiece,
			EstimatedHours:     24,
			IsActive:           true,
			IsExpressAvailable: true,
			ExpressSurcharge:   500,
			IsPickupAvailable:  true,
			Description:        "Lavage doux assouplissant et repassage soigné sur cintre.",
		},
		{
			ID:                 "srv-102",
			TenantID:           tenantID,
			Name:               "Chemise / chemisier (Repassage seul)",
			Article:            "Chemise / chemisier",
			Treatment:          "Repassage seul",
			Category:           CategoryDaily,
			UnitPrice:          400,
			Unit:               "pièce",
			PricingModel:       ModelPiece,
			EstimatedHours:     12,
			IsActive:           true,
			IsExpressAvailable: true,
			ExpressSurcharge:   300,
			IsPickupAvailable:  true,
			Description:        "Repassage vapeur haute précision et cintre individuel.",
		},
		{
			ID:                 "srv-103",
			TenantID:           tenantID,
			Name:               "T-shirt / polo (Lavage + pliage)",
			Article:            "T-shirt / polo",
			Treatment:          "Lavage + pliage",
			Category:           CategoryDaily,
			UnitPrice:          400,
			Unit:               "pièce",
			PricingModel:       ModelPiece,
			EstimatedHours:     24,
			IsActive:           true,
			IsExpressAvailable: true,
			ExpressSurcharge:   300,
			IsPickupAvailable:  true,
			Description:        "Lavage coton délicat et pliage compact sous film.",
		},
		{
			ID:                 "srv-104",
			TenantID:           tenantID,
			Name:               "T-shirt / polo (Lavage + repassage)",
			Article:            "T-shirt / polo",
			Treatment:          "Lavage + repassage",
			Category:           CategoryDaily,
			UnitPrice:          500,
			Unit:               "pièce",
			PricingModel:       ModelPiece,
			EstimatedHours:     24,
			IsActive:           true,
			IsExpressAvailable: true,
			ExpressSurcharge:   300,
			IsPickupAvailable:  true,
			Description:        "Lavage couleurs vives et repassage sans faux plis.",
		},
		{
			ID:                 "srv-105",
			TenantID:           tenantID,
			Name:               "Pantalon / jean (Lavage + repassage)",
			Article:            "Pantalon / jean",
			Treatment:          "Lavage + repassage",
			Category:           CategoryDaily,
			UnitPrice:          1000,
			Unit:               "pièce",
			PricingModel:       ModelPiece,
			EstimatedHours:     24,
			IsActive:           true,
			IsExpressAvailable: true,
			ExpressSurcharge:   500,
			IsPickupAvailable:  true,
			Description:        "Lavage denim/coton assouplissant avec repassage à plis ou plat.",
		},
		{
			ID:                 "srv-106",
			TenantID:           tenantID,
			Name:               "Jupe / robe simple (Lavage + repassage)",
			Article:            "Jupe / robe simple",
			Treatment:          "Lavage + repassage",
			Category:           CategoryDaily,
			UnitPrice:          1000,
			Unit:               "pièce",
			PricingModel:       ModelPiece,
			EstimatedHours:     24,
			IsActive:           true,
			IsExpressAvailable: true,
			ExpressSurcharge:   500,
			IsPickupAvailable:  true,
			Description:        "Nettoyage textile adapté et repassage avec tombé fluide.",
		},
		{
			ID:                 "srv-107",
			TenantID:           tenantID,
			Name:               "Short / bermuda (Lavage + repassage)",
			Article:            "Short / bermuda",
			Treatment:          "Lavage + repassage",
			Category:           CategoryDaily,
			UnitPrice:          700,
			Unit:               "pièce",
			PricingModel:       ModelPiece,
			EstimatedHours:     24,
			IsActive:           true,
			IsExpressAvailable: true,
			ExpressSurcharge:   400,
			IsPickupAvailable:  true,
			Description:        "Lavage détachant et repassage.",
		},
		{
			ID:                 "srv-108",
			TenantID:           tenantID,
			Name:               "Veste simple / costume (Nettoyage à sec spécialisé)",
			Article:            "Veste simple",
			Treatment:          "Nettoyage à sec spécialisé",
			Category:           CategoryDaily,
			UnitPrice:          1500,
			Unit:               "pièce",
			PricingModel:       ModelPiece,
			EstimatedHours:     24,
			IsActive:           true,
			IsExpressAvailable: true,
			ExpressSurcharge:   800,
			IsPickupAvailable:  true,
			Description:        "Dégraissage solvant écologique et repassage sur mannequin.",
		},
		{
			ID:                 "srv-109",
			TenantID:           tenantID,
			Name:               "Costume deux pièces (Nettoyage à sec + finition)",
			Article:            "Costume deux pièces",
			Treatment:          "Nettoyage à sec adapté, finition",
			Category:           CategoryDaily,
			UnitPrice:          2500,
			Unit:               "ensemble",
			PricingModel:       ModelPiece,
			EstimatedHours:     48,
			IsActive:           true,
			IsExpressAvailable: true,
			ExpressSurcharge:   1200,
			IsPickupAvailable:  true,
			Description:        "Veste et pantalon traités ensemble avec pressage thermo-régulé.",
		},
		{
			ID:                 "srv-110",
			TenantID:           tenantID,
			Name:               "Manteau / veste lourde (Nettoyage spécialisé)",
			Article:            "Manteau / veste lourde",
			Treatment:          "Nettoyage spécialisé",
			Category:           CategoryDaily,
			UnitPrice:          3000,
			Unit:               "pièce",
			PricingModel:       ModelPiece,
			EstimatedHours:     48,
			IsActive:           true,
			IsExpressAvailable: true,
			ExpressSurcharge:   1500,
			IsPickupAvailable:  true,
			Description:        "Traitement en profondeur des fibres lourdes et mise sous housse.",
		},
		{
			ID:                 "srv-111",
			TenantID:           tenantID,
			Name:               "Uniforme scolaire (Lavage + repassage)",
			Article:            "Uniforme scolaire",
			Treatment:          "Lavage + repassage",
			Category:           CategoryDaily,
			UnitPrice:          800,
			Unit:               "pièce ou ensemble",
			PricingModel:       ModelPiece,
			EstimatedHours:     24,
			IsActive:           true,
			IsExpressAvailable: true,
			ExpressSurcharge:   400,
			IsPickupAvailable:  true,
			Description:        "Tarif spécial rentrée/famille pour blouses et tenues réglementaires.",
		},
		{
			ID:                 "srv-112",
			TenantID:           tenantID,
			Name:               "Tenue professionnelle / Blouse (Lavage + repassage)",
			Article:            "Tenue professionnelle",
			Treatment:          "Lavage, repassage, finition",
			Category:           CategoryDaily,
			UnitPrice:          1000,
			Unit:               "pièce ou ensemble",
			PricingModel:       ModelPiece,
			EstimatedHours:     24,
			IsActive:           true,
			IsExpressAvailable: true,
			ExpressSurcharge:   500,
			IsPickupAvailable:  true,
			Description:        "Blouses de laboratoire, tenues soignants, tabliers de service.",
		},

		// B. VÊTEMENTS TRADITIONNELS ET CÉRÉMONIELS (Priorité 1 - Côte d'Ivoire)
		{
			ID:                 "srv-201",
			TenantID:           tenantID,
			Name:               "Boubou simple (Lavage et repassage)",
			Article:            "Boubou simple",
			Treatment:          "Lavage et repassage",
			Category:           CategoryTraditional,
			UnitPrice:          2000,
			Unit:               "pièce",
			PricingModel:       ModelPiece,
			EstimatedHours:     24,
			IsActive:           true,
			IsExpressAvailable: true,
			ExpressSurcharge:   1000,
			IsPickupAvailable:  true,
			Description:        "Lavage manuel doux ou machine délicat avec repassage à grand plat.",
		},
		{
			ID:                 "srv-202",
			TenantID:           tenantID,
			Name:               "Boubou brodé (Traitement adapté aux broderies)",
			Article:            "Boubou brodé",
			Treatment:          "Traitement adapté aux broderies",
			Category:           CategoryTraditional,
			UnitPrice:          3500,
			Unit:               "pièce",
			PricingModel:       ModelPiece,
			EstimatedHours:     48,
			IsActive:           true,
			IsExpressAvailable: true,
			ExpressSurcharge:   1500,
			IsPickupAvailable:  true,
			Description:        "Protection minutieuse des fils d'or et d'argent, repassage à l'envers.",
		},
		{
			ID:                 "srv-203",
			TenantID:           tenantID,
			Name:               "Ensemble en bazin (Entretien et finition brillance)",
			Article:            "Ensemble en bazin",
			Treatment:          "Entretien et finition selon le tissu",
			Category:           CategoryTraditional,
			UnitPrice:          4500,
			Unit:               "ensemble",
			PricingModel:       ModelPiece,
			EstimatedHours:     48,
			IsActive:           true,
			IsExpressAvailable: true,
			ExpressSurcharge:   2000,
			IsPickupAvailable:  true,
			Description:        "Restauration de la rigidité et brillance satinée du Bazin riche.",
		},
		{
			ID:                 "srv-204",
			TenantID:           tenantID,
			Name:               "Tenue en dentelle (Traitement délicat)",
			Article:            "Tenue en dentelle",
			Treatment:          "Traitement délicat",
			Category:           CategoryTraditional,
			UnitPrice:          4000,
			Unit:               "ensemble ou pièce",
			PricingModel:       ModelPiece,
			EstimatedHours:     48,
			IsActive:           true,
			IsExpressAvailable: false,
			IsPickupAvailable:  true,
			Description:        "Lavage à froid sans torsion pour préserver la trame de la dentelle.",
		},
		{
			ID:                 "srv-205",
			TenantID:           tenantID,
			Name:               "Tunique ou vêtement brodé (Protection des ornements)",
			Article:            "Tunique ou vêtement brodé",
			Treatment:          "Protection des ornements",
			Category:           CategoryTraditional,
			UnitPrice:          2500,
			Unit:               "pièce",
			PricingModel:       ModelPiece,
			EstimatedHours:     24,
			IsActive:           true,
			IsExpressAvailable: true,
			ExpressSurcharge:   1000,
			IsPickupAvailable:  true,
			Description:        "Détachage de précision autour des ornements et perles cousues.",
		},
		{
			ID:                 "srv-206",
			TenantID:           tenantID,
			Name:               "Tenue de cérémonie (Contrôle d'état et mise en valeur)",
			Article:            "Tenue de cérémonie",
			Treatment:          "Contrôle de l’état du vêtement",
			Category:           CategoryTraditional,
			UnitPrice:          5000,
			Unit:               "ensemble",
			PricingModel:       ModelPiece,
			EstimatedHours:     48,
			IsActive:           true,
			IsExpressAvailable: true,
			ExpressSurcharge:   2500,
			IsPickupAvailable:  true,
			Description:        "Diagnostic avant traitement, repassage drapé sous housse luxe.",
		},
		{
			ID:                 "srv-207",
			TenantID:           tenantID,
			Name:               "Robe de soirée (Nettoyage spécialisé)",
			Article:            "Robe de soirée",
			Treatment:          "Nettoyage spécialisé",
			Category:           CategoryTraditional,
			UnitPrice:          5000,
			Unit:               "pièce",
			PricingModel:       ModelPiece,
			EstimatedHours:     48,
			IsActive:           true,
			IsExpressAvailable: true,
			ExpressSurcharge:   2000,
			IsPickupAvailable:  true,
			Description:        "Nettoyage haute précision des soies, mousselines et sequins.",
		},
		{
			ID:                "srv-208",
			TenantID:          tenantID,
			Name:              "Robe de mariée (Prestige & Finition sur devis)",
			Article:           "Robe de mariée",
			Treatment:         "Devis éventuel selon la complexité",
			Category:          CategoryTraditional,
			UnitPrice:         15000,
			Unit:              "pièce",
			PricingModel:      ModelPiece,
			EstimatedHours:    72,
			IsActive:          true,
			RequiresQuote:     true,
			IsPickupAvailable: true,
			Description:       "Traitement haute couture avec emballage boîte à chapeau protectrice.",
		},
		{
			ID:                "srv-209",
			TenantID:          tenantID,
			Name:              "Voile et accessoires de mariage (Traitement délicat)",
			Article:           "Voile et accessoires de mariage",
			Treatment:         "Traitement délicat",
			Category:          CategoryTraditional,
			UnitPrice:         3500,
			Unit:              "pièce",
			PricingModel:      ModelPiece,
			EstimatedHours:    48,
			IsActive:          true,
			IsPickupAvailable: true,
			Description:       "Nettoyage vapeur doux du tulle et étui protecteur.",
		},

		// C. LINGE DE MAISON
		{
			ID:                 "srv-301",
			TenantID:           tenantID,
			Name:               "Drap simple ou double (Standard)",
			Article:            "Drap simple ou double",
			Treatment:          "Lavage grand volume + calandrage",
			Category:           CategoryHomeLinen,
			UnitPrice:          1000,
			Unit:               "pièce",
			PricingModel:       ModelPiece,
			EstimatedHours:     24,
			IsActive:           true,
			IsExpressAvailable: true,
			ExpressSurcharge:   500,
			IsPickupAvailable:  true,
			Description:        "Lessive anti-acariens haute température et repassage sur calandreuse.",
		},
		{
			ID:                 "srv-302",
			TenantID:           tenantID,
			Name:               "Taie d'oreiller (Standard)",
			Article:            "Taie d'oreiller",
			Treatment:          "Lavage + repassage",
			Category:           CategoryHomeLinen,
			UnitPrice:          400,
			Unit:               "pièce",
			PricingModel:       ModelPiece,
			EstimatedHours:     24,
			IsActive:           true,
			IsExpressAvailable: true,
			ExpressSurcharge:   200,
			IsPickupAvailable:  true,
			Description:        "Désinfection et calandrage.",
		},
		{
			ID:                 "srv-303",
			TenantID:           tenantID,
			Name:               "Housse de couette (Standard / Premium)",
			Article:            "Housse de couette",
			Treatment:          "Lavage assouplissant + repassage calandre",
			Category:           CategoryHomeLinen,
			UnitPrice:          2000,
			Unit:               "pièce",
			PricingModel:       ModelPiece,
			EstimatedHours:     24,
			IsActive:           true,
			IsExpressAvailable: true,
			ExpressSurcharge:   1000,
			IsPickupAvailable:  true,
			Description:        "Lavage en profondeur et pliage au carré.",
		},
		{
			ID:                 "srv-304",
			TenantID:           tenantID,
			Name:               "Couette / couverture (Moderne / Premium)",
			Article:            "Couette / couverture",
			Treatment:          "Désinfection thermique grand tambour",
			Category:           CategoryHomeLinen,
			UnitPrice:          4500,
			Unit:               "pièce",
			PricingModel:       ModelPiece,
			EstimatedHours:     48,
			IsActive:           true,
			IsExpressAvailable: true,
			ExpressSurcharge:   2000,
			IsPickupAvailable:  true,
			Description:        "Traitement plume ou synthétique avec séchage anti-agglomération.",
		},
		{
			ID:                 "srv-305",
			TenantID:           tenantID,
			Name:               "Serviette de toilette (Lavage & séchage moelleux)",
			Article:            "Serviette de toilette",
			Treatment:          "Lavage haute température + assouplissant",
			Category:           CategoryHomeLinen,
			UnitPrice:          600,
			Unit:               "pièce ou kg",
			PricingModel:       ModelPiece,
			EstimatedHours:     24,
			IsActive:           true,
			IsExpressAvailable: true,
			ExpressSurcharge:   300,
			IsPickupAvailable:  true,
			Description:        "Finition éponge ultra-moelleuse et parfum frais.",
		},
		{
			ID:                 "srv-306",
			TenantID:           tenantID,
			Name:               "Nappe de table (Standard / Premium)",
			Article:            "Nappe de table",
			Treatment:          "Détachage gras/vin + repassage impeccable",
			Category:           CategoryHomeLinen,
			UnitPrice:          1500,
			Unit:               "pièce",
			PricingModel:       ModelPiece,
			EstimatedHours:     24,
			IsActive:           true,
			IsExpressAvailable: true,
			ExpressSurcharge:   800,
			IsPickupAvailable:  true,
			Description:        "Détachage spécial restauration et amidonnage léger.",
		},
		{
			ID:                "srv-307",
			TenantID:          tenantID,
			Name:              "Rideau (Panneau ou pièce)",
			Article:           "Rideau",
			Treatment:         "Dépoussiérage + défroissage vapeur",
			Category:          CategoryHomeLinen,
			UnitPrice:         4000,
			Unit:              "panneau ou pièce",
			PricingModel:      ModelPiece,
			EstimatedHours:    48,
			IsActive:          true,
			IsPickupAvailable: true,
			Description:       "Entretien voilages et rideaux lourds sans rétrécissement.",
		},
		{
			ID:                "srv-308",
			TenantID:          tenantID,
			Name:              "Tapis / moquette (Nettoyage mécanique au m²)",
			Article:           "Tapis / moquette",
			Treatment:         "Brossage rotatif + injection/extraction",
			Category:          CategoryHomeLinen,
			UnitPrice:         3000,
			Unit:              "pièce ou m²",
			PricingModel:      ModelPiece,
			EstimatedHours:    72,
			IsActive:          true,
			IsOutsourced:      false,
			IsPickupAvailable: true,
			Description:       "Élimination des taches incrustées et désodorisation profonde.",
		},
		{
			ID:                "srv-309",
			TenantID:          tenantID,
			Name:              "Housse de canapé (Détachage + nettoyage)",
			Article:           "Housse de canapé",
			Treatment:         "Nettoyage haute performance",
			Category:          CategoryHomeLinen,
			UnitPrice:         5000,
			Unit:              "pièce ou ensemble",
			PricingModel:      ModelPiece,
			EstimatedHours:    48,
			IsActive:          true,
			IsPickupAvailable: true,
			Description:       "Traitement tissus d'ameublement anti-taches.",
		},
		{
			ID:                "srv-310",
			TenantID:          tenantID,
			Name:              "Couverture volumineuse (Grand format)",
			Article:           "Couverture volumineuse",
			Treatment:         "Lavage en tambour industriel 25kg",
			Category:          CategoryHomeLinen,
			UnitPrice:         5000,
			Unit:              "pièce",
			PricingModel:      ModelPiece,
			EstimatedHours:    48,
			IsActive:          true,
			IsPickupAvailable: true,
			Description:       "Idéal pour grosses couvertures d'hiver et dessus de lit rembourrés.",
		},

		// D. SERVICES COMPLÉMENTAIRES MODERNES (Options de commande)
		{
			ID:                 "srv-401",
			TenantID:           tenantID,
			Name:               "Lavage et pliage au kilogramme (Linge au poids)",
			Article:            "Linge courant au poids",
			Treatment:          "Lavage et pliage",
			Category:           CategoryAddons,
			UnitPrice:          1200,
			Unit:               "kg",
			PricingModel:       ModelKilogram,
			EstimatedHours:     24,
			IsActive:           true,
			IsExpressAvailable: true,
			ExpressSurcharge:   600,
			IsPickupAvailable:  true,
			Description:        "Formule économique au kilo pour draps, sous-vêtements et t-shirts quotidiens.",
		},
		{
			ID:                 "srv-402",
			TenantID:           tenantID,
			Name:               "Repassage seul au kilogramme",
			Article:            "Linge lavé au poids",
			Treatment:          "Repassage seul",
			Category:           CategoryAddons,
			UnitPrice:          1000,
			Unit:               "kg",
			PricingModel:       ModelKilogram,
			EstimatedHours:     24,
			IsActive:           true,
			IsExpressAvailable: true,
			ExpressSurcharge:   500,
			IsPickupAvailable:  true,
			Description:        "Repassage professionnel pour le linge apporté déjà propre et sec.",
		},
		{
			ID:                "srv-403",
			TenantID:          tenantID,
			Name:              "Détachage renforcé (Option par article)",
			Article:           "Tache difficile (vin, herbe, graisse, rouille)",
			Treatment:         "Détachage chimique ciblé",
			Category:          CategoryAddons,
			UnitPrice:         500,
			Unit:              "supplément par article",
			PricingModel:      ModelPiece,
			EstimatedHours:    12,
			IsActive:          true,
			IsPickupAvailable: true,
			Description:       "Application de détachants enzymatiques spécifiques avant lavage.",
		},
		{
			ID:             "srv-404",
			TenantID:       tenantID,
			Name:           "Repassage avec apprêt / amidonnage",
			Article:        "Chemise, col, boubou",
			Treatment:      "Apprêt amidonné",
			Category:       CategoryAddons,
			UnitPrice:      300,
			Unit:           "supplément par article",
			PricingModel:   ModelPiece,
			EstimatedHours: 6,
			IsActive:       true,
			Description:    "Tenue impeccable des cols, poignets et pagnes traditionnels.",
		},
		{
			ID:             "srv-405",
			TenantID:       tenantID,
			Name:           "Traitement anti-odeurs & assainissant",
			Article:        "Tout vêtement ou linge",
			Treatment:      "Traitement anti-odeurs",
			Category:       CategoryAddons,
			UnitPrice:      500,
			Unit:           "supplément par article ou lot",
			PricingModel:   ModelPiece,
			EstimatedHours: 6,
			IsActive:       true,
			Description:    "Neutralisation des odeurs d'humidité, tabac ou sueur.",
		},
		{
			ID:             "srv-406",
			TenantID:       tenantID,
			Name:           "Traitement imperméabilisant déperlant",
			Article:        "Veste, manteau, coupe-vent",
			Treatment:      "Traitement imperméabilisant",
			Category:       CategoryAddons,
			UnitPrice:      1500,
			Unit:           "par article",
			PricingModel:   ModelPiece,
			EstimatedHours: 24,
			IsActive:       true,
			Description:    "Protection hydrophobe contre la pluie tropicale et les éclaboussures.",
		},
		{
			ID:             "srv-407",
			TenantID:       tenantID,
			Name:           "Mise sur cintre métallique avec film",
			Article:        "Chemise, pantalon, veste",
			Treatment:      "Mise sur cintre",
			Category:       CategoryAddons,
			UnitPrice:      200,
			Unit:           "par article",
			PricingModel:   ModelPiece,
			EstimatedHours: 0,
			IsActive:       true,
			Description:    "Cintre rigide pour maintien sans pli.",
		},
		{
			ID:             "srv-408",
			TenantID:       tenantID,
			Name:           "Emballage individuel sous housse zippée",
			Article:        "Costumes, robes, boubous",
			Treatment:      "Emballage individuel luxe",
			Category:       CategoryAddons,
			UnitPrice:      500,
			Unit:           "par article",
			PricingModel:   ModelPiece,
			EstimatedHours: 0,
			IsActive:       true,
			Description:    "Housse respirante zippée de protection anti-poussière.",
		},
		{
			ID:                "srv-409",
			TenantID:          tenantID,
			Name:              "Collecte & Livraison à domicile (Abidjan)",
			Article:           "Courses pressing",
			Treatment:         "Collecte et livraison",
			Category:          CategoryAddons,
			UnitPrice:         1500,
			Unit:              "forfait par course",
			PricingModel:      ModelPiece,
			EstimatedHours:    0,
			IsActive:          true,
			IsPickupAvailable: true,
			Description:       "Prise en charge directe à domicile ou bureau avec créneau horaire.",
		},

		// E. PRESTATIONS SPÉCIALISÉES ET LUXE
		{
			ID:                "srv-501",
			TenantID:          tenantID,
			Name:              "Nettoyage de vêtements de créateurs (Soins prestige)",
			Article:           "Vêtements de créateurs / haute couture",
			Treatment:         "Nettoyage de vêtements de créateurs",
			Category:          CategorySpecialized,
			UnitPrice:         8000,
			Unit:              "par pièce ou devis",
			PricingModel:      ModelPiece,
			EstimatedHours:    72,
			IsActive:          true,
			RequiresQuote:     false,
			IsPickupAvailable: true,
			Description:       "Soin expert pour textiles précieux, broderies manuelles et coupes haute couture.",
		},
		{
			ID:                "srv-502",
			TenantID:          tenantID,
			Name:              "Nettoyage de soie et textiles délicats",
			Article:           "Soie, mousseline, cachemire",
			Treatment:         "Nettoyage de soie et textiles délicats",
			Category:          CategorySpecialized,
			UnitPrice:         4500,
			Unit:              "par pièce",
			PricingModel:      ModelPiece,
			EstimatedHours:    48,
			IsActive:          true,
			IsPickupAvailable: true,
			Description:       "Bains doux sans friction avec détergents neutres pour protéger le fil de soie.",
		},
		{
			ID:                "srv-503",
			TenantID:          tenantID,
			Name:              "Nettoyage de cuir et daim (Veste, blouson)",
			Article:           "Veste cuir, blouson daim",
			Treatment:         "Nettoyage de cuir et daim",
			Category:          CategorySpecialized,
			UnitPrice:         7000,
			Unit:              "par article",
			PricingModel:      ModelPiece,
			EstimatedHours:    96,
			IsActive:          true,
			IsOutsourced:      true,
			IsPickupAvailable: true,
			Description:       "Nettoyage spécialisé peausserie, réhydratation du cuir et recoloration si nécessaire.",
		},
		{
			ID:                "srv-504",
			TenantID:          tenantID,
			Name:              "Nettoyage & Rénovation de baskets (Sneakers Care)",
			Article:           "Baskets, sneakers toutes marques",
			Treatment:         "Nettoyage de baskets",
			Category:          CategorySpecialized,
			UnitPrice:         3000,
			Unit:              "par paire",
			PricingModel:      ModelPiece,
			EstimatedHours:    48,
			IsActive:          true,
			IsPickupAvailable: true,
			Description:       "Nettoyage semelles, empeignes cuir/mesh, lacets et désinfection intérieure à l'ozone.",
		},
		{
			ID:                "srv-505",
			TenantID:          tenantID,
			Name:              "Nettoyage & Glaçage de chaussures en cuir",
			Article:           "Chaussures de ville, richelieus, derbies",
			Treatment:         "Nettoyage de chaussures en cuir",
			Category:          CategorySpecialized,
			UnitPrice:         2500,
			Unit:              "par paire",
			PricingModel:      ModelPiece,
			EstimatedHours:    24,
			IsActive:          true,
			IsPickupAvailable: true,
			Description:       "Décrassage, crémage nourrissant et glaçage traditionnel à la cire d'abeille.",
		},
		{
			ID:                "srv-506",
			TenantID:          tenantID,
			Name:              "Nettoyage & Rénovation de sacs à main",
			Article:           "Sac à main cuir, toile ou luxe",
			Treatment:         "Nettoyage de sacs à main",
			Category:          CategorySpecialized,
			UnitPrice:         5000,
			Unit:              "par article",
			PricingModel:      ModelPiece,
			EstimatedHours:    72,
			IsActive:          true,
			IsPickupAvailable: true,
			Description:       "Détachage des anses, protection des boucles métalliques et ravivage des teintes.",
		},
		{
			ID:             "srv-507",
			TenantID:       tenantID,
			Name:           "Nettoyage & Désinfection de valises",
			Article:        "Valise cabine ou grand format",
			Treatment:      "Nettoyage de valises",
			Category:       CategorySpecialized,
			UnitPrice:      4000,
			Unit:           "par article",
			PricingModel:   ModelPiece,
			EstimatedHours: 48,
			IsActive:       true,
			Description:    "Dépoussiérage des compartiments, désinfection vapeur et lustrage de la coque.",
		},
		{
			ID:             "srv-508",
			TenantID:       tenantID,
			Name:           "Nettoyage de tapis précieux (Soie d'Orient, Laine nouée)",
			Article:        "Tapis d'Orient, tapis d'art",
			Treatment:      "Nettoyage de tapis précieux",
			Category:       CategorySpecialized,
			UnitPrice:      6000,
			Unit:           "par pièce ou m²",
			PricingModel:   ModelPiece,
			EstimatedHours: 96,
			IsActive:       true,
			RequiresQuote:  true,
			IsOutsourced:   true,
			Description:    "Lavage artisanal à plat, respect des colorants naturels végétaux et séchage thermo-régulé.",
		},
		{
			ID:             "srv-509",
			TenantID:       tenantID,
			Name:           "Nettoyage de matelas et fauteuils à domicile",
			Article:        "Matelas 2 places, fauteuil de salon",
			Treatment:      "Nettoyage de matelas et fauteuils",
			Category:       CategorySpecialized,
			UnitPrice:      15000,
			Unit:           "par élément ou forfait",
			PricingModel:   ModelPiece,
			EstimatedHours: 48,
			IsActive:       true,
			RequiresQuote:  true,
			IsOutsourced:   true,
			Description:    "Intervention mobile d'injection-extraction sur site avec désinfection anti-punaises et acariens.",
		},
	}

}

// NewService initialise le service catalogue avec les tarifs par défaut
// du tenant de démonstration (mode mémoire).
// NewService initialise un catalogue vide. La grille tarifaire par défaut du
// produit n'est copiée que pour un établissement réellement inscrit
// (SeedTenantDefaults) : aucun catalogue d'entreprise fictive n'est embarqué.
func NewService() *Service {
	s := newService()
	s.logger.Printf("[catalog] Service initialisé (séquence: %d, catalogue vide)", s.sequence)
	return s
}

// CreateService enregistre une nouvelle prestation dans la grille
func (s *Service) CreateService(ctx context.Context, tenantID string, req CreateServiceRequest) (*ServiceItem, error) {
	if s.store != nil {
		return s.store.CreateService(ctx, tenantID, req)
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return nil, ErrEmptyName
	}
	if req.UnitPrice < 0 {
		return nil, ErrInvalidPrice
	}

	unit := req.Unit
	if unit == "" {
		unit = "pièce"
	}
	hours := req.EstimatedHours
	if hours <= 0 {
		hours = 24
	}
	cat := req.Category
	if cat == "" {
		cat = CategoryDaily
	}

	pricingModel := req.PricingModel
	if pricingModel == "" {
		pricingModel = ModelPiece
	}

	isActive := true
	if req.IsActive != nil {
		isActive = *req.IsActive
	}

	isExpress := false
	if req.IsExpressAvailable != nil {
		isExpress = *req.IsExpressAvailable
	}
	var surcharge float64
	if req.ExpressSurcharge != nil {
		surcharge = *req.ExpressSurcharge
	}

	isPickup := true
	if req.IsPickupAvailable != nil {
		isPickup = *req.IsPickupAvailable
	}

	isOutsourced := false
	if req.IsOutsourced != nil {
		isOutsourced = *req.IsOutsourced
	}

	requiresQuote := false
	if req.RequiresQuote != nil {
		requiresQuote = *req.RequiresQuote
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	id := fmt.Sprintf("srv-%d", time.Now().UnixNano())
	now := time.Now().UTC()

	article := req.Article
	if article == "" {
		article = name
	}
	treatment := req.Treatment
	if treatment == "" {
		treatment = "Lavage & Soin"
	}

	item := &ServiceItem{
		ID:                 id,
		TenantID:           tenantID,
		Name:               name,
		Article:            article,
		Treatment:          treatment,
		Category:           cat,
		UnitPrice:          req.UnitPrice,
		Unit:               unit,
		PricingModel:       pricingModel,
		EstimatedHours:     hours,
		IsActive:           isActive,
		IsExpressAvailable: isExpress,
		ExpressSurcharge:   surcharge,
		IsPickupAvailable:  isPickup,
		IsOutsourced:       isOutsourced,
		RequiresQuote:      requiresQuote,
		Description:        req.Description,
		CreatedAt:          now,
		UpdatedAt:          now,
	}

	s.services[id] = item
	return item, nil
}

// GetServices liste les prestations du catalogue filtrées par tenant_id
// validateTenantExistence initialise les données par défaut pour un nouveau tenant
// Appelé avant tout accès aux données pour garantir l'isolation
func (s *Service) validateTenantExistence(tenantID string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, item := range s.services {
		if item.TenantID == tenantID {
			return true // Tenant déjà initialisé
		}
	}
	return false
}

// seedDefaultServices copie la grille tarifaire par défaut pour un nouveau
// tenant (création à la volée en mode mémoire). Les identifiants sont préfixés
// par un fragment du tenant afin qu'ils soient uniques dans l'index mémoire
// partagé : deux établissements disposent ainsi de prestations réellement
// distinctes et attribuables à leur propriétaire.
func (s *Service) seedDefaultServices(tenantID string) int {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.tenantSeeded[tenantID]; exists {
		return 0 // déjà initialisé (double vérification sous verrou)
	}

	prefix := tenantIDSuffix(tenantID)
	seeded := 0
	for _, it := range defaultSeedItems(tenantID) {
		item := it
		item.ID = prefix + "-" + it.ID
		item.CreatedAt = time.Now().UTC()
		item.UpdatedAt = item.CreatedAt
		s.services[item.ID] = &item
		seeded++
	}
	s.tenantSeeded[tenantID] = struct{}{}

	s.logger.Printf("[catalog] Nouveau tenant initialisé: %s (%d prestations par défaut)", tenantID, seeded)
	return seeded
}

// SeedTenantDefaults initialise la grille tarifaire par défaut d'un nouvel
// établissement (appelé après l'inscription d'un pressing). L'opération est
// idempotente : un tenant disposant déjà de prestations n'est jamais modifié.
func (s *Service) SeedTenantDefaults(ctx context.Context, tenantID string) (int, error) {
	if strings.TrimSpace(tenantID) == "" {
		return 0, ErrServiceNotFound
	}
	items := defaultSeedItems(tenantID)
	if s.store != nil {
		inserted, err := s.store.SeedIfEmpty(ctx, tenantID, items)
		if err != nil {
			return 0, err
		}
		if inserted > 0 {
			s.logger.Printf("[catalog] Grille tarifaire par défaut créée pour le tenant %s (%d prestations)", tenantID, inserted)
		}
		return inserted, nil
	}
	seeded := s.seedDefaultServices(tenantID)
	return seeded, nil
}

// tenantIDSuffix extrait un fragment stable et sûr du tenant pour préfixer les
// identifiants mémoire (les 8 premiers caractères alphanumériques).
func tenantIDSuffix(tenantID string) string {
	clean := make([]rune, 0, 8)
	for _, r := range tenantID {
		if (r >= '0' && r <= '9') || (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') {
			clean = append(clean, r)
		}
		if len(clean) == 8 {
			break
		}
	}
	if len(clean) == 0 {
		return "t"
	}
	return string(clean)
}

// getSafeService vérifie l'existence et l'isolation d'un service
// Retourne une copie de sécurité pour éviter modifications externes
func (s *Service) getSafeService(id, tenantID string) (*ServiceItem, error) {
	item, exists := s.services[id]
	if !exists || item.TenantID != tenantID {
		s.logger.Printf("[catalog] Tentative d'accès non autorisé ou service introuvable: id=%s tenantID=%s", id, tenantID)
		return nil, ErrServiceNotFound
	}
	// Copie de sécurité
	itemCopy := *item
	return &itemCopy, nil
}

// GetServices liste les services en filtrant obligatoirement par tenant_id
// Supporte le filtrage par catégorie et la recherche textuelle
func (s *Service) GetServices(ctx context.Context, tenantID string, categoryFilter, search string) ([]*ServiceItem, error) {
	s.logger.Printf("[catalog] Récupération services: tenant=%s, catégorie=%s, recherche='%s'", tenantID, categoryFilter, search)

	if s.store != nil {
		s.logger.Printf("[catalog] Délégation récupération à PostgreSQL, tenant=%s", tenantID)
		return s.store.GetServices(ctx, tenantID, categoryFilter, search)
	}

	// Vérification et initialisation du tenant hors du verrou principal
	if !s.validateTenantExistence(tenantID) {
		s.seedDefaultServices(tenantID)
	}

	// Verrou lecture seul pour les accès en lecture
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []*ServiceItem
	searchLower := strings.ToLower(strings.TrimSpace(search))

	for _, item := range s.services {
		// ISOLATION MULTI-TENANT STRICTE
		if item.TenantID != tenantID {
			continue
		}

		// Filtre par catégorie si spécifié
		if categoryFilter != "" && categoryFilter != "all" && item.Category != categoryFilter {
			continue
		}

		// Filtre par recherche textuelle si spécifiée
		if searchLower != "" {
			matchesName := strings.Contains(strings.ToLower(item.Name), searchLower)
			matchesArticle := strings.Contains(strings.ToLower(item.Article), searchLower)
			matchesTreatment := strings.Contains(strings.ToLower(item.Treatment), searchLower)
			matchesDesc := strings.Contains(strings.ToLower(item.Description), searchLower)
			if !matchesName && !matchesArticle && !matchesTreatment && !matchesDesc {
				continue
			}
		}

		// Copie de sécurité pour éviter modification externe des données partagées
		itemCopy := *item
		result = append(result, &itemCopy)
	}

	s.logger.Printf("[catalog] %d services trouvés pour le tenant %s", len(result), tenantID)
	return result, nil
}

// GetServiceByID retourne une prestation par son ID avec isolation multi-tenant stricte
// Utilise getSafeService pour vérifications de sécurité et copie de données
func (s *Service) GetServiceByID(ctx context.Context, tenantID, id string) (*ServiceItem, error) {
	s.logger.Printf("[catalog] Consultation service: id=%s tenant=%s", id, tenantID)

	if s.store != nil {
		s.logger.Printf("[catalog] Délégation récupération à PostgreSQL, tenant=%s", tenantID)
		return s.store.GetService(ctx, tenantID, id)
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.getSafeService(id, tenantID)
}

// UpdateService met à jour les informations ou le tarif d'une prestation
func (s *Service) UpdateService(ctx context.Context, tenantID, id string, req UpdateServiceRequest) (*ServiceItem, error) {
	if s.store != nil {
		return s.store.UpdateService(ctx, tenantID, id, req)
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return nil, ErrEmptyName
	}
	if req.UnitPrice < 0 {
		return nil, ErrInvalidPrice
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	item, exists := s.services[id]
	if !exists || item.TenantID != tenantID {
		return nil, ErrServiceNotFound
	}

	item.Name = name
	if req.Article != "" {
		item.Article = req.Article
	}
	if req.Treatment != "" {
		item.Treatment = req.Treatment
	}
	if req.Category != "" {
		item.Category = req.Category
	}
	item.UnitPrice = req.UnitPrice
	if req.Unit != "" {
		item.Unit = req.Unit
	}
	if req.PricingModel != "" {
		item.PricingModel = req.PricingModel
	}
	if req.EstimatedHours > 0 {
		item.EstimatedHours = req.EstimatedHours
	}
	if req.IsActive != nil {
		item.IsActive = *req.IsActive
	}
	if req.IsExpressAvailable != nil {
		item.IsExpressAvailable = *req.IsExpressAvailable
	}
	if req.ExpressSurcharge != nil {
		item.ExpressSurcharge = *req.ExpressSurcharge
	}
	if req.IsPickupAvailable != nil {
		item.IsPickupAvailable = *req.IsPickupAvailable
	}
	if req.IsOutsourced != nil {
		item.IsOutsourced = *req.IsOutsourced
	}
	if req.RequiresQuote != nil {
		item.RequiresQuote = *req.RequiresQuote
	}
	if req.Description != "" {
		item.Description = req.Description
	}
	item.UpdatedAt = time.Now().UTC()

	return item, nil
}

// DeleteService supprime une prestation de la grille tarifaire
func (s *Service) DeleteService(ctx context.Context, tenantID, id string) error {
	if s.store != nil {
		return s.store.DeleteService(ctx, tenantID, id)
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	item, exists := s.services[id]
	if !exists || item.TenantID != tenantID {
		return ErrServiceNotFound
	}

	delete(s.services, id)
	return nil
}
