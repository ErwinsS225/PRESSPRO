import React, { useState } from 'react';
import { Tag, Plus, Clock, Sparkles } from 'lucide-react';
import { ServiceItem } from '../types';

interface ServicesViewProps {
  services: ServiceItem[];
  onAddService: (service: ServiceItem) => void;
}

export const ServicesView: React.FC<ServicesViewProps> = ({ services, onAddService }) => {
  const [isModalOpen, setIsModalOpen] = useState(false);
  const [name, setName] = useState('');
  const [category, setCategory] = useState('Lavage & Repassage');
  const [price, setPrice] = useState(1500);
  const [unit, setUnit] = useState('pièce');
  const [duration, setDuration] = useState(24);

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    if (!name) return;

    onAddService({
      id: `srv-${Date.now()}`,
      name,
      category,
      price: Number(price),
      unit,
      durationHours: Number(duration),
    });

    setName('');
    setIsModalOpen(false);
  };

  return (
    <div className="space-y-6">
      {/* Top Controls */}
      <div className="flex flex-col gap-3 rounded-lg border border-neutral-200 bg-white p-5 sm:flex-row sm:items-center sm:justify-between">
        <div>
          <h1 className="text-lg font-semibold tracking-tight text-neutral-900">
            Grille Tarifaire & Prestations
          </h1>
          <p className="mt-0.5 text-xs text-neutral-500">
            Catalogue des prestations de lavage, repassage et nettoyage à sec (tarifs en FCFA).
          </p>
        </div>

        <button
          onClick={() => setIsModalOpen(true)}
          className="flex items-center gap-1.5 self-start rounded-md bg-neutral-900 px-3 py-1.5 text-xs font-medium text-white hover:bg-neutral-800 transition-colors sm:self-auto"
        >
          <Plus className="h-3.5 w-3.5" />
          <span>Ajouter une Prestation</span>
        </button>
      </div>

      {/* Services Table */}
      <div className="rounded-lg border border-neutral-200 bg-white">
        <div className="overflow-x-auto">
          <table className="w-full text-left text-xs">
            <thead>
              <tr className="border-b border-neutral-100 bg-neutral-50/75 text-[11px] text-neutral-500 font-medium">
                <th className="px-5 py-2.5">Désignation du Vêtement / Linge</th>
                <th className="px-4 py-2.5">Catégorie de Soin</th>
                <th className="px-4 py-2.5">Unité de Facturation</th>
                <th className="px-4 py-2.5">Délai Estimé</th>
                <th className="px-5 py-2.5 text-right">Tarif Unitaire (FCFA)</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-neutral-100">
              {services.map((item) => (
                <tr key={item.id} className="hover:bg-neutral-50/50 transition-colors">
                  <td className="px-5 py-3.5 font-medium text-neutral-900">{item.name}</td>
                  <td className="px-4 py-3.5 text-neutral-600">{item.category}</td>
                  <td className="px-4 py-3.5 font-mono text-neutral-500">{item.unit}</td>
                  <td className="px-4 py-3.5 font-mono text-neutral-600">
                    <span className="flex items-center gap-1">
                      <Clock className="h-3 w-3 text-neutral-400" />
                      {item.durationHours} heures
                    </span>
                  </td>
                  <td className="px-5 py-3.5 text-right font-mono font-semibold text-neutral-900 tabular-nums">
                    {item.price.toLocaleString('fr-FR')} F
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </div>

      {/* Add Service Modal */}
      {isModalOpen && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-neutral-900/40 p-4 backdrop-blur-xs">
          <div className="w-full max-w-sm rounded-lg border border-neutral-200 bg-white p-5 shadow-lg">
            <h2 className="text-sm font-semibold text-neutral-900">Nouvelle Prestation</h2>
            <p className="mt-0.5 text-xs text-neutral-500">
              Ajoutez un tarif à la grille de votre pressing.
            </p>

            <form onSubmit={handleSubmit} className="mt-4 space-y-3">
              <div>
                <label className="block text-[11px] font-medium text-neutral-700">Désignation</label>
                <input
                  type="text"
                  required
                  placeholder="Ex: Veste en cuir, Couette 2 places..."
                  value={name}
                  onChange={(e) => setName(e.target.value)}
                  className="mt-1 w-full rounded-md border border-neutral-200 px-3 py-1.5 text-xs text-neutral-900 focus:border-neutral-900 focus:outline-hidden"
                />
              </div>

              <div>
                <label className="block text-[11px] font-medium text-neutral-700">Catégorie</label>
                <select
                  value={category}
                  onChange={(e) => setCategory(e.target.value)}
                  className="mt-1 w-full rounded-md border border-neutral-200 px-3 py-1.5 text-xs text-neutral-900 focus:border-neutral-900 focus:outline-hidden"
                >
                  <option value="Lavage & Repassage">Lavage & Repassage</option>
                  <option value="Nettoyage à sec">Nettoyage à sec</option>
                  <option value="Délicat">Délicat</option>
                  <option value="Linge de maison">Linge de maison</option>
                </select>
              </div>

              <div className="grid grid-cols-2 gap-3">
                <div>
                  <label className="block text-[11px] font-medium text-neutral-700">Prix unitaire (FCFA)</label>
                  <input
                    type="number"
                    required
                    min={100}
                    step={100}
                    value={price}
                    onChange={(e) => setPrice(Number(e.target.value))}
                    className="mt-1 w-full rounded-md border border-neutral-200 px-3 py-1.5 text-xs text-neutral-900 focus:border-neutral-900 focus:outline-hidden"
                  />
                </div>
                <div>
                  <label className="block text-[11px] font-medium text-neutral-700">Délai (heures)</label>
                  <input
                    type="number"
                    required
                    min={1}
                    value={duration}
                    onChange={(e) => setDuration(Number(e.target.value))}
                    className="mt-1 w-full rounded-md border border-neutral-200 px-3 py-1.5 text-xs text-neutral-900 focus:border-neutral-900 focus:outline-hidden"
                  />
                </div>
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
                  Ajouter
                </button>
              </div>
            </form>
          </div>
        </div>
      )}
    </div>
  );
};
