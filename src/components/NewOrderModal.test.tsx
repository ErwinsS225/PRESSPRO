import { fireEvent, render, screen } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';
import { NewOrderModal } from './NewOrderModal';
import type { Customer, ServiceItem } from '../types';

const customers: Customer[] = ['client-a', 'client-b'].map((id) => ({
  id,
  fullName: 'Awa Kouadio',
  phone: '0700000000',
  neighborhood: '',
  totalOrders: 0,
  lastVisit: '',
  balanceDue: 0,
}));
const service: ServiceItem = {
  id: 'service',
  name: 'Chemise',
  category: 'Lavage',
  price: 1000,
  unit: 'pièce',
  durationHours: 24,
};

describe('sélection de client au dépôt', () => {
  it('transmet l’identifiant exact même si nom et téléphone sont identiques', () => {
    const onSubmit = vi.fn();
    render(
      <NewOrderModal customers={customers} services={[service]} onClose={vi.fn()} onSubmit={onSubmit} />
    );
    fireEvent.change(screen.getAllByRole('combobox')[0], { target: { value: 'client-b' } });
    fireEvent.click(screen.getByRole('button', { name: 'Valider le Dépôt' }));
    expect(onSubmit).toHaveBeenCalledWith(
      expect.objectContaining({ customerId: 'client-b', customerName: 'Awa Kouadio' })
    );
  });
});
