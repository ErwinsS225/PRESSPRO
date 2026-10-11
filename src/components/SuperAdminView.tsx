import React, { useState } from 'react';
import {
  Building2,
  TrendingUp,
  Shield,
  Plus,
  CheckCircle,
  Search,
} from 'lucide-react';
import { Tenant, User } from '../types';
import { canAccessSuperadmin } from '../lib/permissions';

interface SuperAdminViewProps {
  onSelectTenantView: (tenantId: string) => void;
  currentUser: User;
}

export const SuperAdminView: React.FC<SuperAdminViewProps> = ({ onSelectTenantView, currentUser }) => {
  const [tenants, setTenants] = useState<Tenant[]>([
    {
      id: 'a0000000-0000-0000-0000-000000000001',
      name: 'Pressing Ivoire Cocody',
      slug: 'pressing-ivoire-cocody',
      phone: '+225 07 00 11 22 33',
      city: 'Abidjan (Cocody Angré)',
      currency: 'XOF',
      status: 'active',
      ownerEmail: 'gerant@pressing-ivoire.ci',
      monthlyPlan: 'Pro (45 000 FCFA/mois)',
    },
    {
      id: 'a0000000-0000-0000-0000-000000000002',
      name: 'Buanderie Moderne du Plateau',
      slug: 'buanderie-plateau',
      phone: '+225 05 44 22 11 88',
      city: 'Abidjan (Plateau)',
      currency: 'XOF',
      status: 'active',
      ownerEmail: 'plateau@buanderie.ci',
      monthlyPlan: 'Entreprise (75 000 FCFA/mois)',
    },
    {
      id: 'a0000000-0000-0000-0000-000000000003',
      name: 'Clean Express Zone 4',
      slug: 'clean-express-zone4',
      phone: '+225 01 22 33 44 55',
      city: 'Abidjan (Marcory Zone 4)',
      currency: 'XOF',
      status: 'active',
      ownerEmail: 'contact@cleanexpress.ci',
      monthlyPlan: 'Pro (45 000 FCFA/mois)',
    },
    {
      id: 'a0000000-0000-0000-0000-000000000004',
      name: 'Pressing Étoile de Yopougon',
      slug: 'pressing-etoile-yop',
      phone: '+225 07 55 66 77 88',
      city: 'Abidjan (Yopougon Selmer)',
      currency: 'XOF',
      status: 'trial',
      ownerEmail: 'yopougon@etoile.ci',
      monthlyPlan: 'Essai Gratuit (14 jours)',
    },
    {
      id: 'a0000000-0000-0000-0000-000000000005',
      name: 'Blanchisserie Riviera Golf',
      slug: 'blanchisserie-riviera',
      phone: '+225 07 11 99 88 77',
      city: 'Abidjan (Riviera Golf)',
      currency: 'XOF',
      status: 'suspended',
      ownerEmail: 'golf@blanchisserie.ci',
      monthlyPlan: 'Starter (25 000 FCFA/mois)',
    },
  ]);

  const [searchQuery, setSearchQuery] = useState('');
  const [isModalOpen, setIsModalOpen] = useState(false);
  const [newTenantName, setNewTenantName] = useState('');
  const [newCity, setNewCity] = useState('Abidjan (Cocody)');
  const [newPhone, setNewPhone] = useState('');
  const [newEmail, setNewEmail] = useState('');

  const toggleStatus = (id: string) => {
    setTenants((prev) =>
      prev.map((t) => {
        if (t.id !== id) return t;
        const nextStatus = t.status === 'active' ? 'suspended' : 'active';
        return { ...t, status: nextStatus };
      })
    );
  };

  // Contrôle d'accès : seul un super-utilisateur de la plateforme peut ouvrir
  // cette vue (en dehors du filtrage du menu, pour défense en profondeur).
  const isSuperadmin = canAccessSuperadmin(currentUser);
  if (!isSuperadmin) {
    return (
      <div className="flex h-full flex-col items-center justify-center rounded-lg border border-neutral-200 bg-white p-6 text-center dark:border-neutral-800 dark:bg-[#0B0B0B]">
        <Shield className="h-8 w-8 text-neutral-400 dark:text-neutral-500" />
        <h1 className="mt-3 text-lg font-semibold text-neutral-900 dark:text-neutral-100">
          Accès refusé
        </h1>
        <p className="mt-1 text-xs text-neutral-500 dark:text-neutral-400">
          Vous n&apos;avez pas l&apos;autorisation requise pour accéder à
          l&apos;Espace Éditeur SaaS. Seuls les comptes de super-utilisateur de la
          plateforme peuvent y accéder.
        </p>
      </div>
    );
  }

  const handleCreateTenant = (e: React.FormEvent) => {
    e.preventDefault();
    if (!newTenantName || !newEmail) return;

    const newT: Tenant = {
      id: `tenant-${Date.now()}`,
      name: newTenantName,
      slug: newTenantName.toLowerCase().replace(/ /g, '-'),
      phone: newPhone || '+225 07 00 00 00 00',
      city: newCity,
      currency: 'XOF',
      status: 'trial',
      ownerEmail: newEmail,
      monthlyPlan: 'Essai Gratuit (14 jours)',
    };

    setTenants([newT, ...tenants]);
    setNewTenantName('');
    setNewEmail('');
    setNewPhone('');
    setIsModalOpen(false);
  };

  const filteredTenants = tenants.filter(
    (t) =>
      t.name.toLowerCase().includes(searchQuery.toLowerCase()) ||
      t.city.toLowerCase().includes(searchQuery.toLowerCase()) ||
      (t.ownerEmail && t.ownerEmail.toLowerCase().includes(searchQuery.toLowerCase()))
  );

  return (
    <div className="space-y-6">
      {/* Header Strip */}
      <div className="flex flex-col gap-3 rounded-lg border border-neutral-200 bg-white p-5 dark:border-neutral-800 dark:bg-[#111111] sm:flex-row sm:items-center sm:justify-between">
        <div className="flex items-center gap-3">
          <div className="flex h-10 w-10 items-center justify-center rounded-lg bg-neutral-900 text-white dark:bg-neutral-100 dark:text-neutral-900 shadow-xs">
            <Shield className="h-5 w-5" />
          </div>
          <div>
            <div className="flex items-center gap-2">
              <h1 className="text-lg font-semibold tracking-tight text-neutral-900 dark:text-neutral-100">
                Espace Éditeur SaaS — Super Administration Globale
              </h1>
              <span className="font-mono text-[10px] bg-neutral-900 text-white dark:bg-neutral-100 dark:text-neutral-900 px-2 py-0.5 rounded font-medium">
                SuperAdmin
              </span>
            </div>
            <p className="mt-0.5 text-xs text-neutral-500 dark:text-neutral-400">
              Gestion de l'ensemble des pressings souscrits en Côte d'Ivoire, facturation et abonnements.
            </p>
          </div>
        </div>

        <button
          onClick={() => setIsModalOpen(true)}
          className="flex items-center gap-1.5 self-start rounded-md bg-neutral-900 px-3 py-1.5 text-xs font-medium text-white hover:bg-neutral-800 dark:bg-neutral-100 dark:text-neutral-900 dark:hover:bg-white transition-colors sm:self-auto"
        >
          <Plus className="h-3.5 w-3.5" />
          <span>Enregistrer un Pressing</span>
        </button>
      </div>

      {/* Global SaaS KPIs */}
      <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-4">
        <div className="rounded-lg border border-neutral-200 bg-white p-4 dark:border-neutral-800 dark:bg-[#111111]">
          <div className="flex items-center justify-between text-xs text-neutral-500 dark:text-neutral-400">
            <span>Pressings Souscrits</span>
            <Building2 className="h-4 w-4 text-neutral-400 dark:text-neutral-500" />
          </div>
          <div className="mt-2 flex items-baseline gap-1.5">
            <span className="font-mono text-2xl font-semibold tracking-tight text-neutral-900 dark:text-neutral-100 tabular-nums">
              {tenants.length}
            </span>
            <span className="text-xs text-neutral-500 dark:text-neutral-400">établissements</span>
          </div>
          <div className="mt-1 text-[11px] text-neutral-500 dark:text-neutral-400">Côte d'Ivoire (XOF)</div>
        </div>

        <div className="rounded-lg border border-neutral-200 bg-white p-4 dark:border-neutral-800 dark:bg-[#111111]">
          <div className="flex items-center justify-between text-xs text-neutral-500 dark:text-neutral-400">
            <span>Revenu Mensuel (MRR)</span>
            <TrendingUp className="h-4 w-4 text-neutral-400 dark:text-neutral-500" />
          </div>
          <div className="mt-2 flex items-baseline gap-1.5">
            <span className="font-mono text-2xl font-semibold tracking-tight text-neutral-900 dark:text-neutral-100 tabular-nums">
              265 000
            </span>
            <span className="font-mono text-xs text-neutral-500 dark:text-neutral-400 font-medium">FCFA / mois</span>
          </div>
          <div className="mt-1 text-[11px] text-emerald-700 dark:text-emerald-400 font-medium">85% taux de recouvrement</div>
        </div>

        <div className="rounded-lg border border-neutral-200 bg-white p-4 dark:border-neutral-800 dark:bg-[#111111]">
          <div className="flex items-center justify-between text-xs text-neutral-500 dark:text-neutral-400">
            <span>Comptes Actifs</span>
            <CheckCircle className="h-4 w-4 text-neutral-400 dark:text-neutral-500" />
          </div>
          <div className="mt-2 flex items-baseline gap-1.5">
            <span className="font-mono text-2xl font-semibold tracking-tight text-neutral-900 dark:text-neutral-100 tabular-nums">
              {tenants.filter((t) => t.status === 'active').length}
            </span>
            <span className="text-xs text-neutral-500 dark:text-neutral-400">en production</span>
          </div>
          <div className="mt-1 text-[11px] text-neutral-500 dark:text-neutral-400">1 en période d'essai</div>
        </div>

        <div className="rounded-lg border border-neutral-200 bg-white p-4 dark:border-neutral-800 dark:bg-[#111111]">
          <div className="flex items-center justify-between text-xs text-neutral-500 dark:text-neutral-400">
            <span>Séparation Multi-Tenant</span>
            <Shield className="h-4 w-4 text-neutral-400 dark:text-neutral-500" />
          </div>
          <div className="mt-2 flex items-baseline gap-1.5">
            <span className="font-mono text-base font-semibold tracking-tight text-neutral-900 dark:text-neutral-100">
              100% Cloisonné
            </span>
          </div>
          <div className="mt-1 text-[11px] text-neutral-500 dark:text-neutral-400">Vérifié par middleware Go Chi</div>
        </div>
      </div>

      {/* Tenants Table */}
      <div className="rounded-lg border border-neutral-200 bg-white dark:border-neutral-800 dark:bg-[#111111]">
        <div className="flex flex-col gap-3 border-b border-neutral-200 px-5 py-3.5 dark:border-neutral-800 sm:flex-row sm:items-center sm:justify-between">
          <div>
            <h2 className="text-sm font-semibold text-neutral-900 dark:text-neutral-100">
              Répertoire des Pressings Partenaires
            </h2>
            <p className="text-xs text-neutral-500 dark:text-neutral-400">
              Contrôle d'accès et souscriptions des clients de PressingOS.
            </p>
          </div>

          <div className="relative w-full sm:w-64">
            <Search className="absolute left-2.5 top-2.5 h-3.5 w-3.5 text-neutral-400 dark:text-neutral-500" />
            <input
              type="text"
              placeholder="Rechercher pressing, ville..."
              value={searchQuery}
              onChange={(e) => setSearchQuery(e.target.value)}
              className="w-full rounded-md border border-neutral-200 bg-white pl-8 pr-3 py-1.5 text-xs text-neutral-900 placeholder:text-neutral-400 focus:border-neutral-900 focus:outline-hidden dark:border-neutral-800 dark:bg-neutral-900 dark:text-neutral-100 dark:placeholder:text-neutral-500 dark:focus:border-neutral-100"
            />
          </div>
        </div>

        <div className="overflow-x-auto">
          <table className="w-full text-left text-xs">
            <thead>
              <tr className="border-b border-neutral-100 bg-neutral-50/75 text-[11px] text-neutral-500 font-medium dark:border-neutral-800 dark:bg-neutral-900/60 dark:text-neutral-400">
                <th className="px-5 py-2.5">Établissement</th>
                <th className="px-4 py-2.5">Ville & Contact</th>
                <th className="px-4 py-2.5">Compte Propriétaire</th>
                <th className="px-4 py-2.5">Offre Souscrite</th>
                <th className="px-4 py-2.5">Statut</th>
                <th className="px-5 py-2.5 text-right">Actions Éditeur</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-neutral-100 dark:divide-neutral-800">
              {filteredTenants.map((t) => (
                <tr key={t.id} className="hover:bg-neutral-50/50 dark:hover:bg-neutral-800/40 transition-colors">
                  <td className="px-5 py-3.5">
                    <div className="font-semibold text-neutral-900 dark:text-neutral-100">{t.name}</div>
                    <div className="font-mono text-[10px] text-neutral-400 dark:text-neutral-500">{t.id}</div>
                  </td>
                  <td className="px-4 py-3.5 text-neutral-600 dark:text-neutral-400">
                    <div>{t.city}</div>
                    <div className="font-mono text-[11px] text-neutral-400 dark:text-neutral-500">{t.phone}</div>
                  </td>
                  <td className="px-4 py-3.5 font-mono text-neutral-700 dark:text-neutral-300">{t.ownerEmail}</td>
                  <td className="px-4 py-3.5 font-medium text-neutral-800 dark:text-neutral-200">{t.monthlyPlan}</td>
                  <td className="px-4 py-3.5">
                    {t.status === 'active' && (
                      <span className="font-mono text-[11px] text-emerald-700 dark:text-emerald-400 font-medium">Actif</span>
                    )}
                    {t.status === 'trial' && (
                      <span className="font-mono text-[11px] text-amber-700 dark:text-amber-400 font-medium">Essai 14j</span>
                    )}
                    {t.status === 'suspended' && (
                      <span className="font-mono text-[11px] text-rose-700 dark:text-rose-400 font-medium">Suspendu</span>
                    )}
                  </td>
                  <td className="px-5 py-3.5 text-right">
                    <button
                      onClick={() => toggleStatus(t.id)}
                      className={`rounded px-2.5 py-1 text-[11px] font-medium border transition-colors ${t.status === 'active'
                        ? 'border-neutral-200 text-neutral-700 hover:bg-neutral-100 dark:border-neutral-700 dark:text-neutral-300 dark:hover:bg-neutral-800'
                        : 'border-emerald-200 bg-emerald-50 text-emerald-800 hover:bg-emerald-100 dark:border-emerald-800 dark:bg-emerald-950/60 dark:text-emerald-300'
                        }`}
                    >
                      {t.status === 'active' ? 'Suspendre' : 'Réactiver'}
                    </button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </div>

      {/* Add Tenant Modal */}
      {isModalOpen && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-neutral-900/40 dark:bg-black/70 p-4 backdrop-blur-xs">
          <div className="w-full max-w-sm rounded-lg border border-neutral-200 bg-white p-5 shadow-lg dark:border-neutral-800 dark:bg-[#111111]">
            <h2 className="text-sm font-semibold text-neutral-900 dark:text-neutral-100">Inscrire un Nouveau Pressing</h2>
            <p className="mt-0.5 text-xs text-neutral-500 dark:text-neutral-400">
              Création d'un établissement partenaire avec accès d'essai.
            </p>

            <form onSubmit={handleCreateTenant} className="mt-4 space-y-3">
              <div>
                <label className="block text-[11px] font-medium text-neutral-700 dark:text-neutral-300">Nom du Pressing</label>
                <input
                  type="text"
                  required
                  placeholder="Ex: Pressing de la Lagune"
                  value={newTenantName}
                  onChange={(e) => setNewTenantName(e.target.value)}
                  className="mt-1 w-full rounded-md border border-neutral-200 px-3 py-1.5 text-xs text-neutral-900 focus:border-neutral-900 focus:outline-hidden dark:border-neutral-800 dark:bg-neutral-900 dark:text-neutral-100 dark:focus:border-neutral-100"
                />
              </div>

              <div>
                <label className="block text-[11px] font-medium text-neutral-700 dark:text-neutral-300">Ville / Commune</label>
                <input
                  type="text"
                  required
                  placeholder="Ex: Abidjan (Treichville)"
                  value={newCity}
                  onChange={(e) => setNewCity(e.target.value)}
                  className="mt-1 w-full rounded-md border border-neutral-200 px-3 py-1.5 text-xs text-neutral-900 focus:border-neutral-900 focus:outline-hidden dark:border-neutral-800 dark:bg-neutral-900 dark:text-neutral-100 dark:focus:border-neutral-100"
                />
              </div>

              <div>
                <label className="block text-[11px] font-medium text-neutral-700 dark:text-neutral-300">Email Propriétaire</label>
                <input
                  type="email"
                  required
                  placeholder="contact@pressing-lagune.ci"
                  value={newEmail}
                  onChange={(e) => setNewEmail(e.target.value)}
                  className="mt-1 w-full rounded-md border border-neutral-200 px-3 py-1.5 text-xs text-neutral-900 focus:border-neutral-900 focus:outline-hidden dark:border-neutral-800 dark:bg-neutral-900 dark:text-neutral-100 dark:focus:border-neutral-100"
                />
              </div>

              <div>
                <label className="block text-[11px] font-medium text-neutral-700 dark:text-neutral-300">Téléphone (+225)</label>
                <input
                  type="text"
                  placeholder="+225 07 12 34 56 78"
                  value={newPhone}
                  onChange={(e) => setNewPhone(e.target.value)}
                  className="mt-1 w-full rounded-md border border-neutral-200 px-3 py-1.5 text-xs text-neutral-900 focus:border-neutral-900 focus:outline-hidden dark:border-neutral-800 dark:bg-neutral-900 dark:text-neutral-100 dark:focus:border-neutral-100"
                />
              </div>

              <div className="mt-5 flex justify-end gap-2 pt-2 border-t border-neutral-100 dark:border-neutral-800">
                <button
                  type="button"
                  onClick={() => setIsModalOpen(false)}
                  className="rounded-md border border-neutral-200 px-3 py-1.5 text-xs font-medium text-neutral-700 hover:bg-neutral-50 dark:border-neutral-700 dark:text-neutral-300 dark:hover:bg-neutral-800"
                >
                  Annuler
                </button>
                <button
                  type="submit"
                  className="rounded-md bg-neutral-900 px-3 py-1.5 text-xs font-medium text-white hover:bg-neutral-800 dark:bg-neutral-100 dark:text-neutral-900 dark:hover:bg-white"
                >
                  Créer le Pressing
                </button>
              </div>
            </form>
          </div>
        </div>
      )}
    </div>
  );
};
