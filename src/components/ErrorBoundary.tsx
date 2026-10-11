import React from 'react';

interface ErrorBoundaryState {
  erreur: Error | null;
}

/**
 * Garde-fou de dernier recours : une erreur de rendu n'efface plus l'interface
 * (écran blanc muet) mais affiche un message lisible et un bouton de reprise.
 */
export class ErrorBoundary extends React.Component<React.PropsWithChildren, ErrorBoundaryState> {
  state: ErrorBoundaryState = { erreur: null };

  static getDerivedStateFromError(erreur: Error): ErrorBoundaryState {
    return { erreur };
  }

  componentDidCatch(erreur: Error, infos: React.ErrorInfo) {
    console.error('Erreur de rendu interceptée par ErrorBoundary:', erreur, infos.componentStack);
  }

  private reinitialiser = () => {
    this.setState({ erreur: null });
  };

  render() {
    if (!this.state.erreur) return this.props.children;

    return (
      <div className="flex min-h-screen items-center justify-center bg-neutral-50 p-6 font-sans text-neutral-900 dark:bg-[#050505] dark:text-neutral-100">
        <div className="w-full max-w-md rounded-xl border border-neutral-200 bg-white p-6 shadow-sm dark:border-neutral-800 dark:bg-neutral-950">
          <h1 className="text-base font-semibold">Une erreur d’affichage est survenue</h1>
          <p className="mt-2 text-sm text-neutral-600 dark:text-neutral-400">
            L’application a intercepté l’erreur au lieu de rester sur un écran vide. Vos données enregistrées
            ne sont pas affectées.
          </p>
          <p className="mt-3 rounded-lg bg-neutral-100 px-3 py-2 font-mono text-xs text-neutral-700 dark:bg-neutral-900 dark:text-neutral-300">
            {this.state.erreur.message}
          </p>
          <div className="mt-4 flex gap-2">
            <button
              type="button"
              onClick={this.reinitialiser}
              className="rounded-lg border border-neutral-300 px-3 py-2 text-sm font-medium hover:bg-neutral-100 dark:border-neutral-700 dark:hover:bg-neutral-900"
            >
              Réessayer
            </button>
            <button
              type="button"
              onClick={() => window.location.reload()}
              className="rounded-lg bg-neutral-900 px-3 py-2 text-sm font-medium text-white hover:bg-neutral-800 dark:bg-neutral-100 dark:text-neutral-900"
            >
              Recharger la page
            </button>
          </div>
        </div>
      </div>
    );
  }
}
