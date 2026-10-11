import { beforeEach, describe, expect, it } from 'vitest';
import { getLocalServices, saveLocalServices } from './servicesApi';
import type { ServiceItem } from '../types';

const PRESTATION: ServiceItem = {
  id: 'srv-1',
  name: 'Chemise',
  article: 'Chemise',
  treatment: 'Lavage',
  category: 'Vêtements du quotidien',
  price: 1000,
  unit: 'pièce',
  pricingModel: 'piece',
  durationHours: 24,
  isActive: true,
  isExpressAvailable: false,
  expressSurcharge: 0,
  isPickupAvailable: true,
  isOutsourced: false,
  requiresQuote: false,
  description: '',
};

beforeEach(() => {
  localStorage.clear();
});

describe('catalogue local — aucune prestation de démonstration', () => {
  it('démarre vide lorsque rien n’a été enregistré', () => {
    expect(getLocalServices()).toEqual([]);
  });

  it('ne restitue que ce que l’établissement a réellement enregistré', () => {
    saveLocalServices([PRESTATION]);
    expect(getLocalServices()).toEqual([PRESTATION]);
  });

  it('ignore un cache corrompu sans injecter de données fictives', () => {
    localStorage.setItem('pressingos_catalog_services_v2', '{invalide');
    expect(getLocalServices()).toEqual([]);
  });
});
