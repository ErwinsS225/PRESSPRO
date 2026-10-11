import React, { useState, useMemo } from 'react';
import {
  Plus,
  Clock,
  Search,
  Edit2,
  Trash2,
  Database,
  RefreshCw,
  Check,
  AlertTriangle,
  Sparkles,
  TrendingUp,
  Tag,
  Printer,
  Calculator,
  SlidersHorizontal,
  X,
  CheckCircle2,
  Shield,
  Layers,
  ArrowUpDown,
  ShoppingBag,
  Info,
} from 'lucide-react';
import { ServiceItem, User } from '../types';
import {
  createServiceOnBackend,
  updateServiceOnBackend,
  deleteServiceOnBackend,
  bulkAdjustCategoryPrices,
  getLastSyncTime,
} from '../lib/servicesApi';

interface ServicesViewProps {
  services: ServiceItem[];
  currentUser: User;
  token?: string;
  onAddService: (service: ServiceItem) => void;
  onUpdateService: (service: ServiceItem) => void;
  onDeleteService: (serviceId: string) => void;
  onSyncWithDB?: () => Promise<void>;
  showToast?: (msg: string) => void;
}

export const ServicesView: React.FC<ServicesViewProps> = ({
  services,
  currentUser,
  token,
  onAddService,
  onUpdateService,
  onDeleteService,
  onSyncWithDB,
  showToast,
}) => {
  // Navigation & filtres
  const [categoryFilter, setCategoryFilter] = useState<string>('all');
  const [searchQuery, setSearchQuery] = useState('');
  const [statusFilter, setStatusFilter] = useState<'all' | 'active' | 'inactive'>('all');
  const [sortOrder, setSortOrder] = useState<'name' | 'price-asc' | 'price-desc'>('name');

  // Modales & tiroirs
  const [isAddModalOpen, setIsAddModalOpen] = useState(false);
  const [editingItem, setEditingItem] = useState<ServiceItem | null>(null);
  const [deletingItem, setDeletingItem] = useState<ServiceItem | null>(null);
  const [isBulkAdjustOpen, setIsBulkAdjustOpen] = useState(false);
  const [isQuoteCalcOpen, setIsQuoteCalcOpen] = useState(false);
  const [isPrintModalOpen, setIsPrintModalOpen] = useState(false);

  // État de synchronisation DB
  const [isSyncing, setIsSyncing] = useState(false);
  const [lastSyncText, setLastSyncText] = useState(getLastSyncTime());
  const [syncFeedback, setSyncFeedback] = useState<'idle' | 'success' | 'error'>('idle');

  // Formulaire d'ajout / modification
  const [formName, setFormName] = useState('');
  const [formCategory, setFormCategory] = useState('Lavage & Repassage');
  const [formPrice, setFormPrice] = useState(1000);
  const [formUnit, setFormUnit] = useState('pièce');
  const [formDuration, setFormDuration] = useState(24);
  const [formIsActive, setFormIsActive] = useState(true);
  const [formDescription, setFormDescription] = useState('');
  const [isSubmitting, setIsSubmitting] = useState(false);

  // Ajusteur de tarif en masse
  const [bulkCategory, setBulkCategory] = useState('all');
  const [bulkPercent, setBulkPercent] = useState<number>(5);

  // Simulateur de devis
  const [quoteItems, setQuoteItems] = useState<{ [serviceId: string]: number }>({});

  const canEdit =
    currentUser.role === 'owner' ||
    currentUser.role === 'manager' ||
    currentUser.role === 'superadmin';

  const categories = [
    { id: 'all', label: 'Toutes les prestations' },
    { id: 'Lavage & Repassage', label: 'Lavage & Repassage' },
    { id: 'Nettoyage à sec', label: 'Nettoyage à sec' },
    { id: 'Repassage seul', label: 'Repassage seul' },
    { id: 'Délicat', label: 'Délicat / Cérémonie' },
    { id: 'Linge de maison', label: 'Linge de maison' },
  ];

  // Calcul du nombre d'articles par catégorie
  const categoryCounts = useMemo(() => {
    const counts: Record<string, number> = { all: services.length };
    services.forEach((s) => {
      counts[s.category] = (counts[s.category] || 0) + 1;
    });
    return counts;
  }, [services]);

  // Filtrage et tri des prestations
  const filteredServices = useMemo(() => {
    return services
      .filter((s) => {
        const matchesCategory = categoryFilter === 'all' || s.category === categoryFilter;
        const matchesSearch =
          s.name.toLowerCase().includes(searchQuery.toLowerCase()) ||
          s.category.toLowerCase().includes(searchQuery.toLowerCase()) ||
          (s.description && s.description.toLowerCase().includes(searchQuery.toLowerCase()));
        const matchesStatus =
          statusFilter === 'all' ||
          (statusFilter === 'active' && s.isActive !== false) ||
          (statusFilter === 'inactive' && s.isActive === false);
        return matchesCategory && matchesSearch && matchesStatus;
      })
      .sort((a, b) => {
        if (sortOrder === 'price-asc') return a.price - b.price;
        if (sortOrder === 'price-desc') return b.price - a.price;
        return a.name.localeCompare(b.name, 'fr');
      });
  }, [services, categoryFilter, searchQuery, statusFilter, sortOrder]);

  // Ouvrir modal d'ajout
  const handleOpenAdd = () => {
    setFormName('');
    setFormCategory('Lavage & Repassage');
    setFormPrice(1000);
    setFormUnit('pièce');
    setFormDuration(24);
    setFormIsActive(true);
    setFormDescription('');
    setIsAddModalOpen(true);
  };

  // Ouvrir modal d'édition
  const handleOpenEdit = (item: ServiceItem) => {
    setEditingItem(item);
    setFormName(item.name);
    setFormCategory(item.category);
    setFormPrice(item.price);
    setFormUnit(item.unit);
    setFormDuration(item.durationHours);
    setFormIsActive(item.isActive !== false);
    setFormDescription(item.description || '');
  };

  // Enregistrer ajout
  const handleSaveAdd = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!formName.trim() || isSubmitting) return;

    setIsSubmitting(true);
    try {
      const { service, synced } = await createServiceOnBackend(
        {
          name: formName.trim(),
          category: formCategory,
          price: Number(formPrice),
          unit: formUnit,
          durationHours: Number(formDuration),
          isActive: formIsActive,
          description: formDescription.trim(),
        },
        token
      );

      onAddService(service);
      setIsAddModalOpen(false);
      setLastSyncText(getLastSyncTime());

      if (showToast) {
        showToast(
          `Prestation "${service.name}" enregistrée (${service.price.toLocaleString('fr-FR')} FCFA) ${
            synced ? '• Synchro DB validée' : '• Sauvegarde locale active'
          }`
        );
      }
    } catch (err: any) {
      if (showToast) showToast(`Erreur : ${err.message || 'Impossible d\'ajouter la prestation'}`);
    } finally {
      setIsSubmitting(false);
    }
  };

  // Enregistrer modification
  const handleSaveEdit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!editingItem || !formName.trim() || isSubmitting) return;

    setIsSubmitting(true);
    try {
      const updatedItem: ServiceItem = {
        ...editingItem,
        name: formName.trim(),
        category: formCategory,
        price: Number(formPrice),
        unit: formUnit,
        durationHours: Number(formDuration),
        isActive: formIsActive,
        description: formDescription.trim(),
      };

      const { service, synced } = await updateServiceOnBackend(updatedItem, token);
      onUpdateService(service);
      setEditingItem(null);
      setLastSyncText(getLastSyncTime());

      if (showToast) {
        showToast(
          `Prestation "${service.name}" mise à jour (${service.price.toLocaleString('fr-FR')} FCFA) ${
            synced ? '• DB synchronisée' : ''
          }`
        );
      }
    } catch (err: any) {
      if (showToast) showToast(`Erreur : ${err.message || 'Impossible de modifier la prestation'}`);
    } finally {
      setIsSubmitting(false);
    }
  };

  // Basculer l'état actif/inactif
  const handleToggleActive = async (item: ServiceItem) => {
    const updated = { ...item, isActive: item.isActive === false ? true : false };
    onUpdateService(updated);
    await updateServiceOnBackend(updated, token);
    setLastSyncText(getLastSyncTime());
    if (showToast) {
      showToast(
        `Prestation "${item.name}" ${updated.isActive ? 'réactivée' : 'désactivée (archivée temporairement)'}`
      );
    }
  };

  // Confirmer suppression
  const handleConfirmDelete = async () => {
    if (!deletingItem) return;

    const id = deletingItem.id;
    const name = deletingItem.name;
    const { synced } = await deleteServiceOnBackend(id, token);
    onDeleteService(id);
    setDeletingItem(null);
    setLastSyncText(getLastSyncTime());

    if (showToast) {
      showToast(
        `Prestation "${name}" supprimée avec succès ${synced ? 'de la base PostgreSQL' : ''}`
      );
    }
  };

  // Déclencher actualisation DB
  const triggerSync = async () => {
    setIsSyncing(true);
    try {
      if (onSyncWithDB) {
        await onSyncWithDB();
      }
      setSyncFeedback('success');
      setLastSyncText(getLastSyncTime());
      setTimeout(() => setSyncFeedback('idle'), 2500);
    } catch {
      setSyncFeedback('error');
      setTimeout(() => setSyncFeedback('idle'), 3000);
    } finally {
      setIsSyncing(false);
    }
  };

  // Appliquer ajustement en masse
  const handleApplyBulkAdjust = async () => {
    if (bulkPercent === 0) return;
    setIsSyncing(true);
    try {
      const { updatedServices, count } = await bulkAdjustCategoryPrices(
        bulkCategory,
        bulkPercent,
        services,
        token
      );
      // Mettre à jour l'état local pour chaque élément
      updatedServices.forEach((item) => onUpdateService(item));
      setIsBulkAdjustOpen(false);
      setLastSyncText(getLastSyncTime());
      if (showToast) {
        showToast(
          `Ajustement de ${bulkPercent > 0 ? '+' : ''}${bulkPercent}% appliqué à ${count} prestation(s) en FCFA`
        );
      }
    } finally {
      setIsSyncing(false);
    }
  };

  // Gestion du simulateur de devis
  const handleUpdateQuoteQty = (serviceId: string, delta: number) => {
    setQuoteItems((prev) => {
      const current = prev[serviceId] || 0;
      const next = Math.max(0, current + delta);
      if (next === 0) {
        const copy = { ...prev };
        delete copy[serviceId];
        return copy;
      }
      return { ...prev, [serviceId]: next };
    });
  };

  const quoteTotal = useMemo(() => {
    return Object.entries(quoteItems).reduce((sum, [id, qty]) => {
      const service = services.find((s) => s.id === id);
      return sum + (service ? service.price * qty : 0);
    }, 0);
  }, [quoteItems, services]);

  const quoteTotalPieces = useMemo(() => {
    return Object.values(quoteItems).reduce((sum, qty) => sum + qty, 0);
  }, [quoteItems]);

  return (
    <div className="space-y-6">
      {/* BANNIÈRE HAUTE : TITRE, STATUT DB & ACTIONS ADMIN */}
      <div className="flex flex-col gap-4 rounded-xl border border-neutral-200 bg-white p-5 shadow-xs dark:border-neutral-800 dark:bg-[#0c0c0c] sm:flex-row sm:items-center sm:justify-between">
        <div className="space-y-1">
          <div className="flex flex-wrap items-center gap-2">
            <h1 className="text-xl font-semibold tracking-tight text-neutral-900 dark:text-neutral-50">
              Grille Tarifaire & Catalogue des Prestations
            </h1>
            <span className="inline-flex items-center gap-1.5 rounded-full border border-emerald-500/20 bg-emerald-500/10 px-2.5 py-0.5 text-[11px] font-medium text-emerald-700 dark:text-emerald-400">
              <span className="relative flex h-1.5 w-1.5">
                <span className="absolute inline-flex h-full w-full animate-ping rounded-full bg-emerald-400 opacity-75"></span>
                <span className="relative inline-flex h-1.5 w-1.5 rounded-full bg-emerald-500"></span>
              </span>
              <Database className="h-3 w-3" />
              <span>PostgreSQL 16 Connecté</span>
            </span>
          </div>
          <p className="text-xs text-neutral-500 dark:text-neutral-400">
            Tarifs officiels en Francs CFA (XOF) applicables au comptoir pour le lavage, repassage et nettoyage à sec à Abidjan.
          </p>
          <div className="flex items-center gap-2 text-[11px] text-neutral-400 dark:text-neutral-500">
            <span>Dernière synchro DB : {lastSyncText}</span>
            <span>•</span>
            <span>{services.length} prestations répertoriées</span>
          </div>
        </div>

        <div className="flex flex-wrap items-center gap-2 self-start sm:self-auto">
          {/* Bouton Simulateur de devis */}
          <button
            onClick={() => setIsQuoteCalcOpen(!isQuoteCalcOpen)}
            className={`flex items-center gap-1.5 rounded-lg border px-3 py-1.5 text-xs font-medium transition-colors ${
              isQuoteCalcOpen
                ? 'border-neutral-900 bg-neutral-900 text-white dark:border-neutral-100 dark:bg-neutral-100 dark:text-neutral-900'
                : 'border-neutral-200 bg-neutral-50 text-neutral-700 hover:bg-neutral-100 dark:border-neutral-800 dark:bg-neutral-900 dark:text-neutral-300 dark:hover:bg-neutral-800'
            }`}
            title="Calculer instantanément une note ou devis client"
          >
            <Calculator className="h-3.5 w-3.5" />
            <span>Simulateur Devis {quoteTotalPieces > 0 ? `(${quoteTotalPieces})` : ''}</span>
          </button>

          {/* Bouton Imprimer / Exporter Grille */}
          <button
            onClick={() => setIsPrintModalOpen(true)}
            className="flex items-center gap-1.5 rounded-lg border border-neutral-200 bg-neutral-50 px-3 py-1.5 text-xs font-medium text-neutral-700 hover:bg-neutral-100 dark:border-neutral-800 dark:bg-neutral-900 dark:text-neutral-300 dark:hover:bg-neutral-800 transition-colors"
            title="Afficher la fiche tarifaire réglementaire pour impression"
          >
            <Printer className="h-3.5 w-3.5" />
            <span>Fiche Comptoir</span>
          </button>

          {/* Bouton Synchro Manuelle */}
          <button
            onClick={triggerSync}
            disabled={isSyncing}
            className="flex items-center gap-1.5 rounded-lg border border-neutral-200 bg-neutral-50 px-3 py-1.5 text-xs font-medium text-neutral-700 hover:bg-neutral-100 dark:border-neutral-800 dark:bg-neutral-900 dark:text-neutral-300 dark:hover:bg-neutral-800 transition-colors disabled:opacity-50"
            title="Rafraîchir depuis PostgreSQL"
          >
            <RefreshCw className={`h-3.5 w-3.5 ${isSyncing ? 'animate-spin text-emerald-600' : ''}`} />
            <span>
              {isSyncing
                ? 'Synchronisation...'
                : syncFeedback === 'success'
                ? 'Synchronisé !'
                : 'Actualiser DB'}
            </span>
          </button>

          {canEdit && (
            <>
              {/* Ajustement en masse */}
              <button
                onClick={() => setIsBulkAdjustOpen(true)}
                className="flex items-center gap-1.5 rounded-lg border border-neutral-200 bg-neutral-50 px-3 py-1.5 text-xs font-medium text-neutral-700 hover:bg-neutral-100 dark:border-neutral-800 dark:bg-neutral-900 dark:text-neutral-300 dark:hover:bg-neutral-800 transition-colors"
                title="Ajuster les tarifs par pourcentage"
              >
                <SlidersHorizontal className="h-3.5 w-3.5" />
                <span>Ajuster Grille</span>
              </button>

              {/* Ajouter Prestation */}
              <button
                onClick={handleOpenAdd}
                className="flex items-center gap-1.5 rounded-lg bg-neutral-900 px-3.5 py-1.5 text-xs font-medium text-white shadow-xs hover:bg-neutral-800 dark:bg-neutral-100 dark:text-neutral-900 dark:hover:bg-white transition-colors"
              >
                <Plus className="h-3.5 w-3.5" />
                <span>Nouvelle Prestation</span>
              </button>
            </>
          )}
        </div>
      </div>

      {/* PANNEAU CALCULATEUR DE DEVIS INSTANTANÉ (SIMULATEUR DE COMPTOIR) */}
      {isQuoteCalcOpen && (
        <div className="rounded-xl border border-neutral-200 bg-gradient-to-b from-neutral-50 to-white p-5 shadow-xs dark:border-neutral-800 dark:from-neutral-900 dark:to-[#0d0d0d]">
          <div className="flex items-center justify-between border-b border-neutral-200/80 pb-3 dark:border-neutral-800">
            <div className="flex items-center gap-2">
              <ShoppingBag className="h-4 w-4 text-emerald-600 dark:text-emerald-400" />
              <h2 className="text-sm font-semibold text-neutral-900 dark:text-neutral-100">
                Simulateur de Devis & Calcul de Comptoir Express
              </h2>
            </div>
            <div className="flex items-center gap-3">
              <span className="font-mono text-sm font-bold text-neutral-900 dark:text-neutral-100 tabular-nums">
                Total : {quoteTotal.toLocaleString('fr-FR')} FCFA ({quoteTotalPieces} article{quoteTotalPieces > 1 ? 's' : ''})
              </span>
              {quoteTotalPieces > 0 && (
                <button
                  onClick={() => setQuoteItems({})}
                  className="text-xs text-rose-600 hover:underline dark:text-rose-400"
                >
                  Réinitialiser
                </button>
              )}
            </div>
          </div>

          <div className="mt-3 grid grid-cols-1 gap-2 sm:grid-cols-2 md:grid-cols-3 lg:grid-cols-4">
            {services.map((item) => {
              const qty = quoteItems[item.id] || 0;
              return (
                <div
                  key={item.id}
                  className={`flex items-center justify-between rounded-lg border p-2 text-xs transition-colors ${
                    qty > 0
                      ? 'border-emerald-500/50 bg-emerald-50/50 dark:border-emerald-500/30 dark:bg-emerald-950/20'
                      : 'border-neutral-200/60 bg-white dark:border-neutral-800/80 dark:bg-neutral-900/50'
                  }`}
                >
                  <div className="min-w-0 pr-2">
                    <p className="truncate font-medium text-neutral-900 dark:text-neutral-100">{item.name}</p>
                    <p className="font-mono text-[11px] text-neutral-500 dark:text-neutral-400">
                      {item.price.toLocaleString('fr-FR')} FCFA / {item.unit}
                    </p>
                  </div>
                  <div className="flex items-center gap-1.5 shrink-0">
                    <button
                      onClick={() => handleUpdateQuoteQty(item.id, -1)}
                      disabled={qty === 0}
                      className="flex h-6 w-6 items-center justify-center rounded bg-neutral-200 text-neutral-700 hover:bg-neutral-300 disabled:opacity-30 dark:bg-neutral-800 dark:text-neutral-200"
                    >
                      -
                    </button>
                    <span className="w-5 text-center font-mono font-semibold">{qty}</span>
                    <button
                      onClick={() => handleUpdateQuoteQty(item.id, 1)}
                      className="flex h-6 w-6 items-center justify-center rounded bg-neutral-900 text-white hover:bg-neutral-800 dark:bg-neutral-100 dark:text-neutral-900"
                    >
                      +
                    </button>
                  </div>
                </div>
              );
            })}
          </div>
        </div>
      )}

      {/* BARRE DE FILTRES ET RECHERCHE */}
      <div className="flex flex-col gap-3">
        {/* Catégories sous forme d'onglets épurés Vercel/Linear */}
        <div className="flex flex-wrap items-center gap-1.5 p-1 bg-neutral-100 dark:bg-[#141414] rounded-xl border border-neutral-200/80 dark:border-neutral-800/80">
          {categories.map((cat) => {
            const count = categoryCounts[cat.id] || 0;
            const isSelected = categoryFilter === cat.id;
            return (
              <button
                key={cat.id}
                onClick={() => setCategoryFilter(cat.id)}
                className={`flex items-center gap-1.5 px-3 py-1.5 text-xs font-medium rounded-lg transition-all ${
                  isSelected
                    ? 'bg-white text-neutral-900 shadow-2xs font-semibold dark:bg-neutral-800 dark:text-neutral-100'
                    : 'text-neutral-600 hover:text-neutral-900 dark:text-neutral-400 dark:hover:text-neutral-200'
                }`}
              >
                <span>{cat.label}</span>
                <span
                  className={`rounded-full px-1.5 py-0.2 text-[10px] font-mono ${
                    isSelected
                      ? 'bg-neutral-100 text-neutral-900 dark:bg-neutral-700 dark:text-neutral-200'
                      : 'bg-neutral-200/60 text-neutral-500 dark:bg-neutral-800 dark:text-neutral-400'
                  }`}
                >
                  {count}
                </span>
              </button>
            );
          })}
        </div>

        {/* Barre de recherche, statut et tri */}
        <div className="flex flex-col gap-2.5 sm:flex-row sm:items-center sm:justify-between">
          <div className="relative flex-1 sm:max-w-md">
            <Search className="absolute left-3 top-2.5 h-3.5 w-3.5 text-neutral-400 dark:text-neutral-500" />
            <input
              type="text"
              placeholder="Rechercher vêtement, tissu, linge, catégorie..."
              value={searchQuery}
              onChange={(e) => setSearchQuery(e.target.value)}
              className="w-full rounded-lg border border-neutral-200 bg-white pl-9 pr-3 py-1.5 text-xs text-neutral-900 placeholder:text-neutral-400 focus:border-neutral-900 focus:outline-hidden dark:border-neutral-800 dark:bg-neutral-900 dark:text-neutral-100 dark:placeholder:text-neutral-500 dark:focus:border-neutral-100"
            />
          </div>

          <div className="flex flex-wrap items-center gap-2">
            {/* Filtre Statut */}
            <select
              value={statusFilter}
              onChange={(e) => setStatusFilter(e.target.value as any)}
              className="rounded-lg border border-neutral-200 bg-white px-2.5 py-1.5 text-xs text-neutral-700 dark:border-neutral-800 dark:bg-neutral-900 dark:text-neutral-300 focus:outline-hidden"
            >
              <option value="all">Tous les états</option>
              <option value="active">Actives uniquement</option>
              <option value="inactive">Désactivées (Archivées)</option>
            </select>

            {/* Tri des prix */}
            <select
              value={sortOrder}
              onChange={(e) => setSortOrder(e.target.value as any)}
              className="rounded-lg border border-neutral-200 bg-white px-2.5 py-1.5 text-xs text-neutral-700 dark:border-neutral-800 dark:bg-neutral-900 dark:text-neutral-300 focus:outline-hidden"
            >
              <option value="name">Trier par nom (A-Z)</option>
              <option value="price-asc">Tarif croissant</option>
              <option value="price-desc">Tarif décroissant</option>
            </select>
          </div>
        </div>
      </div>

      {/* TABLEAU PRINCIPAL DES PRESTATIONS */}
      <div className="overflow-hidden rounded-xl border border-neutral-200 bg-white shadow-2xs dark:border-neutral-800 dark:bg-[#0f0f0f]">
        <div className="overflow-x-auto">
          <table className="w-full text-left text-xs">
            <thead>
              <tr className="border-b border-neutral-100 bg-neutral-50/75 text-[11px] font-medium text-neutral-500 dark:border-neutral-800 dark:bg-neutral-900/60 dark:text-neutral-400">
                <th className="px-5 py-3">Désignation & Soin d'Atelier</th>
                <th className="px-4 py-3">Catégorie de Soin</th>
                <th className="px-4 py-3">Unité</th>
                <th className="px-4 py-3">Délai Estimé</th>
                <th className="px-4 py-3 text-right">Tarif Unitaire (FCFA)</th>
                <th className="px-4 py-3 text-center">État</th>
                {canEdit && <th className="px-5 py-3 text-right">Actions Admin</th>}
              </tr>
            </thead>
            <tbody className="divide-y divide-neutral-100 dark:divide-neutral-800/80">
              {filteredServices.length === 0 ? (
                <tr>
                  <td colSpan={canEdit ? 7 : 6} className="px-5 py-16 text-center">
                    <div className="mx-auto flex max-w-xs flex-col items-center justify-center text-center">
                      <Tag className="h-8 w-8 text-neutral-300 dark:text-neutral-700 mb-2" />
                      <p className="text-xs font-semibold text-neutral-800 dark:text-neutral-200">
                        Aucune prestation correspondante
                      </p>
                      <p className="mt-1 text-[11px] text-neutral-500 dark:text-neutral-400">
                        Essayez de modifier vos filtres de recherche ou ajoutez une nouvelle prestation à la grille.
                      </p>
                      {canEdit && (
                        <button
                          onClick={handleOpenAdd}
                          className="mt-4 flex items-center gap-1.5 rounded-md bg-neutral-900 px-3 py-1.5 text-xs text-white hover:bg-neutral-800 dark:bg-neutral-100 dark:text-neutral-900 dark:hover:bg-white"
                        >
                          <Plus className="h-3 w-3" />
                          <span>Ajouter une prestation</span>
                        </button>
                      )}
                    </div>
                  </td>
                </tr>
              ) : (
                filteredServices.map((item) => {
                  const isActive = item.isActive !== false;
                  return (
                    <tr
                      key={item.id}
                      className={`transition-colors ${
                        !isActive
                          ? 'bg-neutral-50/40 opacity-60 dark:bg-neutral-900/20'
                          : 'hover:bg-neutral-50/70 dark:hover:bg-neutral-900/50'
                      }`}
                    >
                      {/* Nom & Description */}
                      <td className="px-5 py-3.5">
                        <div className="space-y-0.5">
                          <span className="font-semibold text-neutral-900 dark:text-neutral-100">
                            {item.name}
                          </span>
                          {item.description && (
                            <p className="text-[11px] text-neutral-500 dark:text-neutral-400 max-w-md line-clamp-1">
                              {item.description}
                            </p>
                          )}
                        </div>
                      </td>

                      {/* Catégorie */}
                      <td className="px-4 py-3.5">
                        <span className="inline-flex items-center rounded-md border border-neutral-200 bg-neutral-50 px-2 py-0.5 font-mono text-[11px] text-neutral-700 dark:border-neutral-800 dark:bg-neutral-900 dark:text-neutral-300">
                          {item.category}
                        </span>
                      </td>

                      {/* Unité */}
                      <td className="px-4 py-3.5 font-mono text-[11px] text-neutral-600 dark:text-neutral-400">
                        {item.unit}
                      </td>

                      {/* Délai estimé */}
                      <td className="px-4 py-3.5 font-mono text-[11px] text-neutral-600 dark:text-neutral-400">
                        <span className="inline-flex items-center gap-1">
                          <Clock className="h-3 w-3 text-neutral-400" />
                          {item.durationHours}h ({Math.round(item.durationHours / 24) || 1}j)
                        </span>
                      </td>

                      {/* Tarif en FCFA */}
                      <td className="px-4 py-3.5 text-right font-mono text-xs font-bold text-neutral-900 dark:text-neutral-100 tabular-nums">
                        {item.price.toLocaleString('fr-FR')} FCFA
                      </td>

                      {/* État Actif/Inactif */}
                      <td className="px-4 py-3.5 text-center">
                        <button
                          onClick={() => canEdit && handleToggleActive(item)}
                          disabled={!canEdit}
                          title={isActive ? 'Cliquer pour désactiver' : 'Cliquer pour réactiver'}
                          className={`inline-flex items-center gap-1 rounded-full px-2 py-0.5 text-[10px] font-medium transition-colors ${
                            isActive
                              ? 'bg-emerald-50 text-emerald-700 border border-emerald-200/60 dark:bg-emerald-950/40 dark:text-emerald-400 dark:border-emerald-800/40'
                              : 'bg-neutral-100 text-neutral-500 border border-neutral-200 dark:bg-neutral-800 dark:text-neutral-400 dark:border-neutral-700'
                          }`}
                        >
                          <span className={`h-1.5 w-1.5 rounded-full ${isActive ? 'bg-emerald-500' : 'bg-neutral-400'}`} />
                          <span>{isActive ? 'Actif' : 'Inactif'}</span>
                        </button>
                      </td>

                      {/* Actions Admin */}
                      {canEdit && (
                        <td className="px-5 py-3.5 text-right">
                          <div className="flex items-center justify-end gap-1.5">
                            <button
                              onClick={() => handleOpenEdit(item)}
                              className="rounded-md p-1.5 text-neutral-500 hover:bg-neutral-100 hover:text-neutral-900 dark:text-neutral-400 dark:hover:bg-neutral-800 dark:hover:text-white transition-colors"
                              title="Modifier la prestation et le tarif"
                            >
                              <Edit2 className="h-3.5 w-3.5" />
                            </button>
                            <button
                              onClick={() => setDeletingItem(item)}
                              className="rounded-md p-1.5 text-neutral-400 hover:bg-rose-50 hover:text-rose-600 dark:hover:bg-rose-950/40 dark:hover:text-rose-400 transition-colors"
                              title="Supprimer définitivement"
                            >
                              <Trash2 className="h-3.5 w-3.5" />
                            </button>
                          </div>
                        </td>
                      )}
                    </tr>
                  );
                })
              )}
            </tbody>
          </table>
        </div>
      </div>

      {/* MODAL AJOUT PRESTATION */}
      {isAddModalOpen && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 p-4 backdrop-blur-xs">
          <div className="w-full max-w-md rounded-xl border border-neutral-200 bg-white p-6 shadow-xl dark:border-neutral-800 dark:bg-[#111111]">
            <div className="flex items-center justify-between border-b border-neutral-100 pb-3 dark:border-neutral-800">
              <div className="flex items-center gap-2">
                <div className="flex h-7 w-7 items-center justify-center rounded-lg bg-neutral-900 text-white dark:bg-neutral-100 dark:text-neutral-900">
                  <Plus className="h-4 w-4" />
                </div>
                <div>
                  <h2 className="text-sm font-semibold text-neutral-900 dark:text-neutral-100">
                    Ajouter une Prestation au Catalogue
                  </h2>
                  <p className="text-[11px] text-neutral-500 dark:text-neutral-400">
                    Synchronisé automatiquement avec la base PostgreSQL.
                  </p>
                </div>
              </div>
              <button
                onClick={() => setIsAddModalOpen(false)}
                className="text-neutral-400 hover:text-neutral-700 dark:hover:text-neutral-200"
              >
                <X className="h-4 w-4" />
              </button>
            </div>

            <form onSubmit={handleSaveAdd} className="mt-4 space-y-3.5">
              <div>
                <label className="block text-xs font-medium text-neutral-700 dark:text-neutral-300">
                  Désignation du Vêtement / Linge <span className="text-rose-500">*</span>
                </label>
                <input
                  type="text"
                  required
                  placeholder="Ex: Chemise classique, Pantalon en lin, Couette..."
                  value={formName}
                  onChange={(e) => setFormName(e.target.value)}
                  className="mt-1 w-full rounded-lg border border-neutral-200 px-3 py-1.5 text-xs text-neutral-900 focus:border-neutral-900 focus:outline-hidden dark:border-neutral-800 dark:bg-neutral-900 dark:text-neutral-100 dark:focus:border-neutral-100"
                />
              </div>

              <div className="grid grid-cols-2 gap-3">
                <div>
                  <label className="block text-xs font-medium text-neutral-700 dark:text-neutral-300">
                    Catégorie de Soin
                  </label>
                  <select
                    value={formCategory}
                    onChange={(e) => setFormCategory(e.target.value)}
                    className="mt-1 w-full rounded-lg border border-neutral-200 px-3 py-1.5 text-xs text-neutral-900 focus:border-neutral-900 focus:outline-hidden dark:border-neutral-800 dark:bg-neutral-900 dark:text-neutral-100 dark:focus:border-neutral-100"
                  >
                    <option value="Lavage & Repassage">Lavage & Repassage</option>
                    <option value="Nettoyage à sec">Nettoyage à sec</option>
                    <option value="Repassage seul">Repassage seul</option>
                    <option value="Délicat">Délicat / Cérémonie</option>
                    <option value="Linge de maison">Linge de maison</option>
                  </select>
                </div>

                <div>
                  <label className="block text-xs font-medium text-neutral-700 dark:text-neutral-300">
                    Unité de Facturation
                  </label>
                  <select
                    value={formUnit}
                    onChange={(e) => setFormUnit(e.target.value)}
                    className="mt-1 w-full rounded-lg border border-neutral-200 px-3 py-1.5 text-xs text-neutral-900 focus:border-neutral-900 focus:outline-hidden dark:border-neutral-800 dark:bg-neutral-900 dark:text-neutral-100 dark:focus:border-neutral-100"
                  >
                    <option value="pièce">pièce</option>
                    <option value="ensemble">ensemble</option>
                    <option value="kg">kg (au poids)</option>
                    <option value="mètre">mètre linéaire</option>
                    <option value="paire">la paire</option>
                  </select>
                </div>
              </div>

              <div className="grid grid-cols-2 gap-3">
                <div>
                  <label className="block text-xs font-medium text-neutral-700 dark:text-neutral-300">
                    Tarif Unitaire (FCFA) <span className="text-rose-500">*</span>
                  </label>
                  <input
                    type="number"
                    required
                    min={100}
                    step={50}
                    value={formPrice}
                    onChange={(e) => setFormPrice(Number(e.target.value))}
                    className="mt-1 w-full rounded-lg border border-neutral-200 px-3 py-1.5 text-xs text-neutral-900 focus:border-neutral-900 focus:outline-hidden dark:border-neutral-800 dark:bg-neutral-900 dark:text-neutral-100 dark:focus:border-neutral-100 tabular-nums font-mono font-semibold"
                  />
                </div>

                <div>
                  <label className="block text-xs font-medium text-neutral-700 dark:text-neutral-300">
                    Délai atelier (heures)
                  </label>
                  <input
                    type="number"
                    required
                    min={1}
                    value={formDuration}
                    onChange={(e) => setFormDuration(Number(e.target.value))}
                    className="mt-1 w-full rounded-lg border border-neutral-200 px-3 py-1.5 text-xs text-neutral-900 focus:border-neutral-900 focus:outline-hidden dark:border-neutral-800 dark:bg-neutral-900 dark:text-neutral-100 dark:focus:border-neutral-100 font-mono"
                  />
                </div>
              </div>

              <div>
                <label className="block text-xs font-medium text-neutral-700 dark:text-neutral-300">
                  Instructions d'atelier & Description (Optionnel)
                </label>
                <textarea
                  rows={2}
                  placeholder="Ex: Cintre obligatoire, solvants écologiques, repassage doux..."
                  value={formDescription}
                  onChange={(e) => setFormDescription(e.target.value)}
                  className="mt-1 w-full rounded-lg border border-neutral-200 px-3 py-1.5 text-xs text-neutral-900 focus:border-neutral-900 focus:outline-hidden dark:border-neutral-800 dark:bg-neutral-900 dark:text-neutral-100 dark:focus:border-neutral-100 resize-none"
                />
              </div>

              <div className="flex items-center gap-2 pt-1">
                <input
                  type="checkbox"
                  id="formIsActiveCheck"
                  checked={formIsActive}
                  onChange={(e) => setFormIsActive(e.target.checked)}
                  className="rounded border-neutral-300 text-neutral-900 focus:ring-0 dark:border-neutral-700 dark:bg-neutral-800"
                />
                <label htmlFor="formIsActiveCheck" className="text-xs text-neutral-700 dark:text-neutral-300">
                  Rendre cette prestation immédiatement active et disponible au comptoir
                </label>
              </div>

              <div className="mt-5 flex justify-end gap-2 border-t border-neutral-100 pt-3 dark:border-neutral-800">
                <button
                  type="button"
                  onClick={() => setIsAddModalOpen(false)}
                  className="rounded-lg border border-neutral-200 px-3 py-1.5 text-xs font-medium text-neutral-700 hover:bg-neutral-50 dark:border-neutral-700 dark:text-neutral-300 dark:hover:bg-neutral-800"
                >
                  Annuler
                </button>
                <button
                  type="submit"
                  disabled={isSubmitting}
                  className="rounded-lg bg-neutral-900 px-4 py-1.5 text-xs font-medium text-white hover:bg-neutral-800 dark:bg-neutral-100 dark:text-neutral-900 dark:hover:bg-white disabled:opacity-50"
                >
                  {isSubmitting ? 'Enregistrement...' : 'Enregistrer dans la DB'}
                </button>
              </div>
            </form>
          </div>
        </div>
      )}

      {/* MODAL MODIFICATION PRESTATION */}
      {editingItem && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 p-4 backdrop-blur-xs">
          <div className="w-full max-w-md rounded-xl border border-neutral-200 bg-white p-6 shadow-xl dark:border-neutral-800 dark:bg-[#111111]">
            <div className="flex items-center justify-between border-b border-neutral-100 pb-3 dark:border-neutral-800">
              <div className="flex items-center gap-2">
                <div className="flex h-7 w-7 items-center justify-center rounded-lg bg-neutral-900 text-white dark:bg-neutral-100 dark:text-neutral-900">
                  <Edit2 className="h-4 w-4" />
                </div>
                <div>
                  <h2 className="text-sm font-semibold text-neutral-900 dark:text-neutral-100">
                    Modifier la Prestation & Tarif
                  </h2>
                  <p className="text-[11px] text-neutral-500 dark:text-neutral-400">
                    Mise à jour en direct dans la base de données.
                  </p>
                </div>
              </div>
              <button
                onClick={() => setEditingItem(null)}
                className="text-neutral-400 hover:text-neutral-700 dark:hover:text-neutral-200"
              >
                <X className="h-4 w-4" />
              </button>
            </div>

            <form onSubmit={handleSaveEdit} className="mt-4 space-y-3.5">
              <div>
                <label className="block text-xs font-medium text-neutral-700 dark:text-neutral-300">
                  Désignation du Vêtement / Linge
                </label>
                <input
                  type="text"
                  required
                  value={formName}
                  onChange={(e) => setFormName(e.target.value)}
                  className="mt-1 w-full rounded-lg border border-neutral-200 px-3 py-1.5 text-xs text-neutral-900 focus:border-neutral-900 focus:outline-hidden dark:border-neutral-800 dark:bg-neutral-900 dark:text-neutral-100 dark:focus:border-neutral-100"
                />
              </div>

              <div className="grid grid-cols-2 gap-3">
                <div>
                  <label className="block text-xs font-medium text-neutral-700 dark:text-neutral-300">
                    Catégorie de Soin
                  </label>
                  <select
                    value={formCategory}
                    onChange={(e) => setFormCategory(e.target.value)}
                    className="mt-1 w-full rounded-lg border border-neutral-200 px-3 py-1.5 text-xs text-neutral-900 focus:border-neutral-900 focus:outline-hidden dark:border-neutral-800 dark:bg-neutral-900 dark:text-neutral-100 dark:focus:border-neutral-100"
                  >
                    <option value="Lavage & Repassage">Lavage & Repassage</option>
                    <option value="Nettoyage à sec">Nettoyage à sec</option>
                    <option value="Repassage seul">Repassage seul</option>
                    <option value="Délicat">Délicat / Cérémonie</option>
                    <option value="Linge de maison">Linge de maison</option>
                  </select>
                </div>

                <div>
                  <label className="block text-xs font-medium text-neutral-700 dark:text-neutral-300">
                    Unité de Facturation
                  </label>
                  <select
                    value={formUnit}
                    onChange={(e) => setFormUnit(e.target.value)}
                    className="mt-1 w-full rounded-lg border border-neutral-200 px-3 py-1.5 text-xs text-neutral-900 focus:border-neutral-900 focus:outline-hidden dark:border-neutral-800 dark:bg-neutral-900 dark:text-neutral-100 dark:focus:border-neutral-100"
                  >
                    <option value="pièce">pièce</option>
                    <option value="ensemble">ensemble</option>
                    <option value="kg">kg (au poids)</option>
                    <option value="mètre">mètre linéaire</option>
                    <option value="paire">la paire</option>
                  </select>
                </div>
              </div>

              <div className="grid grid-cols-2 gap-3">
                <div>
                  <label className="block text-xs font-medium text-neutral-700 dark:text-neutral-300">
                    Tarif Unitaire (FCFA)
                  </label>
                  <input
                    type="number"
                    required
                    min={100}
                    step={50}
                    value={formPrice}
                    onChange={(e) => setFormPrice(Number(e.target.value))}
                    className="mt-1 w-full rounded-lg border border-neutral-200 px-3 py-1.5 text-xs text-neutral-900 focus:border-neutral-900 focus:outline-hidden dark:border-neutral-800 dark:bg-neutral-900 dark:text-neutral-100 dark:focus:border-neutral-100 tabular-nums font-mono font-semibold"
                  />
                </div>

                <div>
                  <label className="block text-xs font-medium text-neutral-700 dark:text-neutral-300">
                    Délai atelier (heures)
                  </label>
                  <input
                    type="number"
                    required
                    min={1}
                    value={formDuration}
                    onChange={(e) => setFormDuration(Number(e.target.value))}
                    className="mt-1 w-full rounded-lg border border-neutral-200 px-3 py-1.5 text-xs text-neutral-900 focus:border-neutral-900 focus:outline-hidden dark:border-neutral-800 dark:bg-neutral-900 dark:text-neutral-100 dark:focus:border-neutral-100 font-mono"
                  />
                </div>
              </div>

              <div>
                <label className="block text-xs font-medium text-neutral-700 dark:text-neutral-300">
                  Instructions d'atelier & Description
                </label>
                <textarea
                  rows={2}
                  value={formDescription}
                  onChange={(e) => setFormDescription(e.target.value)}
                  className="mt-1 w-full rounded-lg border border-neutral-200 px-3 py-1.5 text-xs text-neutral-900 focus:border-neutral-900 focus:outline-hidden dark:border-neutral-800 dark:bg-neutral-900 dark:text-neutral-100 dark:focus:border-neutral-100 resize-none"
                />
              </div>

              <div className="flex items-center gap-2 pt-1">
                <input
                  type="checkbox"
                  id="formEditActiveCheck"
                  checked={formIsActive}
                  onChange={(e) => setFormIsActive(e.target.checked)}
                  className="rounded border-neutral-300 text-neutral-900 focus:ring-0 dark:border-neutral-700 dark:bg-neutral-800"
                />
                <label htmlFor="formEditActiveCheck" className="text-xs text-neutral-700 dark:text-neutral-300">
                  Prestation active et facturable au comptoir
                </label>
              </div>

              <div className="mt-5 flex justify-end gap-2 border-t border-neutral-100 pt-3 dark:border-neutral-800">
                <button
                  type="button"
                  onClick={() => setEditingItem(null)}
                  className="rounded-lg border border-neutral-200 px-3 py-1.5 text-xs font-medium text-neutral-700 hover:bg-neutral-50 dark:border-neutral-700 dark:text-neutral-300 dark:hover:bg-neutral-800"
                >
                  Annuler
                </button>
                <button
                  type="submit"
                  disabled={isSubmitting}
                  className="rounded-lg bg-neutral-900 px-4 py-1.5 text-xs font-medium text-white hover:bg-neutral-800 dark:bg-neutral-100 dark:text-neutral-900 dark:hover:bg-white disabled:opacity-50"
                >
                  {isSubmitting ? 'Mise à jour...' : 'Mettre à jour'}
                </button>
              </div>
            </form>
          </div>
        </div>
      )}

      {/* MODAL CONFIRMATION SUPPRESSION */}
      {deletingItem && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 p-4 backdrop-blur-xs">
          <div className="w-full max-w-sm rounded-xl border border-neutral-200 bg-white p-5 shadow-xl dark:border-neutral-800 dark:bg-[#111111]">
            <div className="flex items-center gap-2 text-rose-600 dark:text-rose-400">
              <AlertTriangle className="h-4 w-4" />
              <h2 className="text-sm font-semibold">Confirmer la Suppression</h2>
            </div>
            <p className="mt-2 text-xs text-neutral-600 dark:text-neutral-300 leading-relaxed">
              Êtes-vous certain de vouloir supprimer définitivement <strong>{deletingItem.name}</strong> ({deletingItem.price.toLocaleString('fr-FR')} FCFA) de la base de données ?
            </p>
            <p className="mt-1 text-[11px] text-neutral-400 dark:text-neutral-500">
              Astuce : Si le service est temporairement indisponible, préférez le basculer en statut « Inactif ».
            </p>

            <div className="mt-5 flex justify-end gap-2 border-t border-neutral-100 pt-3 dark:border-neutral-800">
              <button
                type="button"
                onClick={() => setDeletingItem(null)}
                className="rounded-lg border border-neutral-200 px-3 py-1.5 text-xs font-medium text-neutral-700 hover:bg-neutral-50 dark:border-neutral-700 dark:text-neutral-300 dark:hover:bg-neutral-800"
              >
                Annuler
              </button>
              <button
                type="button"
                onClick={handleConfirmDelete}
                className="rounded-lg bg-rose-600 px-3.5 py-1.5 text-xs font-medium text-white hover:bg-rose-700 transition-colors"
              >
                Confirmer la suppression
              </button>
            </div>
          </div>
        </div>
      )}

      {/* MODAL AJUSTEUR DE GRILLE TARIFAIRE EN MASSE */}
      {isBulkAdjustOpen && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 p-4 backdrop-blur-xs">
          <div className="w-full max-w-md rounded-xl border border-neutral-200 bg-white p-6 shadow-xl dark:border-neutral-800 dark:bg-[#111111]">
            <div className="flex items-center justify-between border-b border-neutral-100 pb-3 dark:border-neutral-800">
              <div className="flex items-center gap-2">
                <div className="flex h-7 w-7 items-center justify-center rounded-lg bg-neutral-900 text-white dark:bg-neutral-100 dark:text-neutral-900">
                  <SlidersHorizontal className="h-4 w-4" />
                </div>
                <div>
                  <h2 className="text-sm font-semibold text-neutral-900 dark:text-neutral-100">
                    Ajustement de Grille Tarifaire en Masse
                  </h2>
                  <p className="text-[11px] text-neutral-500 dark:text-neutral-400">
                    Modifier les tarifs par catégorie avec arrondi automatique (ex: +5%, +10%).
                  </p>
                </div>
              </div>
              <button
                onClick={() => setIsBulkAdjustOpen(false)}
                className="text-neutral-400 hover:text-neutral-700 dark:hover:text-neutral-200"
              >
                <X className="h-4 w-4" />
              </button>
            </div>

            <div className="mt-4 space-y-4">
              <div>
                <label className="block text-xs font-medium text-neutral-700 dark:text-neutral-300">
                  Catégorie Cible
                </label>
                <select
                  value={bulkCategory}
                  onChange={(e) => setBulkCategory(e.target.value)}
                  className="mt-1 w-full rounded-lg border border-neutral-200 px-3 py-1.5 text-xs text-neutral-900 focus:border-neutral-900 focus:outline-hidden dark:border-neutral-800 dark:bg-neutral-900 dark:text-neutral-100"
                >
                  <option value="all">Toutes les catégories confondues ({services.length} articles)</option>
                  <option value="Lavage & Repassage">Lavage & Repassage</option>
                  <option value="Nettoyage à sec">Nettoyage à sec</option>
                  <option value="Repassage seul">Repassage seul</option>
                  <option value="Délicat">Délicat / Cérémonie</option>
                  <option value="Linge de maison">Linge de maison</option>
                </select>
              </div>

              <div>
                <label className="block text-xs font-medium text-neutral-700 dark:text-neutral-300">
                  Pourcentage d'Ajustement (%)
                </label>
                <div className="mt-1 flex items-center gap-2">
                  {[-10, -5, 5, 10, 15].map((pct) => (
                    <button
                      key={pct}
                      type="button"
                      onClick={() => setBulkPercent(pct)}
                      className={`flex-1 rounded-md border py-1.5 text-xs font-mono font-medium transition-colors ${
                        bulkPercent === pct
                          ? 'border-neutral-900 bg-neutral-900 text-white dark:border-neutral-100 dark:bg-neutral-100 dark:text-neutral-900'
                          : 'border-neutral-200 hover:bg-neutral-50 dark:border-neutral-700 dark:text-neutral-300 dark:hover:bg-neutral-800'
                      }`}
                    >
                      {pct > 0 ? `+${pct}%` : `${pct}%`}
                    </button>
                  ))}
                </div>
                <div className="mt-2 flex items-center gap-2">
                  <span className="text-xs text-neutral-500">Personnalisé :</span>
                  <input
                    type="number"
                    value={bulkPercent}
                    onChange={(e) => setBulkPercent(Number(e.target.value))}
                    className="w-24 rounded-md border border-neutral-200 px-2 py-1 text-xs font-mono dark:border-neutral-700 dark:bg-neutral-900 dark:text-neutral-100"
                  />
                  <span className="text-xs font-mono">%</span>
                </div>
              </div>

              {/* Prévisualisation de l'ajustement */}
              <div className="rounded-lg border border-neutral-200/60 bg-neutral-50 p-3 text-xs dark:border-neutral-800 dark:bg-neutral-900/60">
                <p className="font-medium text-neutral-800 dark:text-neutral-200">Exemple de simulation :</p>
                <div className="mt-1.5 space-y-1 font-mono text-[11px] text-neutral-600 dark:text-neutral-400">
                  <p>Chemise (1 000 FCFA) ➔ {Math.round((1000 * (1 + bulkPercent / 100)) / 50) * 50} FCFA</p>
                  <p>Costume 2 pièces (3 500 FCFA) ➔ {Math.round((3500 * (1 + bulkPercent / 100)) / 50) * 50} FCFA</p>
                </div>
              </div>

              <div className="flex justify-end gap-2 border-t border-neutral-100 pt-3 dark:border-neutral-800">
                <button
                  type="button"
                  onClick={() => setIsBulkAdjustOpen(false)}
                  className="rounded-lg border border-neutral-200 px-3 py-1.5 text-xs font-medium text-neutral-700 hover:bg-neutral-50 dark:border-neutral-700 dark:text-neutral-300 dark:hover:bg-neutral-800"
                >
                  Annuler
                </button>
                <button
                  type="button"
                  onClick={handleApplyBulkAdjust}
                  disabled={isSyncing}
                  className="rounded-lg bg-neutral-900 px-4 py-1.5 text-xs font-medium text-white hover:bg-neutral-800 dark:bg-neutral-100 dark:text-neutral-900 dark:hover:bg-white disabled:opacity-50"
                >
                  {isSyncing ? 'Mise à jour...' : 'Appliquer & Synchroniser DB'}
                </button>
              </div>
            </div>
          </div>
        </div>
      )}

      {/* MODAL FICHE TARIFAIRE OFFICIELLE COMPTOIR (POUR IMPRESSION) */}
      {isPrintModalOpen && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 p-4 backdrop-blur-xs">
          <div className="w-full max-w-2xl max-h-[90vh] overflow-y-auto rounded-xl border border-neutral-200 bg-white p-6 shadow-xl dark:border-neutral-800 dark:bg-[#111111]">
            <div className="flex items-center justify-between border-b border-neutral-100 pb-3 dark:border-neutral-800">
              <div className="flex items-center gap-2">
                <Printer className="h-4 w-4 text-neutral-700 dark:text-neutral-300" />
                <h2 className="text-sm font-semibold text-neutral-900 dark:text-neutral-100">
                  Fiche Tarifaire Officielle de Comptoir (Affichage Réglementaire)
                </h2>
              </div>
              <div className="flex items-center gap-2">
                <button
                  onClick={() => window.print()}
                  className="flex items-center gap-1 rounded-md bg-neutral-900 px-2.5 py-1 text-xs text-white hover:bg-neutral-800 dark:bg-neutral-100 dark:text-neutral-900"
                >
                  <Printer className="h-3 w-3" />
                  <span>Imprimer</span>
                </button>
                <button
                  onClick={() => setIsPrintModalOpen(false)}
                  className="text-neutral-400 hover:text-neutral-700 dark:hover:text-neutral-200"
                >
                  <X className="h-4 w-4" />
                </button>
              </div>
            </div>

            {/* Fiche imprimable */}
            <div className="mt-4 rounded-lg border border-neutral-200 p-6 bg-white text-neutral-900">
              <div className="border-b-2 border-neutral-900 pb-3 text-center">
                <h3 className="text-lg font-bold uppercase tracking-wider">PRESSING IVOIRE COCODY</h3>
                <p className="text-xs text-neutral-600">Abidjan Cocody Angré • Tél: +225 07 00 11 22 33</p>
                <p className="mt-1 font-mono text-[11px] font-semibold uppercase tracking-widest text-neutral-800">
                  GRILLE TARIFAIRE OFFICIELLE EN FRANCS CFA (XOF)
                </p>
              </div>

              <div className="mt-4 space-y-4">
                {['Lavage & Repassage', 'Nettoyage à sec', 'Repassage seul', 'Délicat', 'Linge de maison'].map((cat) => {
                  const itemsInCat = services.filter((s) => s.category === cat && s.isActive !== false);
                  if (itemsInCat.length === 0) return null;
                  return (
                    <div key={cat} className="space-y-1">
                      <h4 className="border-b border-neutral-200 pb-0.5 text-xs font-bold uppercase tracking-wide text-neutral-900">
                        {cat}
                      </h4>
                      <table className="w-full text-xs">
                        <tbody>
                          {itemsInCat.map((item) => (
                            <tr key={item.id} className="border-b border-neutral-100/60">
                              <td className="py-1">{item.name}</td>
                              <td className="py-1 text-neutral-500 font-mono text-[11px]">{item.unit}</td>
                              <td className="py-1 text-right font-mono font-bold tabular-nums">
                                {item.price.toLocaleString('fr-FR')} FCFA
                              </td>
                            </tr>
                          ))}
                        </tbody>
                      </table>
                    </div>
                  );
                })}
              </div>

              <div className="mt-6 border-t border-neutral-200 pt-2 text-center text-[10px] text-neutral-500">
                Tous les tarifs sont indiqués toutes taxes comprises (TTC). Les délais mentionnés sont indicatifs et dépendent de la charge d'atelier.
              </div>
            </div>
          </div>
        </div>
      )}
    </div>
  );
};
