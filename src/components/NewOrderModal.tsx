import React, { useState } from 'react';
import { X, Plus, Trash2 } from 'lucide-react';
import { Customer, ServiceItem, Order, OrderItem } from '../types';

interface NewOrderModalProps {
  customers: Customer[];
  services: ServiceItem[];
  onClose: () => void;
  onSubmit: (order: Order) => void;
}

export const NewOrderModal: React.FC<NewOrderModalProps> = ({ customers, services, onClose, onSubmit }) => {
  const [selectedCustomer, setSelectedCustomer] = useState(customers[0]?.fullName || '');
  const [phone, setPhone] = useState(customers[0]?.phone || '+225 07 00 00 00 00');
  const [items, setItems] = useState<OrderItem[]>([
    {
      id: 'item-1',
      serviceName: services[0]?.name || 'Chemise',
      quantity: 2,
      unitPrice: services[0]?.price || 1000,
      subtotal: (services[0]?.price || 1000) * 2,
    },
  ]);
  const [paymentStatus, setPaymentStatus] = useState<'paid' | 'unpaid' | 'partially_paid'>('paid');
  const [notes, setNotes] = useState('');

  const handleCustomerChange = (name: string) => {
    setSelectedCustomer(name);
    const found = customers.find((c) => c.fullName === name);
    if (found) setPhone(found.phone);
  };

  const handleAddItem = () => {
    const srv = services[0];
    setItems([
      ...items,
      {
        id: `item-${Date.now()}`,
        serviceName: srv.name,
        quantity: 1,
        unitPrice: srv.price,
        subtotal: srv.price,
      },
    ]);
  };

  const handleRemoveItem = (index: number) => {
    setItems(items.filter((_, i) => i !== index));
  };

  const handleItemChange = (index: number, srvName: string) => {
    const srv = services.find((s) => s.name === srvName);
    if (!srv) return;
    const next = [...items];
    next[index] = {
      ...next[index],
      serviceName: srv.name,
      unitPrice: srv.price,
      subtotal: srv.price * next[index].quantity,
    };
    setItems(next);
  };

  const handleQuantityChange = (index: number, qty: number) => {
    const safeQty = Math.max(1, qty);
    const next = [...items];
    next[index] = {
      ...next[index],
      quantity: safeQty,
      subtotal: next[index].unitPrice * safeQty,
    };
    setItems(next);
  };

  const totalAmount = items.reduce((sum, it) => sum + it.subtotal, 0);

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    if (items.length === 0) return;

    const newOrder: Order = {
      id: `ord-${Date.now()}`,
      orderNumber: `CMD-2026-${Math.floor(100 + Math.random() * 900)}`,
      customerName: selectedCustomer,
      customerPhone: phone,
      items,
      status: 'received',
      paymentStatus,
      totalAmount,
      paidAmount:
        paymentStatus === 'paid'
          ? totalAmount
          : paymentStatus === 'partially_paid'
            ? Math.floor(totalAmount / 2)
            : 0,
      receivedAt: 'À l’instant',
      dueDate: 'Dans 2 jours (18:00)',
      notes: notes || undefined,
    };

    onSubmit(newOrder);
    onClose();
  };

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-neutral-900/40 dark:bg-black/75 p-4 backdrop-blur-xs">
      <div className="w-full max-w-lg rounded-lg border border-neutral-200 bg-white p-5 shadow-lg dark:border-neutral-800 dark:bg-[#111111] max-h-[90vh] overflow-y-auto shadow-2xl shadow-neutral-950/25 dark:shadow-black/80 ring-1 ring-black/5 dark:ring-white/10">
        <div className="flex items-center justify-between border-b border-neutral-100 dark:border-neutral-800 pb-3">
          <div>
            <h2 className="text-sm font-semibold text-neutral-900 dark:text-neutral-100">
              Enregistrer un Dépôt de Linge
            </h2>
            <p className="text-xs text-neutral-500 dark:text-neutral-400">
              Création d'un ticket de pressing avec calcul automatique.
            </p>
          </div>
          <button
            onClick={onClose}
            className="rounded p-1 text-neutral-400 hover:text-neutral-950 dark:text-neutral-500 dark:hover:text-neutral-200"
          >
            <X className="h-4 w-4" />
          </button>
        </div>

        <form onSubmit={handleSubmit} className="mt-4 space-y-4">
          <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
            <div>
              <label className="block text-[11px] font-medium text-neutral-700 dark:text-neutral-300">
                Client
              </label>
              <select
                value={selectedCustomer}
                onChange={(e) => handleCustomerChange(e.target.value)}
                className="mt-1 w-full rounded-md border border-neutral-200 px-3 py-1.5 text-xs text-neutral-900 focus:border-neutral-900 focus:outline-hidden dark:border-neutral-800 dark:bg-neutral-900 dark:text-neutral-100 dark:focus:border-neutral-100"
              >
                {customers.map((c) => (
                  <option key={c.id} value={c.fullName}>
                    {c.fullName}
                  </option>
                ))}
              </select>
            </div>

            <div>
              <label className="block text-[11px] font-medium text-neutral-700 dark:text-neutral-300">
                Téléphone de contact
              </label>
              <input
                type="text"
                value={phone}
                onChange={(e) => setPhone(e.target.value)}
                className="mt-1 w-full rounded-md border border-neutral-200 px-3 py-1.5 text-xs text-neutral-900 focus:border-neutral-900 focus:outline-hidden dark:border-neutral-800 dark:bg-neutral-900 dark:text-neutral-100 dark:focus:border-neutral-100"
              />
            </div>
          </div>

          {/* Items Section */}
          <div>
            <div className="flex items-center justify-between mb-2">
              <label className="text-[11px] font-medium text-neutral-700 dark:text-neutral-300">
                Vêtements déposés
              </label>
              <button
                type="button"
                onClick={handleAddItem}
                className="flex items-center gap-1 text-[11px] text-neutral-600 hover:text-neutral-950 dark:text-neutral-400 dark:hover:text-neutral-200"
              >
                <Plus className="h-3 w-3" />
                <span>Ajouter un vêtement</span>
              </button>
            </div>

            <div className="space-y-2">
              {items.map((item, idx) => (
                <div
                  key={item.id}
                  className="flex items-center gap-2 rounded-md border border-neutral-100 bg-neutral-50 p-2 dark:border-neutral-800 dark:bg-neutral-900/60"
                >
                  <select
                    value={item.serviceName}
                    onChange={(e) => handleItemChange(idx, e.target.value)}
                    className="flex-1 rounded-md border border-neutral-200 bg-white px-2 py-1 text-xs text-neutral-900 dark:border-neutral-700 dark:bg-neutral-800 dark:text-neutral-100 hover:border-neutral-950 dark:hover:border-white/70 ps-elevated"
                  >
                    {services
                      .filter((s) => s.isActive !== false)
                      .map((s) => (
                        <option key={s.id} value={s.name}>
                          {s.name} — {s.price.toLocaleString('fr-FR')} FCFA ({s.unit})
                        </option>
                      ))}
                  </select>

                  <input
                    type="number"
                    min={1}
                    value={item.quantity}
                    onChange={(e) => handleQuantityChange(idx, parseInt(e.target.value, 10))}
                    className="w-16 rounded-md border border-neutral-200 bg-white px-2 py-1 text-xs text-center font-mono dark:border-neutral-700 dark:bg-neutral-800 dark:text-neutral-100 hover:border-neutral-950 dark:hover:border-white/70 ps-elevated"
                  />

                  <div className="w-24 text-right font-mono text-xs font-medium text-neutral-900 dark:text-neutral-100 tabular-nums">
                    {item.subtotal.toLocaleString('fr-FR')} F
                  </div>

                  {items.length > 1 && (
                    <button
                      type="button"
                      onClick={() => handleRemoveItem(idx)}
                      className="text-neutral-400 hover:text-neutral-950 dark:text-neutral-500 dark:hover:text-neutral-200 p-1"
                    >
                      <Trash2 className="h-3.5 w-3.5" />
                    </button>
                  )}
                </div>
              ))}
            </div>
          </div>

          <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
            <div>
              <label className="block text-[11px] font-medium text-neutral-700 dark:text-neutral-300">
                Règlement au Dépôt
              </label>
              <select
                value={paymentStatus}
                onChange={(e) => setPaymentStatus(e.target.value as any)}
                className="mt-1 w-full rounded-md border border-neutral-200 px-3 py-1.5 text-xs text-neutral-900 focus:border-neutral-900 focus:outline-hidden dark:border-neutral-800 dark:bg-neutral-900 dark:text-neutral-100 dark:focus:border-neutral-100"
              >
                <option value="paid">Payé d'avance (Total)</option>
                <option value="partially_paid">Acompte versé (50%)</option>
                <option value="unpaid">Paiement au retrait</option>
              </select>
            </div>

            <div>
              <label className="block text-[11px] font-medium text-neutral-700 dark:text-neutral-300">
                Instructions particulières
              </label>
              <input
                type="text"
                placeholder="Ex: Tache café, col dur..."
                value={notes}
                onChange={(e) => setNotes(e.target.value)}
                className="mt-1 w-full rounded-md border border-neutral-200 px-3 py-1.5 text-xs text-neutral-900 focus:border-neutral-900 focus:outline-hidden dark:border-neutral-800 dark:bg-neutral-900 dark:text-neutral-100 dark:focus:border-neutral-100"
              />
            </div>
          </div>

          <div className="flex items-center justify-between border-t border-neutral-100 dark:border-neutral-800 pt-3">
            <div>
              <span className="text-[11px] text-neutral-500 dark:text-neutral-400">Total à payer :</span>
              <div className="font-mono text-base font-bold text-neutral-900 dark:text-neutral-100 tabular-nums">
                {totalAmount.toLocaleString('fr-FR')} FCFA
              </div>
            </div>

            <div className="flex items-center gap-2">
              <button
                type="button"
                onClick={onClose}
                className="rounded-md border border-neutral-200 px-3 py-1.5 text-xs font-medium text-neutral-700 hover:bg-neutral-900/[0.06] dark:border-neutral-700 dark:text-neutral-300 dark:hover:bg-neutral-800"
              >
                Annuler
              </button>
              <button
                type="submit"
                className="rounded-md bg-neutral-900 px-3 py-1.5 text-xs font-medium text-white hover:bg-black dark:bg-neutral-100 dark:text-neutral-900 dark:hover:bg-white"
              >
                Valider le Dépôt
              </button>
            </div>
          </div>
        </form>
      </div>
    </div>
  );
};
