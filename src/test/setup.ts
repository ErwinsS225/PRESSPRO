import '@testing-library/jest-dom/vitest';

// --- Émulation des API navigateur absentes de jsdom ---------------------------------
// embla-carousel (carrousel) interroge `window.matchMedia` et recharts s'appuie sur
// `ResizeObserver` pour mesurer son conteneur : sans ces deux ponts, les composants
// ReUI échoueraient au montage dans les tests alors qu'ils fonctionnent au navigateur.
if (!window.matchMedia) {
  window.matchMedia = ((query: string) => ({
    matches: false,
    media: query,
    onchange: null,
    addListener: () => {},
    removeListener: () => {},
    addEventListener: () => {},
    removeEventListener: () => {},
    dispatchEvent: () => false,
  })) as unknown as typeof window.matchMedia;
}

const globalTest = globalThis as unknown as Record<string, unknown>;

if (!globalTest.ResizeObserver) {
  class ResizeObserverPont {
    observe() {}
    unobserve() {}
    disconnect() {}
  }
  globalTest.ResizeObserver = ResizeObserverPont;
}

// embla-carousel déclenche aussi l'observation d'intersection des diapositives.
if (!globalTest.IntersectionObserver) {
  class IntersectionObserverPont {
    root = null;
    rootMargin = '';
    thresholds: number[] = [];
    observe() {}
    unobserve() {}
    disconnect() {}
    takeRecords() {
      return [];
    }
  }
  globalTest.IntersectionObserver = IntersectionObserverPont;
}
