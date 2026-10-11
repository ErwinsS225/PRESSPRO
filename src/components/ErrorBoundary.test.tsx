import { render, screen } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { ErrorBoundary } from './ErrorBoundary';

function EnfantFautif(): React.ReactElement {
  throw new Error('composant en échec');
}

describe('ErrorBoundary', () => {
  beforeEach(() => {
    // React journalise l'erreur interceptée : on garde la sortie de test propre.
    vi.spyOn(console, 'error').mockImplementation(() => {});
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it('affiche un message lisible au lieu d’un écran blanc', () => {
    render(
      <ErrorBoundary>
        <EnfantFautif />
      </ErrorBoundary>
    );

    expect(screen.getByText('Une erreur d’affichage est survenue')).toBeInTheDocument();
    expect(screen.getByText(/composant en échec/)).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Recharger la page' })).toBeInTheDocument();
  });

  it('laisse passer le rendu normal', () => {
    render(
      <ErrorBoundary>
        <p>contenu nominal</p>
      </ErrorBoundary>
    );

    expect(screen.getByText('contenu nominal')).toBeInTheDocument();
    expect(screen.queryByText('Une erreur d’affichage est survenue')).not.toBeInTheDocument();
  });
});
