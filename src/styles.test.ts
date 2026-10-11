import { readFileSync, readdirSync, statSync } from 'fs';
import { join } from 'path';
import { describe, expect, it } from 'vitest';

/**
 * Garde-fou du jeu de styles : les cadres de l'application doivent rester
 * « surélevés » (liseré d'ombre sur les bords + ombre portée) et les survols
 * doivent rester foncés. Un utilitaire supprimé par mégarde casserait
 * silencieusement l'aspect de toutes les vues — d'où ce contrôle.
 */
const RACINE = join(__dirname);

function fichiersTsx(dossier: string): string[] {
  return readdirSync(dossier).flatMap((entree) => {
    const chemin = join(dossier, entree);
    if (statSync(chemin).isDirectory()) return fichiersTsx(chemin);
    return chemin.endsWith('.tsx') && !chemin.includes('.test.') ? [chemin] : [];
  });
}

const CSS = readFileSync(join(RACINE, 'index.css'), 'utf8');
const SOURCES = fichiersTsx(RACINE).map((f) => readFileSync(f, 'utf8'));

describe('jeu de styles — cadres surélevés et survols foncés', () => {
  it('définit l’ombre des bords et le soulèvement au survol', () => {
    expect(CSS).toContain('@utility ps-elevated');
    expect(CSS).toMatch(/inset 0 0 0 1px rgb\(17 17 17 \/ 5%\)/);
    expect(CSS).toMatch(/&:hover\s*\{[\s\S]*translateY\(-2px\)/);
    // Variante sombre : liseré clair, ombre noire dense.
    expect(CSS).toMatch(/\.dark &[\s\S]*rgb\(255 255 255 \/ 8%\)/);
    expect(CSS).toMatch(/0 26px 60px -24px rgb\(0 0 0 \/ 100%\)/);
  });

  it('équipe les vues de l’application', () => {
    const total = SOURCES.reduce((n, s) => n + (s.match(/ps-elevated\b/g)?.length ?? 0), 0);
    expect(total).toBeGreaterThanOrEqual(60);
  });

  it('n’a plus de survols gris clair sur les éléments atteignables', () => {
    const restants: string[] = [];
    for (const source of SOURCES) {
      for (const motif of source.matchAll(
        /(?<![-\w:])hover:(?:bg|border)-(neutral-(?:50|100|200|300|400|600|700))\b/g
      )) {
        restants.push(motif[0]);
      }
    }
    expect(restants).toEqual([]);
  });

  it('conserve le liseré noir au survol des cartes', () => {
    const avecBordure = SOURCES.filter((s) => /ps-elevated/.test(s) && /hover:border-neutral-950/.test(s));
    expect(avecBordure.length).toBeGreaterThan(5);
  });
});
