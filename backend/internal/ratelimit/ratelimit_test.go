package ratelimit

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestAllow_LimitsPerKey(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 10, 11, 12, 0, 0, 0, time.UTC)
	l := New(3, 5*time.Minute)
	l.now = func() time.Time { return now }

	// 3 tentatives autorisées, la 4e est refusée.
	for i := 0; i < 3; i++ {
		ok, _ := l.Allow("ip:1.2.3.4")
		assert.True(t, ok, "tentative %d autorisée", i+1)
	}
	ok, retry := l.Allow("ip:1.2.3.4")
	assert.False(t, ok, "4e tentative refusée")
	assert.Positive(t, retry)

	// Une autre clé n'est pas pénalisée.
	okOther, _ := l.Allow("ip:5.6.7.8")
	assert.True(t, okOther)
}

func TestAllow_WindowExpires(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 10, 11, 12, 0, 0, 0, time.UTC)
	l := New(1, time.Minute)
	l.now = func() time.Time { return now }

	ok, _ := l.Allow("k")
	assert.True(t, ok)
	ok, _ = l.Allow("k")
	assert.False(t, ok)

	// Après la fenêtre, la tentative redevient possible.
	now = now.Add(61 * time.Second)
	ok, _ = l.Allow("k")
	assert.True(t, ok)
}

func TestReset_ClearsCounter(t *testing.T) {
	t.Parallel()
	l := New(1, time.Minute)
	ok, _ := l.Allow("k")
	assert.True(t, ok)
	ok, _ = l.Allow("k")
	assert.False(t, ok)

	l.Reset("k")
	ok, _ = l.Allow("k")
	assert.True(t, ok, "après Reset (connexion réussie), la clé repart à zéro")
}

func TestNilLimiter_AllowsEverything(t *testing.T) {
	t.Parallel()
	var l *Limiter
	ok, _ := l.Allow("k")
	assert.True(t, ok, "un limiteur nil ne doit jamais bloquer (mode dégradé)")
	l.Reset("k") // ne doit pas paniquer
}
