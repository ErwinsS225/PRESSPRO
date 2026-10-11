import { afterEach, describe, expect, it, vi } from 'vitest';
import { loginUser, PRESET_ACCOUNTS } from './authApi';

afterEach(() => {
  vi.unstubAllGlobals();
});

describe('loginUser — session issue du serveur uniquement', () => {
  it('retourne la session renvoyée par l’API (jeton, compte, établissement)', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn(async () => ({
        ok: true,
        status: 200,
        json: async () => ({
          token: 'jwt-reel',
          user: { id: 'usr-1', email: 'gerant@test.ci', full_name: 'Gérant', role: 'owner' },
          tenant: { id: 't-1', name: 'Pressing Réel', slug: 'pressing-reel' },
        }),
      }))
    );

    const session = await loginUser('gerant@test.ci', 'motdepasse');
    expect(session.token).toBe('jwt-reel');
    expect(session.tenant?.name).toBe('Pressing Réel');
  });

  it('refuse une connexion hors-ligne : aucun jeton ni établissement fabriqué', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn(async () => {
        throw new Error('réseau indisponible');
      })
    );

    await expect(loginUser('gerant@pressing-ivoire.ci', 'pass123')).rejects.toThrow(
      /Serveur PressingOS injoignable/
    );
  });

  it('refuse des identifiants rejetés par le serveur', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn(async () => ({ ok: false, status: 401, json: async () => ({}) }))
    );

    await expect(loginUser('inconnu@test.ci', 'mauvais')).rejects.toThrow(/Identifiants refusés/);
  });

  it('refuse une réponse incomplète au lieu d’inventer une session', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn(async () => ({ ok: true, status: 200, json: async () => ({ user: { id: 'usr-1' } }) }))
    );

    await expect(loginUser('gerant@test.ci', 'motdepasse')).rejects.toThrow(/incomplète/);
  });

  it('ne propose que les comptes de démonstration réellement embarqués côté serveur', () => {
    expect(PRESET_ACCOUNTS.map((a) => a.email)).toEqual([
      'gerant@pressing-ivoire.ci',
      'atelier@pressing-ivoire.ci',
      'admin@pressingos.ci',
    ]);
    // Aucun nom d'entreprise fictive n'est présenté à l'utilisateur.
    expect(PRESET_ACCOUNTS.map((a) => a.tenantName)).not.toContain('Pressing Ivoire Cocody');
  });
});
