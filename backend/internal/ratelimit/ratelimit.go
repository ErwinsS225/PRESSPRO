// Package ratelimit fournit un limiteur de débit en mémoire (fenêtre glissante
// simple) destiné à ralentir les attaques par force brute sur l'authentification.
//
// Choix d'implémentation : aucune dépendance externe, fenêtre fixe par clé
// (IP, IP+email…), purge paresseuse des entrées expirées. Le limiteur est
// thread-safe et adapté à un déploiement mono-instance ; en multi-instance, il
// conviendra de le remplacer par un compteur partagé (Redis).
package ratelimit

import (
	"sync"
	"time"
)

// Limiter compte les tentatives par clé sur une fenêtre glissante.
type Limiter struct {
	mu       sync.Mutex
	entries  map[string]*entry
	max      int
	window   time.Duration
	now      func() time.Time
	lastGC   time.Time
	gcPeriod time.Duration
}

type entry struct {
	count       int
	windowStart time.Time
}

// New crée un limiteur autorisant `max` tentatives par `window` pour chaque clé.
func New(max int, window time.Duration) *Limiter {
	return &Limiter{
		entries:  make(map[string]*entry),
		max:      max,
		window:   window,
		now:      time.Now,
		gcPeriod: 10 * time.Minute,
	}
}

// Allow enregistre une tentative pour la clé et indique si elle est autorisée.
// Lorsqu'elle est refusée, la durée de réessai (Retry-After) est retournée.
func (l *Limiter) Allow(key string) (allowed bool, retryAfter time.Duration) {
	if l == nil || l.max <= 0 {
		return true, 0
	}
	l.mu.Lock()
	defer l.mu.Unlock()

	now := l.now()
	l.gcLocked(now)

	e, ok := l.entries[key]
	if !ok || now.Sub(e.windowStart) >= l.window {
		l.entries[key] = &entry{count: 1, windowStart: now}
		return true, 0
	}
	if e.count >= l.max {
		return false, l.window - now.Sub(e.windowStart)
	}
	e.count++
	return true, 0
}

// Reset efface le compteur d'une clé (ex. après une connexion réussie).
func (l *Limiter) Reset(key string) {
	if l == nil {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.entries, key)
}

// gcLocked purge les fenêtres expirées (appelé sous verrou).
func (l *Limiter) gcLocked(now time.Time) {
	if now.Sub(l.lastGC) < l.gcPeriod {
		return
	}
	l.lastGC = now
	for k, e := range l.entries {
		if now.Sub(e.windowStart) >= l.window {
			delete(l.entries, k)
		}
	}
}
