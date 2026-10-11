import React, { useState } from 'react';
import { Search, UserPlus, Phone, MapPin, Calendar, CreditCard } from 'lucide-react';
import { Customer } from '../types';

interface CustomersViewProps {
  customers: Customer[];
  onAddCustomer: (customer: Customer) => void;
}

export const CustomersView: React.FC<CustomersViewProps> = ({ customers, onAddCustomer }) => {
  const [searchQuery, setSearchQuery] = useState('');
  const [isModalOpen, setIsModalOpen] = useState(false);
  const [newFullName, setNewFullName] = useState('');
  const [newPhone, setNewPhone] = useState('');
  const [newNeighborhood, setNewNeighborhood] = useState('');

  const filteredCustomers = customers.filter(
    (c) =>
      c.fullName.toLowerCase().includes(searchQuery.toLowerCase()) ||
      c.phone.includes(searchQuery) ||
      c.neighborhood.toLowerCase().includes(searchQuery.toLowerCase())
  );

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    if (!newFullName || !newPhone) return;

    const newCust: Customer = {
      id: `cust-${Date.now()}`,
      fullName: newFullName,
      phone: newPhone,
      neighborhood: newNeighborhood || 'Abidjan',
      totalOrders: 0,
      lastVisit: 'Aujourd’hui',
      balanceDue: 0,
    };

    onAddCustomer(newCust);
    setNewFullName('');
    setNewPhone('');
    setNewNeighborhood('');
    setIsModalOpen(false);
  };

  return (
    <div className="space-y-6">
      {/* Top Controls */}
      <div className="flex flex-col gap-3 rounded-lg border border-neutral-200 bg-white p-5 sm:flex-row sm:items-center sm:justify-between">
        <div>
          <h1 className="text-lg font-semibold tracking-tight text-neutral-900">
            Fichier Clients & Fidélisation
          </h1>
          <p className="mt-0.5 text-xs text-neutral-500">
            Coordonnées des habitués, historique des dépôts et suivi des soldes dus.
          </p>
        </div>

        <button
          onClick={() => setIsModalOpen(true)}
          className="flex items-center gap-1.5 self-start rounded-md bg-neutral-900 px-3 py-1.5 text-xs font-medium text-white hover:bg-neutral-800 transition-colors sm:self-auto"
        >
          <UserPlus className="h-3.5 w-3.5" />
          <span>Nouveau Client</span>
        </button>
      </div>

      {/* Search Bar */}
      <div className="relative max-w-md">
        <Search className="absolute left-2.5 top-2.5 h-3.5 w-3.5 text-neutral-400" />
        <input
          type="text"
          placeholder="Rechercher par nom, téléphone, quartier..."
          value={searchQuery}
          onChange={(e) => setSearchQuery(e.target.value)}
          className="w-full rounded-md border border-neutral-200 bg-white pl-8 pr-3 py-1.5 text-xs text-neutral-900 placeholder:text-neutral-400 focus:border-neutral-900 focus:outline-hidden"
        />
      </div>

      {/* Customers Table */}
      <div className="rounded-lg border border-neutral-200 bg-white">
        <div className="overflow-x-auto">
          <table className="w-full text-left text-xs">
            <thead>
              <tr className="border-b border-neutral-100 bg-neutral-50/75 text-[11px] text-neutral-500 font-medium">
                <th className="px-5 py-2.5">Nom Complet</th>
                <th className="px-4 py-2.5">Contact GSM / WhatsApp</th>
                <th className="px-4 py-2.5">Quartier (Abidjan)</th>
                <th className="px-4 py-2.5 text-center">Historique</th>
                <th className="px-4 py-2.5">Dernière Visite</th>
                <th className="px-5 py-2.5 text-right">Solde Dû</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-neutral-100">
              {filteredCustomers.length === 0 ? (
                <tr>
                  <td colSpan={6} className="px-5 py-10 text-center text-neutral-500">
                    Aucun client ne correspond à votre recherche.
                  </td>
                </tr>
              ) : (
                filteredCustomers.map((c) => (
                  <tr key={c.id} className="hover:bg-neutral-50/50 transition-colors">
                    <td className="px-5 py-3.5 font-medium text-neutral-900">{c.fullName}</td>
                    <td className="px-4 py-3.5 font-mono text-neutral-600">{c.phone}</td>
                    <td className="px-4 py-3.5 text-neutral-600">{c.neighborhood}</td>
                    <td className="px-4 py-3.5 text-center font-mono text-neutral-700 tabular-nums">
                      {c.totalOrders} dépôts
                    </td>
                    <td className="px-4 py-3.5 text-neutral-500">{c.lastVisit}</td>
                    <td className="px-5 py-3.5 text-right font-mono tabular-nums">
                      {c.balanceDue > 0 ? (
                        <span className="font-semibold text-neutral-900">
                          {c.balanceDue.toLocaleString('fr-FR')} F
                        </span>
                      ) : (
                        <span className="text-neutral-400">À jour (0 F)</span>
                      )}
                    </td>
                  </tr>
                ))
              )}
            </tbody>
          </table>
        </div>
      </div>

      {/* New Customer Modal */}
      {isModalOpen && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-neutral-900/40 p-4 backdrop-blur-xs">
          <div className="w-full max-w-sm rounded-lg border border-neutral-200 bg-white p-5 shadow-lg">
            <h2 className="text-sm font-semibold text-neutral-900">Ajouter un Client</h2>
            <p className="mt-0.5 text-xs text-neutral-500">
              Enregistrez un nouvel habitué pour votre pressing.
            </p>

            <form onSubmit={handleSubmit} className="mt-4 space-y-3">
              <div>
                <label className="block text-[11px] font-medium text-neutral-700">Nom & Prénoms</label>
                <input
                  type="text"
                  required
                  placeholder="Ex: Koffi Emmanuel"
                  value={newFullName}
                  onChange={(e) => setNewFullName(e.target.value)}
                  className="mt-1 w-full rounded-md border border-neutral-200 px-3 py-1.5 text-xs text-neutral-900 focus:border-neutral-900 focus:outline-hidden"
                />
              </div>

              <div>
                <label className="block text-[11px] font-medium text-neutral-700">Numéro Téléphone (+225)</label>
                <input
                  type="text"
                  required
                  placeholder="Ex: +225 07 12 34 56 78"
                  value={newPhone}
                  onChange={(e) => setNewPhone(e.target.value)}
                  className="mt-1 w-full rounded-md border border-neutral-200 px-3 py-1.5 text-xs text-neutral-900 focus:border-neutral-900 focus:outline-hidden"
                />
              </div>

              <div>
                <label className="block text-[11px] font-medium text-neutral-700">Quartier de résidence</label>
                <input
                  type="text"
                  placeholder="Ex: Cocody Angré, Marcory, Plateau..."
                  value={newNeighborhood}
                  onChange={(e) => setNewNeighborhood(e.target.value)}
                  className="mt-1 w-full rounded-md border border-neutral-200 px-3 py-1.5 text-xs text-neutral-900 focus:border-neutral-900 focus:outline-hidden"
                />
              </div>

              <div className="mt-5 flex justify-end gap-2 pt-2 border-t border-neutral-100">
                <button
                  type="button"
                  onClick={() => setIsModalOpen(false)}
                  className="rounded-md border border-neutral-200 px-3 py-1.5 text-xs font-medium text-neutral-700 hover:bg-neutral-50"
                >
                  Annuler
                </button>
                <button
                  type="submit"
                  className="rounded-md bg-neutral-900 px-3 py-1.5 text-xs font-medium text-white hover:bg-neutral-800"
                >
                  Enregistrer
                </button>
              </div>
            </form>
          </div>
        </div>
      )}
    </div>
  );
};
