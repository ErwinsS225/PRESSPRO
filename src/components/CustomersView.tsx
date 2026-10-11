import React, { useState } from 'react';
import { Search, UserPlus } from 'lucide-react';
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
      <div className="flex flex-col gap-3 rounded-lg border border-neutral-200 bg-white p-5 dark:border-neutral-800 dark:bg-[#111111] sm:flex-row sm:items-center sm:justify-between hover:border-neutral-950 dark:hover:border-white/70 ps-elevated">
        <div>
          <h1 className="text-lg font-semibold tracking-tight text-neutral-900 dark:text-neutral-100">
            Fichier Clients & Fidélisation
          </h1>
          <p className="mt-0.5 text-xs text-neutral-500 dark:text-neutral-400">
            Coordonnées des habitués, historique des dépôts et suivi des soldes dus.
          </p>
        </div>

        <button
          onClick={() => setIsModalOpen(true)}
          className="flex items-center gap-1.5 self-start rounded-md bg-neutral-900 px-3 py-1.5 text-xs font-medium text-white hover:bg-black dark:bg-neutral-100 dark:text-neutral-900 dark:hover:bg-white transition-colors sm:self-auto"
        >
          <UserPlus className="h-3.5 w-3.5" />
          <span>Nouveau Client</span>
        </button>
      </div>

      {/* Search Bar */}
      <div className="relative max-w-md">
        <Search className="absolute left-2.5 top-2.5 h-3.5 w-3.5 text-neutral-400 dark:text-neutral-500" />
        <input
          type="text"
          placeholder="Rechercher par nom, téléphone, quartier..."
          value={searchQuery}
          onChange={(e) => setSearchQuery(e.target.value)}
          className="w-full rounded-md border border-neutral-200 bg-white pl-8 pr-3 py-1.5 text-xs text-neutral-900 placeholder:text-neutral-400 focus:border-neutral-900 focus:outline-hidden dark:border-neutral-800 dark:bg-neutral-900 dark:text-neutral-100 dark:placeholder:text-neutral-500 dark:focus:border-neutral-100"
        />
      </div>

      {/* Customers Table */}
      <div className="rounded-lg border border-neutral-200 bg-white dark:border-neutral-800 dark:bg-[#111111]">
        <div className="overflow-x-auto">
          <table className="w-full text-left text-xs">
            <thead>
              <tr className="border-b border-neutral-100 bg-neutral-50/75 text-[11px] text-neutral-500 font-medium dark:border-neutral-800 dark:bg-neutral-900/60 dark:text-neutral-400">
                <th className="px-5 py-2.5">Nom Complet</th>
                <th className="px-4 py-2.5">Contact GSM / WhatsApp</th>
                <th className="px-4 py-2.5">Quartier (Abidjan)</th>
                <th className="px-4 py-2.5 text-center">Historique</th>
                <th className="px-4 py-2.5">Dernière Visite</th>
                <th className="px-5 py-2.5 text-right">Solde Dû</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-neutral-100 dark:divide-neutral-800">
              {filteredCustomers.length === 0 ? (
                <tr>
                  <td colSpan={6} className="px-5 py-10 text-center text-neutral-500 dark:text-neutral-400">
                    Aucun client ne correspond à votre recherche.
                  </td>
                </tr>
              ) : (
                filteredCustomers.map((c) => (
                  <tr
                    key={c.id}
                    className="hover:bg-neutral-900/[0.06] dark:hover:bg-neutral-800/40 transition-colors"
                  >
                    <td className="px-5 py-3.5 font-medium text-neutral-900 dark:text-neutral-100">
                      {c.fullName}
                    </td>
                    <td className="px-4 py-3.5 font-mono text-neutral-600 dark:text-neutral-400">
                      {c.phone}
                    </td>
                    <td className="px-4 py-3.5 text-neutral-600 dark:text-neutral-400">{c.neighborhood}</td>
                    <td className="px-4 py-3.5 text-center font-mono text-neutral-700 dark:text-neutral-300 tabular-nums">
                      {c.totalOrders} dépôts
                    </td>
                    <td className="px-4 py-3.5 text-neutral-500 dark:text-neutral-400">{c.lastVisit}</td>
                    <td className="px-5 py-3.5 text-right font-mono tabular-nums">
                      {c.balanceDue > 0 ? (
                        <span className="font-semibold text-neutral-900 dark:text-neutral-100">
                          {c.balanceDue.toLocaleString('fr-FR')} F
                        </span>
                      ) : (
                        <span className="text-neutral-400 dark:text-neutral-500">À jour (0 F)</span>
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
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-neutral-900/40 dark:bg-black/70 p-4 backdrop-blur-xs">
          <div className="w-full max-w-sm rounded-lg border border-neutral-200 bg-white p-5 shadow-lg dark:border-neutral-800 dark:bg-[#111111] shadow-2xl shadow-neutral-950/25 dark:shadow-black/80 ring-1 ring-black/5 dark:ring-white/10">
            <h2 className="text-sm font-semibold text-neutral-900 dark:text-neutral-100">
              Ajouter un Client
            </h2>
            <p className="mt-0.5 text-xs text-neutral-500 dark:text-neutral-400">
              Enregistrez un nouvel habitué pour votre pressing.
            </p>

            <form onSubmit={handleSubmit} className="mt-4 space-y-3">
              <div>
                <label className="block text-[11px] font-medium text-neutral-700 dark:text-neutral-300">
                  Nom & Prénoms
                </label>
                <input
                  type="text"
                  required
                  placeholder="Ex: Koffi Emmanuel"
                  value={newFullName}
                  onChange={(e) => setNewFullName(e.target.value)}
                  className="mt-1 w-full rounded-md border border-neutral-200 px-3 py-1.5 text-xs text-neutral-900 focus:border-neutral-900 focus:outline-hidden dark:border-neutral-800 dark:bg-neutral-900 dark:text-neutral-100 dark:focus:border-neutral-100"
                />
              </div>

              <div>
                <label className="block text-[11px] font-medium text-neutral-700 dark:text-neutral-300">
                  Numéro Téléphone (+225)
                </label>
                <input
                  type="text"
                  required
                  placeholder="Ex: +225 07 12 34 56 78"
                  value={newPhone}
                  onChange={(e) => setNewPhone(e.target.value)}
                  className="mt-1 w-full rounded-md border border-neutral-200 px-3 py-1.5 text-xs text-neutral-900 focus:border-neutral-900 focus:outline-hidden dark:border-neutral-800 dark:bg-neutral-900 dark:text-neutral-100 dark:focus:border-neutral-100"
                />
              </div>

              <div>
                <label className="block text-[11px] font-medium text-neutral-700 dark:text-neutral-300">
                  Quartier de résidence
                </label>
                <input
                  type="text"
                  placeholder="Ex: Cocody Angré, Marcory, Plateau..."
                  value={newNeighborhood}
                  onChange={(e) => setNewNeighborhood(e.target.value)}
                  className="mt-1 w-full rounded-md border border-neutral-200 px-3 py-1.5 text-xs text-neutral-900 focus:border-neutral-900 focus:outline-hidden dark:border-neutral-800 dark:bg-neutral-900 dark:text-neutral-100 dark:focus:border-neutral-100"
                />
              </div>

              <div className="mt-5 flex justify-end gap-2 pt-2 border-t border-neutral-100 dark:border-neutral-800">
                <button
                  type="button"
                  onClick={() => setIsModalOpen(false)}
                  className="rounded-md border border-neutral-200 px-3 py-1.5 text-xs font-medium text-neutral-700 hover:bg-neutral-900/[0.06] dark:border-neutral-700 dark:text-neutral-300 dark:hover:bg-neutral-800"
                >
                  Annuler
                </button>
                <button
                  type="submit"
                  className="rounded-md bg-neutral-900 px-3 py-1.5 text-xs font-medium text-white hover:bg-black dark:bg-neutral-100 dark:text-neutral-900 dark:hover:bg-white"
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
