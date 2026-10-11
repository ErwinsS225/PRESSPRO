import React, { useState, useMemo } from 'react';
import {
  Plus,
  Clock,
  Search,
  Edit2,
  Trash2,
  Database,
  RefreshCw,
  AlertTriangle,
  AlertCircle,
  Loader2,
  Tag,
  Receipt,
  Calculator,
  SlidersHorizontal,
  X,
  CheckCircle2,
  Layers,
  ShoppingBag,
  Info,
  Truck,
  Zap,
  Briefcase,
  Calendar,
  Copy,
} from 'lucide-react';
import { ServiceItem, User, SubscriptionPlan, B2BContract, PricingModel } from '../types';
import {
  createServiceOnBackend,
  updateServiceOnBackend,
  deleteServiceOnBackend,
  bulkAdjustCategoryPrices,
  getLastSyncTime,
} from '../lib/servicesApi';
import { DEMO_SUBSCRIPTION_PLANS, DEMO_B2B_CONTRACTS } from '../lib/demoData';
import { CounterReceiptModal } from './CounterReceiptModal';
import { CatalogManagerModal } from './CatalogManagerModal';

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

// Modèle de facturation pour le filtre du catalogue
type ServiceFilter = 'all' | 'piece' | 'kilogram' | 'quote';

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
  // Onglet principal : Catalogue / Abonnements / Contrats B2B
  const [mainTab, setMainTab] = useState<'catalog' | 'subscriptions' | 'contracts'>('catalog');
  // Sous-onglets du catalogue : mode de facturation

  // Navigation & filtres de catalogue
  const [categoryFilter, setCategoryFilter] = useState<string>('all');
  const [searchQuery, setSearchQuery] = useState('');
  const [statusFilter, setStatusFilter] = useState<'all' | 'active' | 'inactive'>('all');
  const [modelFilter, setModelFilter] = useState<ServiceFilter>('all');
  const [sortOrder, setSortOrder] = useState<'name' | 'price-asc' | 'price-desc'>('name');

  // Modales & tiroirs
  const [isCatalogManagerOpen, setIsCatalogManagerOpen] = useState(false);
  const [isAddModalOpen, setIsAddModalOpen] = useState(false);
  const [editingItem, setEditingItem] = useState<ServiceItem | null>(null);
  const [deletingItem, setDeletingItem] = useState<ServiceItem | null>(null);
  const [isDeleting, setIsDeleting] = useState(false);
  const [deleteError, setDeleteError] = useState<string | null>(null);
  const [isBulkAdjustOpen, setIsBulkAdjustOpen] = useState(false);
  const [isQuoteCalcOpen, setIsQuoteCalcOpen] = useState(false);
  const [isPrintModalOpen, setIsPrintModalOpen] = useState(false);

  // Données Abonnements et Contrats B2B (Modèles commerciaux 3 & 4)
  const [subscriptions] = useState<SubscriptionPlan[]>(() => {
    try {
      const saved = localStorage.getItem('pressingos_subscriptions');
      return saved ? JSON.parse(saved) : DEMO_SUBSCRIPTION_PLANS;
    } catch {
      return DEMO_SUBSCRIPTION_PLANS;
    }
  });

  const [b2bContracts] = useState<B2BContract[]>(() => {
    try {
      const saved = localStorage.getItem('pressingos_b2b_contracts');
      return saved ? JSON.parse(saved) : DEMO_B2B_CONTRACTS;
    } catch {
      return DEMO_B2B_CONTRACTS;
    }
  });

  // État de synchronisation DB
  const [isSyncing, setIsSyncing] = useState(false);
  const [lastSyncText, setLastSyncText] = useState(getLastSyncTime());
  const [syncFeedback, setSyncFeedback] = useState<'idle' | 'success' | 'error'>('idle');

  // Formulaire d'ajout / modification de prestation
  const [formArticle, setFormArticle] = useState('');
  const [formTreatment, setFormTreatment] = useState('Lavage + repassage');
  const [formCategory, setFormCategory] = useState('Vêtements du quotidien');
  const [formPrice, setFormPrice] = useState(1000);
  const [formUnit, setFormUnit] = useState('pièce');
  const [formPricingModel, setFormPricingModel] = useState<PricingModel>('piece');
  const [formDuration, setFormDuration] = useState(24);
  const [formIsActive, setFormIsActive] = useState(true);
  const [formIsExpress, setFormIsExpress] = useState(false);
  const [formExpressSurcharge, setFormExpressSurcharge] = useState(500);
  const [formIsPickup, setFormIsPickup] = useState(true);
  const [formIsOutsourced, setFormIsOutsourced] = useState(false);
  const [formRequiresQuote, setFormRequiresQuote] = useState(false);
  const [formDescription, setFormDescription] = useState('');
  const [isSubmitting, setIsSubmitting] = useState(false);

  // Ajusteur de tarif en masse
  const [bulkCategory, setBulkCategory] = useState('all');
  const [bulkPercent, setBulkPercent] = useState<number>(5);

  // Simulateur de devis
  const [quoteItems, setQuoteItems] = useState<{ [serviceId: string]: number }>({});
  const [includePickupInQuote, setIncludePickupInQuote] = useState(false);
  const [includeExpressInQuote, setIncludeExpressInQuote] = useState(false);

  const canEdit =
    currentUser.role === 'owner' || currentUser.role === 'manager' || currentUser.role === 'superadmin';

  // Les 5 grandes catégories officielles d'Abidjan
  const categories = [
    { id: 'all', label: 'Toutes les catégories', short: 'Toutes' },
    { id: 'Vêtements du quotidien', label: 'A. Vêtements du quotidien (Priorité 1)', short: 'Quotidien' },
    {
      id: 'Vêtements traditionnels & cérémoniels',
      label: 'B. Traditionnels & Cérémoniels (Priorité 1)',
      short: 'Traditionnel',
    },
    { id: 'Linge de maison', label: 'C. Linge de maison', short: 'Linge de maison' },
    {
      id: 'Services complémentaires & Options',
      label: 'D. Services complémentaires & Options',
      short: 'Compléments',
    },
    { id: 'Prestations spécialisées & Luxe', label: 'E. Spécialisées & Luxe', short: 'Luxe & Spécial' },
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
          (s.article && s.article.toLowerCase().includes(searchQuery.toLowerCase())) ||
          (s.treatment && s.treatment.toLowerCase().includes(searchQuery.toLowerCase())) ||
          s.category.toLowerCase().includes(searchQuery.toLowerCase()) ||
          (s.description && s.description.toLowerCase().includes(searchQuery.toLowerCase()));

        const matchesStatus =
          statusFilter === 'all' ||
          (statusFilter === 'active' && s.isActive !== false) ||
          (statusFilter === 'inactive' && s.isActive === false);

        const matchesModel =
          modelFilter === 'all' ||
          (modelFilter === 'piece' && (s.pricingModel === 'piece' || !s.pricingModel)) ||
          (modelFilter === 'kilogram' && s.pricingModel === 'kilogram') ||
          (modelFilter === 'quote' && s.requiresQuote === true);

        return matchesCategory && matchesSearch && matchesStatus && matchesModel;
      })
      .sort((a, b) => {
        if (sortOrder === 'price-asc') return a.price - b.price;
        if (sortOrder === 'price-desc') return b.price - a.price;
        return a.name.localeCompare(b.name, 'fr');
      });
  }, [services, categoryFilter, searchQuery, statusFilter, modelFilter, sortOrder]);

  // Ouvrir modal d'ajout
  const handleOpenAdd = () => {
    setFormArticle('');
    setFormTreatment('Lavage + repassage');
    setFormCategory('Vêtements du quotidien');
    setFormPrice(1000);
    setFormUnit('pièce');
    setFormPricingModel('piece');
    setFormDuration(24);
    setFormIsActive(true);
    setFormIsExpress(false);
    setFormExpressSurcharge(500);
    setFormIsPickup(true);
    setFormIsOutsourced(false);
    setFormRequiresQuote(false);
    setFormDescription('');
    setIsAddModalOpen(true);
  };

  // Ouvrir modal d'édition
  const handleOpenEdit = (item: ServiceItem) => {
    setEditingItem(item);
    setFormArticle(item.article || item.name);
    setFormTreatment(item.treatment || 'Lavage & Soin');
    setFormCategory(item.category);
    setFormPrice(item.price);
    setFormUnit(item.unit);
    setFormPricingModel(item.pricingModel || 'piece');
    setFormDuration(item.durationHours);
    setFormIsActive(item.isActive !== false);
    setFormIsExpress(item.isExpressAvailable === true);
    setFormExpressSurcharge(item.expressSurcharge || 500);
    setFormIsPickup(item.isPickupAvailable !== false);
    setFormIsOutsourced(item.isOutsourced === true);
    setFormRequiresQuote(item.requiresQuote === true);
    setFormDescription(item.description || '');
  };

  // Dupliquer un article pour créer une déclinaison (ex: Repassage seul)
  const handleDuplicate = (item: ServiceItem) => {
    setFormArticle(item.article || item.name);
    setFormTreatment('Repassage seul');
    setFormCategory(item.category);
    setFormPrice(Math.round((item.price * 0.6) / 50) * 50); // ~60% du prix
    setFormUnit(item.unit);
    setFormPricingModel(item.pricingModel || 'piece');
    setFormDuration(12);
    setFormIsActive(true);
    setFormIsExpress(item.isExpressAvailable === true);
    setFormExpressSurcharge(item.expressSurcharge || 300);
    setFormIsPickup(item.isPickupAvailable !== false);
    setFormIsOutsourced(false);
    setFormRequiresQuote(false);
    setFormDescription(`Déclinaison spécifique : Repassage pour ${item.article || item.name}.`);
    setIsAddModalOpen(true);
  };

  // Enregistrer ajout
  const handleSaveAdd = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!formArticle.trim() || isSubmitting) return;

    setIsSubmitting(true);
    const fullName = `${formArticle.trim()} (${formTreatment.trim()})`;

    try {
      const { service, synced } = await createServiceOnBackend(
        {
          name: fullName,
          article: formArticle.trim(),
          treatment: formTreatment.trim(),
          category: formCategory,
          price: Number(formPrice),
          unit: formUnit,
          pricingModel: formPricingModel,
          durationHours: Number(formDuration),
          isActive: formIsActive,
          isExpressAvailable: formIsExpress,
          expressSurcharge: formIsExpress ? Number(formExpressSurcharge) : 0,
          isPickupAvailable: formIsPickup,
          isOutsourced: formIsOutsourced,
          requiresQuote: formRequiresQuote,
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
            synced ? '• Base PostgreSQL mise à jour' : '• Sauvegarde locale active'
          }`
        );
      }
    } catch (err: any) {
      if (showToast) showToast(`Erreur : ${err.message || "Impossible d'ajouter la prestation"}`);
    } finally {
      setIsSubmitting(false);
    }
  };

  // Enregistrer modification
  const handleSaveEdit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!editingItem || !formArticle.trim() || isSubmitting) return;

    setIsSubmitting(true);
    const fullName = `${formArticle.trim()} (${formTreatment.trim()})`;

    try {
      const updatedItem: ServiceItem = {
        ...editingItem,
        name: fullName,
        article: formArticle.trim(),
        treatment: formTreatment.trim(),
        category: formCategory,
        price: Number(formPrice),
        unit: formUnit,
        pricingModel: formPricingModel,
        durationHours: Number(formDuration),
        isActive: formIsActive,
        isExpressAvailable: formIsExpress,
        expressSurcharge: formIsExpress ? Number(formExpressSurcharge) : 0,
        isPickupAvailable: formIsPickup,
        isOutsourced: formIsOutsourced,
        requiresQuote: formRequiresQuote,
        description: formDescription.trim(),
      };

      const { service, synced } = await updateServiceOnBackend(updatedItem, token);
      onUpdateService(service);
      setEditingItem(null);
      setLastSyncText(getLastSyncTime());

      if (showToast) {
        showToast(
          `Prestation "${service.name}" mise à jour (${service.price.toLocaleString('fr-FR')} FCFA) ${
            synced ? '• Base PostgreSQL actualisée' : ''
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
    const updated: ServiceItem = { ...item, isActive: item.isActive === false ? true : false };
    onUpdateService(updated);
    await updateServiceOnBackend(updated, token);
    setLastSyncText(getLastSyncTime());
    if (showToast) {
      showToast(
        `Prestation "${item.name}" ${updated.isActive ? 'réactivée au comptoir' : 'désactivée (archivée sans impact historique)'}`
      );
    }
  };

  // Ouvrir le dialogue de suppression sécurisée
  const handleRequestDelete = (item: ServiceItem) => {
    setDeletingItem(item);
    setDeleteError(null);
    setIsDeleting(false);
  };

  // Alternative sécurisée : désactiver la prestation au lieu de la supprimer définitivement
  const handleDeactivateInstead = async () => {
    if (!deletingItem || isDeleting) return;
    const targetItem = deletingItem;
    setIsDeleting(true);
    try {
      await handleToggleActive(targetItem);
      setDeletingItem(null);
      setDeleteError(null);
    } finally {
      setIsDeleting(false);
    }
  };

  // Confirmer suppression sécurisée avec envoi de requête DELETE à l'API Go
  const handleConfirmDelete = async () => {
    if (!deletingItem || isDeleting) return;

    setIsDeleting(true);
    setDeleteError(null);
    const itemToDelete = deletingItem;

    try {
      // Envoi de la requête DELETE à l'API Go (/api/v1/services/{id})
      const result = await deleteServiceOnBackend(itemToDelete.id, token);

      if (!result.success && result.error) {
        setDeleteError(result.error);
        setIsDeleting(false);
        return;
      }

      // Suppression confirmée et exécutée
      onDeleteService(itemToDelete.id);
      setDeletingItem(null);
      setLastSyncText(getLastSyncTime());

      if (showToast) {
        showToast(
          `Prestation "${itemToDelete.name}" supprimée définitivement ${
            result.synced ? 'de la base de données PostgreSQL' : ''
          }`
        );
      }
    } catch (err: any) {
      setDeleteError(err.message || 'Erreur inattendue lors de la suppression de la prestation.');
    } finally {
      setIsDeleting(false);
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
      updatedServices.forEach((item) => onUpdateService(item));
      setIsBulkAdjustOpen(false);
      setLastSyncText(getLastSyncTime());
      if (showToast) {
        showToast(
          `Ajustement tarifaire de ${bulkPercent > 0 ? '+' : ''}${bulkPercent}% appliqué à ${count} prestation(s) en FCFA`
        );
      }
    } finally {
      setIsSyncing(false);
    }
  };

  // Simulateur de devis
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
    let base = Object.entries(quoteItems).reduce((sum, [id, qty]) => {
      const service = services.find((s) => s.id === id);
      return sum + (service ? service.price * qty : 0);
    }, 0);

    if (includeExpressInQuote) {
      base += Math.round(base * 0.5); // +50% express
    }
    if (includePickupInQuote && base > 0) {
      base += 1500; // forfait collecte Abidjan
    }
    return base;
  }, [quoteItems, services, includeExpressInQuote, includePickupInQuote]);

  const quoteTotalPieces = useMemo(() => {
    return Object.values(quoteItems).reduce((sum, qty) => sum + qty, 0);
  }, [quoteItems]);

  return (
    <div className="space-y-6">
      {/* BANNIÈRE SUPÉRIEURE : EN-TÊTE ERP & SYNC STATUS */}
      <div className="flex flex-col gap-4 rounded-xl border border-neutral-200 bg-white p-5 dark:border-neutral-800 dark:bg-[#0c0c0c] sm:flex-row sm:items-center sm:justify-between hover:border-neutral-950 dark:hover:border-white/70 ps-elevated">
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
              <span>PostgreSQL 16 Synchro</span>
            </span>
          </div>
          <p className="text-xs text-neutral-500 dark:text-neutral-400">
            Catalogue multi-modèles (Pièce, Kilogramme, Abonnements & B2B) paramétrable par pressing en Francs
            CFA (XOF).
          </p>
          <div className="flex items-center gap-2 text-[11px] text-neutral-400 dark:text-neutral-500">
            <span>Dernière synchro DB : {lastSyncText}</span>
            <span>•</span>
            <span>{services.length} prestations répertoriées</span>
            <span>•</span>
            <span className="text-emerald-600 dark:text-emerald-400">Tarifs de référence Abidjan (XOF)</span>
          </div>
        </div>

        <div className="flex flex-wrap items-center gap-2 self-start sm:self-auto">
          {/* Simulateur express */}
          <button
            onClick={() => setIsQuoteCalcOpen(!isQuoteCalcOpen)}
            className={`flex items-center gap-1.5 rounded-lg border px-3 py-1.5 text-xs font-medium transition-colors ${
              isQuoteCalcOpen
                ? 'border-neutral-900 bg-neutral-900 text-white dark:border-neutral-100 dark:bg-neutral-100 dark:text-neutral-900'
                : 'border-neutral-200 bg-neutral-50 text-neutral-700 hover:bg-neutral-950 hover:text-white dark:border-neutral-800 dark:bg-neutral-900 dark:text-neutral-300 dark:hover:bg-neutral-100 dark:hover:text-neutral-950'
            }`}
          >
            <Calculator className="h-3.5 w-3.5" />
            <span>Simulateur Devis {quoteTotalPieces > 0 ? `(${quoteTotalPieces})` : ''}</span>
          </button>

          {/* Affichage Reçu Comptoir (Format Ticket 80mm/58mm) */}
          <button
            onClick={() => setIsPrintModalOpen(true)}
            className="flex items-center gap-1.5 rounded-lg border border-neutral-200 bg-neutral-50 px-3 py-1.5 text-xs font-medium text-neutral-700 hover:bg-neutral-950 hover:text-white dark:border-neutral-800 dark:bg-neutral-900 dark:text-neutral-300 transition-colors hover:border-neutral-950 dark:hover:border-white/70 ps-elevated dark:hover:bg-neutral-100 dark:hover:text-neutral-950"
            title="Imprimer le barème au format reçu thermique (80mm / 58mm comme en pharmacie, boulangerie ou supermarché)"
          >
            <Receipt className="h-3.5 w-3.5 text-emerald-600 dark:text-emerald-400" />
            <span>Reçu Tarifaire Comptoir</span>
            <span className="rounded bg-neutral-200/80 px-1 py-0.2 text-[9px] font-mono dark:bg-neutral-800 text-neutral-600 dark:text-neutral-400">
              80mm
            </span>
          </button>

          {/* Actualiser DB */}
          <button
            onClick={triggerSync}
            disabled={isSyncing}
            className="flex items-center gap-1.5 rounded-lg border border-neutral-200 bg-neutral-50 px-3 py-1.5 text-xs font-medium text-neutral-700 hover:bg-neutral-950 hover:text-white dark:border-neutral-800 dark:bg-neutral-900 dark:text-neutral-300 transition-colors disabled:opacity-50 hover:border-neutral-950 dark:hover:border-white/70 ps-elevated dark:hover:bg-neutral-100 dark:hover:text-neutral-950"
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
              {/* Module de Gestion de Catalogue (Créer, Modifier, Supprimer, Désactiver) */}
              <button
                onClick={() => setIsCatalogManagerOpen(true)}
                className="flex items-center gap-1.5 rounded-lg border border-neutral-200 bg-neutral-50 px-3 py-1.5 text-xs font-medium text-neutral-800 hover:bg-neutral-950 hover:text-white dark:border-neutral-800 dark:bg-neutral-900 dark:text-neutral-200 transition-colors hover:border-neutral-950 dark:hover:border-white/70 ps-elevated dark:hover:bg-neutral-100 dark:hover:text-neutral-950"
                title="Ouvrir le module complet de gestion du catalogue et synchronisation PostgreSQL"
              >
                <SlidersHorizontal className="h-3.5 w-3.5 text-neutral-600 dark:text-neutral-400" />
                <span>Gérer Catalogue</span>
                <span className="rounded bg-emerald-500/10 px-1 py-0.2 text-[9px] font-mono text-emerald-600 dark:text-emerald-400">
                  DB
                </span>
              </button>

              {/* Ajuster grille */}
              <button
                onClick={() => setIsBulkAdjustOpen(true)}
                className="flex items-center gap-1.5 rounded-lg border border-neutral-200 bg-neutral-50 px-3 py-1.5 text-xs font-medium text-neutral-700 hover:bg-neutral-950 hover:text-white dark:border-neutral-800 dark:bg-neutral-900 dark:text-neutral-300 transition-colors hover:border-neutral-950 dark:hover:border-white/70 ps-elevated dark:hover:bg-neutral-100 dark:hover:text-neutral-950"
              >
                <SlidersHorizontal className="h-3.5 w-3.5" />
                <span>Ajuster Grille</span>
              </button>

              {/* Ajouter prestation */}
              <button
                onClick={handleOpenAdd}
                className="flex items-center gap-1.5 rounded-lg bg-neutral-900 px-3.5 py-1.5 text-xs font-medium text-white shadow-xs hover:bg-black dark:bg-neutral-100 dark:text-neutral-900 dark:hover:bg-white transition-colors"
              >
                <Plus className="h-3.5 w-3.5" />
                <span>Nouvelle Prestation</span>
              </button>
            </>
          )}
        </div>
      </div>

      {/* ONGLETS DES MODÈLES COMMERCIAUX DE L'ERP */}
      <div className="flex border-b border-neutral-200 dark:border-neutral-800">
        <button
          onClick={() => setMainTab('catalog')}
          className={`flex items-center gap-2 border-b-2 px-4 py-2.5 text-xs font-medium transition-colors ${
            mainTab === 'catalog'
              ? 'border-neutral-900 text-neutral-900 dark:border-neutral-100 dark:text-neutral-100 font-semibold'
              : 'border-transparent text-neutral-500 hover:text-neutral-950 dark:text-neutral-400 dark:hover:text-neutral-200'
          }`}
        >
          <Layers className="h-4 w-4" />
          <span>Modèles 1 & 2 : Prestations à la Pièce & au Kg ({services.length})</span>
        </button>

        <button
          onClick={() => setMainTab('subscriptions')}
          className={`flex items-center gap-2 border-b-2 px-4 py-2.5 text-xs font-medium transition-colors ${
            mainTab === 'subscriptions'
              ? 'border-neutral-900 text-neutral-900 dark:border-neutral-100 dark:text-neutral-100 font-semibold'
              : 'border-transparent text-neutral-500 hover:text-neutral-950 dark:text-neutral-400 dark:hover:text-neutral-200'
          }`}
        >
          <Calendar className="h-4 w-4" />
          <span>Modèle 3 : Forfaits & Abonnements Mensuels ({subscriptions.length})</span>
        </button>

        <button
          onClick={() => setMainTab('contracts')}
          className={`flex items-center gap-2 border-b-2 px-4 py-2.5 text-xs font-medium transition-colors ${
            mainTab === 'contracts'
              ? 'border-neutral-900 text-neutral-900 dark:border-neutral-100 dark:text-neutral-100 font-semibold'
              : 'border-transparent text-neutral-500 hover:text-neutral-950 dark:text-neutral-400 dark:hover:text-neutral-200'
          }`}
        >
          <Briefcase className="h-4 w-4" />
          <span>Modèle 4 : Contrats Professionnels B2B ({b2bContracts.length})</span>
        </button>
      </div>

      {/* PANNEAU CALCULATEUR DE DEVIS INSTANTANÉ (SIMULATEUR DE COMPTOIR) */}
      {isQuoteCalcOpen && (
        <div className="rounded-xl border border-neutral-200 bg-gradient-to-b from-neutral-50 to-white p-5 dark:border-neutral-800 dark:from-neutral-900 dark:to-[#0d0d0d] hover:border-neutral-950 dark:hover:border-white/70 ps-elevated">
          <div className="flex items-center justify-between border-b border-neutral-200/80 pb-3 dark:border-neutral-800">
            <div className="flex items-center gap-2">
              <ShoppingBag className="h-4 w-4 text-emerald-600 dark:text-emerald-400" />
              <div>
                <h2 className="text-sm font-semibold text-neutral-900 dark:text-neutral-100">
                  Simulateur de Devis & Calcul de Comptoir Express
                </h2>
                <p className="text-[11px] text-neutral-500 dark:text-neutral-400">
                  Ajoutez les articles du client pour calculer instantanément le montant total en FCFA.
                </p>
              </div>
            </div>
            <div className="flex items-center gap-4">
              <div className="flex items-center gap-3">
                <label className="flex items-center gap-1.5 text-xs cursor-pointer">
                  <input
                    type="checkbox"
                    checked={includeExpressInQuote}
                    onChange={(e) => setIncludeExpressInQuote(e.target.checked)}
                    className="rounded border-neutral-300 text-neutral-900"
                  />
                  <span>Option Express (+50%)</span>
                </label>
                <label className="flex items-center gap-1.5 text-xs cursor-pointer">
                  <input
                    type="checkbox"
                    checked={includePickupInQuote}
                    onChange={(e) => setIncludePickupInQuote(e.target.checked)}
                    className="rounded border-neutral-300 text-neutral-900"
                  />
                  <span>Collecte Abidjan (+1 500 FCFA)</span>
                </label>
              </div>
              <span className="font-mono text-base font-bold text-neutral-900 dark:text-neutral-100 tabular-nums">
                Total : {quoteTotal.toLocaleString('fr-FR')} FCFA ({quoteTotalPieces} pcs)
              </span>
              {quoteTotalPieces > 0 && (
                <div className="flex items-center gap-2">
                  <button
                    onClick={() => setIsPrintModalOpen(true)}
                    className="flex items-center gap-1.5 rounded-lg bg-emerald-600 px-2.5 py-1 text-xs font-semibold text-white shadow-2xs hover:bg-emerald-700 transition-colors"
                    title="Imprimer le ticket devis au format reçu thermique 80mm"
                  >
                    <Receipt className="h-3.5 w-3.5" />
                    <span>Ticket Devis (80mm)</span>
                  </button>
                  <button
                    onClick={() => setQuoteItems({})}
                    className="text-xs text-rose-600 hover:underline dark:text-rose-400"
                  >
                    Vider
                  </button>
                </div>
              )}
            </div>
          </div>

          <div className="mt-3 grid grid-cols-1 gap-2 sm:grid-cols-2 md:grid-cols-3 lg:grid-cols-4 max-h-72 overflow-y-auto pr-1">
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
                      className="flex h-6 w-6 items-center justify-center rounded bg-neutral-200 text-neutral-700 hover:bg-neutral-900/[0.16] dark:hover:bg-white/[0.16] disabled:opacity-30 dark:bg-neutral-800 dark:text-neutral-200"
                    >
                      -
                    </button>
                    <span className="w-5 text-center font-mono font-semibold">{qty}</span>
                    <button
                      onClick={() => handleUpdateQuoteQty(item.id, 1)}
                      className="flex h-6 w-6 items-center justify-center rounded bg-neutral-900 text-white hover:bg-black dark:hover:bg-white dark:bg-neutral-100 dark:text-neutral-900"
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

      {/* VUE 1 : CATALOGUE PRINCIPAL À LA PIÈCE ET AU KILO */}
      {mainTab === 'catalog' && (
        <div className="space-y-4">
          {/* Navigation par catégories (Les 5 piliers) */}
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
                      : 'text-neutral-600 hover:text-neutral-950 dark:text-neutral-400 dark:hover:text-neutral-200'
                  }`}
                >
                  <span className="hidden md:inline">{cat.label}</span>
                  <span className="md:hidden">{cat.short}</span>
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

          {/* Recherche, filtres modèles & tri */}
          <div className="flex flex-col gap-2.5 sm:flex-row sm:items-center sm:justify-between">
            <div className="relative flex-1 sm:max-w-md">
              <Search className="absolute left-3 top-2.5 h-3.5 w-3.5 text-neutral-400 dark:text-neutral-500" />
              <input
                type="text"
                placeholder="Rechercher article, soin (ex: boubou, couette, cintre, bazin)..."
                value={searchQuery}
                onChange={(e) => setSearchQuery(e.target.value)}
                className="w-full rounded-lg border border-neutral-200 bg-white pl-9 pr-3 py-1.5 text-xs text-neutral-900 placeholder:text-neutral-400 focus:border-neutral-900 focus:outline-hidden dark:border-neutral-800 dark:bg-neutral-900 dark:text-neutral-100 dark:placeholder:text-neutral-500 dark:focus:border-neutral-100 hover:border-neutral-950 dark:hover:border-white/70 ps-elevated"
              />
            </div>

            <div className="flex flex-wrap items-center gap-2">
              {/* Filtre Modèle de facturation */}
              <select
                value={modelFilter}
                onChange={(e) => setModelFilter(e.target.value as ServiceFilter)}
                className="rounded-lg border border-neutral-200 bg-white px-2.5 py-1.5 text-xs text-neutral-700 dark:border-neutral-800 dark:bg-neutral-900 dark:text-neutral-300 focus:outline-hidden hover:border-neutral-950 dark:hover:border-white/70 ps-elevated"
              >
                <option value="all">Tous les modes de facturation</option>
                <option value="piece">Paiement à la pièce</option>
                <option value="kilogram">Facturation au kg</option>
                <option value="quote">Sur devis / Luxe</option>
              </select>

              {/* Filtre Statut */}
              <select
                value={statusFilter}
                onChange={(e) => setStatusFilter(e.target.value as any)}
                className="rounded-lg border border-neutral-200 bg-white px-2.5 py-1.5 text-xs text-neutral-700 dark:border-neutral-800 dark:bg-neutral-900 dark:text-neutral-300 focus:outline-hidden hover:border-neutral-950 dark:hover:border-white/70 ps-elevated"
              >
                <option value="all">Tous les états</option>
                <option value="active">Actives au comptoir</option>
                <option value="inactive">Désactivées</option>
              </select>

              {/* Tri */}
              <select
                value={sortOrder}
                onChange={(e) => setSortOrder(e.target.value as any)}
                className="rounded-lg border border-neutral-200 bg-white px-2.5 py-1.5 text-xs text-neutral-700 dark:border-neutral-800 dark:bg-neutral-900 dark:text-neutral-300 focus:outline-hidden hover:border-neutral-950 dark:hover:border-white/70 ps-elevated"
              >
                <option value="name">Trier par nom (A-Z)</option>
                <option value="price-asc">Tarif croissant (FCFA)</option>
                <option value="price-desc">Tarif décroissant (FCFA)</option>
              </select>
            </div>
          </div>

          {/* TABLEAU DES PRESTATIONS */}
          <div className="overflow-hidden rounded-xl border border-neutral-200 bg-white shadow-2xs dark:border-neutral-800 dark:bg-[#0f0f0f]">
            <div className="overflow-x-auto">
              <table className="w-full text-left text-xs">
                <thead>
                  <tr className="border-b border-neutral-100 bg-neutral-50/75 text-[11px] font-medium text-neutral-500 dark:border-neutral-800 dark:bg-neutral-900/60 dark:text-neutral-400">
                    <th className="px-5 py-3">Article & Traitement Spécifique</th>
                    <th className="px-4 py-3">Catégorie</th>
                    <th className="px-4 py-3">Unité & Modèle</th>
                    <th className="px-4 py-3">Délai Estimé</th>
                    <th className="px-4 py-3">Options & Particularités</th>
                    <th className="px-4 py-3 text-right">Tarif (FCFA)</th>
                    <th className="px-4 py-3 text-center">Statut</th>
                    {canEdit && <th className="px-5 py-3 text-right">Actions</th>}
                  </tr>
                </thead>
                <tbody className="divide-y divide-neutral-100 dark:divide-neutral-800/80">
                  {filteredServices.length === 0 ? (
                    <tr>
                      <td colSpan={canEdit ? 8 : 7} className="px-5 py-16 text-center">
                        <div className="mx-auto flex max-w-xs flex-col items-center justify-center text-center">
                          <Tag className="h-8 w-8 text-neutral-300 dark:text-neutral-700 mb-2" />
                          <p className="text-xs font-semibold text-neutral-800 dark:text-neutral-200">
                            Aucune prestation correspondante
                          </p>
                          <p className="mt-1 text-[11px] text-neutral-500 dark:text-neutral-400">
                            Essayez de modifier vos critères de recherche ou ajoutez un nouveau service.
                          </p>
                          {canEdit && (
                            <button
                              onClick={handleOpenAdd}
                              className="mt-4 flex items-center gap-1.5 rounded-md bg-neutral-900 px-3 py-1.5 text-xs text-white hover:bg-black dark:bg-neutral-100 dark:text-neutral-900 dark:hover:bg-white"
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
                              : 'hover:bg-neutral-900/[0.06] dark:hover:bg-neutral-900/50'
                          }`}
                        >
                          {/* Article & Traitement */}
                          <td className="px-5 py-3.5">
                            <div className="space-y-0.5">
                              <div className="flex items-center gap-1.5">
                                <span className="font-semibold text-neutral-900 dark:text-neutral-100">
                                  {item.article || item.name}
                                </span>
                                {item.treatment && (
                                  <span className="font-normal text-neutral-500 dark:text-neutral-400">
                                    — {item.treatment}
                                  </span>
                                )}
                              </div>
                              {item.description && (
                                <p className="text-[11px] text-neutral-500 dark:text-neutral-400 max-w-sm line-clamp-1">
                                  {item.description}
                                </p>
                              )}
                            </div>
                          </td>

                          {/* Catégorie */}
                          <td className="px-4 py-3.5">
                            <span className="inline-flex items-center rounded-md border border-neutral-200 bg-neutral-50 px-2 py-0.5 font-mono text-[10px] text-neutral-700 dark:border-neutral-800 dark:bg-neutral-900 dark:text-neutral-300 hover:border-neutral-950 dark:hover:border-white/70 ps-elevated">
                              {item.category}
                            </span>
                          </td>

                          {/* Unité & Modèle */}
                          <td className="px-4 py-3.5">
                            <div className="space-y-0.5 font-mono text-[11px]">
                              <span className="font-medium text-neutral-800 dark:text-neutral-200">
                                {item.unit}
                              </span>
                              {item.pricingModel === 'kilogram' && (
                                <span className="block text-[10px] text-indigo-600 dark:text-indigo-400">
                                  Au kilogramme
                                </span>
                              )}
                            </div>
                          </td>

                          {/* Délai estimé */}
                          <td className="px-4 py-3.5 font-mono text-[11px] text-neutral-600 dark:text-neutral-400">
                            <span className="inline-flex items-center gap-1">
                              <Clock className="h-3 w-3 text-neutral-400" />
                              {item.durationHours}h ({Math.max(1, Math.round(item.durationHours / 24))}j)
                            </span>
                          </td>

                          {/* Options & Badges (Express, Pickup, Devis, Sous-traitance) */}
                          <td className="px-4 py-3.5">
                            <div className="flex flex-wrap items-center gap-1">
                              {item.isExpressAvailable && (
                                <span className="inline-flex items-center gap-0.5 rounded px-1.5 py-0.5 text-[9px] font-medium bg-amber-50 text-amber-700 border border-amber-200 dark:bg-amber-950/40 dark:text-amber-300 dark:border-amber-800">
                                  <Zap className="h-2.5 w-2.5" />
                                  <span>Express</span>
                                </span>
                              )}
                              {item.isPickupAvailable && (
                                <span className="inline-flex items-center gap-0.5 rounded px-1.5 py-0.5 text-[9px] font-medium bg-blue-50 text-blue-700 border border-blue-200 dark:bg-blue-950/40 dark:text-blue-300 dark:border-blue-800">
                                  <Truck className="h-2.5 w-2.5" />
                                  <span>Collecte</span>
                                </span>
                              )}
                              {item.isOutsourced && (
                                <span className="inline-flex items-center gap-0.5 rounded px-1.5 py-0.5 text-[9px] font-medium bg-purple-50 text-purple-700 border border-purple-200 dark:bg-purple-950/40 dark:text-purple-300 dark:border-purple-800">
                                  <span>Externalisé</span>
                                </span>
                              )}
                              {item.requiresQuote && (
                                <span className="inline-flex items-center gap-0.5 rounded px-1.5 py-0.5 text-[9px] font-medium bg-rose-50 text-rose-700 border border-rose-200 dark:bg-rose-950/40 dark:text-rose-300 dark:border-rose-800">
                                  <span>Sur devis</span>
                                </span>
                              )}
                            </div>
                          </td>

                          {/* Tarif en FCFA */}
                          <td className="px-4 py-3.5 text-right font-mono text-xs font-bold text-neutral-900 dark:text-neutral-100 tabular-nums">
                            {item.requiresQuote ? 'Dès ' : ''}
                            {item.price.toLocaleString('fr-FR')} FCFA
                          </td>

                          {/* Statut Actif / Inactif */}
                          <td className="px-4 py-3.5 text-center">
                            <button
                              onClick={() => canEdit && handleToggleActive(item)}
                              disabled={!canEdit}
                              title={isActive ? 'Désactiver (archivage)' : 'Réactiver au comptoir'}
                              className={`inline-flex items-center gap-1 rounded-full px-2 py-0.5 text-[10px] font-medium transition-colors ${
                                isActive
                                  ? 'bg-emerald-50 text-emerald-700 border border-emerald-200/60 dark:bg-emerald-950/40 dark:text-emerald-400 dark:border-emerald-800/40'
                                  : 'bg-neutral-100 text-neutral-500 border border-neutral-200 dark:bg-neutral-800 dark:text-neutral-400 dark:border-neutral-700'
                              }`}
                            >
                              <span
                                className={`h-1.5 w-1.5 rounded-full ${isActive ? 'bg-emerald-500' : 'bg-neutral-400'}`}
                              />
                              <span>{isActive ? 'Actif' : 'Inactif'}</span>
                            </button>
                          </td>

                          {/* Actions Admin */}
                          {canEdit && (
                            <td className="px-5 py-3.5 text-right">
                              <div className="flex items-center justify-end gap-1">
                                <button
                                  onClick={() => handleDuplicate(item)}
                                  className="rounded-md p-1.5 text-neutral-400 hover:bg-neutral-950 hover:text-white transition-colors dark:hover:bg-neutral-100 dark:hover:text-neutral-950"
                                  title="Décliner cet article (ex: Repassage seul)"
                                >
                                  <Copy className="h-3.5 w-3.5" />
                                </button>
                                <button
                                  onClick={() => handleOpenEdit(item)}
                                  className="rounded-md p-1.5 text-neutral-500 hover:bg-neutral-950 hover:text-white dark:text-neutral-400 transition-colors dark:hover:bg-neutral-100 dark:hover:text-neutral-950"
                                  title="Modifier la prestation et le tarif"
                                >
                                  <Edit2 className="h-3.5 w-3.5" />
                                </button>
                                <button
                                  onClick={() => handleRequestDelete(item)}
                                  className="rounded-md p-1.5 text-neutral-400 hover:bg-rose-50 hover:text-rose-600 dark:hover:bg-rose-950/40 dark:hover:text-rose-400 transition-colors"
                                  title="Supprimer définitivement cette prestation"
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
        </div>
      )}

      {/* VUE 2 : MODÈLE 3 — FORFAITS & ABONNEMENTS MENSUELS */}
      {mainTab === 'subscriptions' && (
        <div className="space-y-4">
          <div className="rounded-xl border border-neutral-200 bg-neutral-50/50 p-4 dark:border-neutral-800 dark:bg-neutral-900/30 hover:border-neutral-950 dark:hover:border-white/70 ps-elevated">
            <h2 className="text-sm font-semibold text-neutral-900 dark:text-neutral-100">
              Modèle 3 — Abonnements Mensuels & Forfaits Prédéfinis
            </h2>
            <p className="mt-1 text-xs text-neutral-500 dark:text-neutral-400">
              Formules avec volume de pièces inclus, fréquence de collecte programmée et tarification des
              dépassements. Adaptées aux cadres, familles et résidences d'Abidjan.
            </p>
          </div>

          <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-4">
            {subscriptions.map((plan) => (
              <div
                key={plan.id}
                className={`relative flex flex-col justify-between rounded-xl border p-5 transition-all ${
                  plan.isPopular
                    ? 'border-neutral-900 bg-white shadow-md dark:border-neutral-100 dark:bg-[#111111]'
                    : 'border-neutral-200 bg-white dark:border-neutral-800 dark:bg-[#0c0c0c]'
                }`}
              >
                {plan.isPopular && (
                  <span className="absolute -top-2.5 right-4 rounded-full bg-neutral-900 px-2.5 py-0.5 text-[10px] font-semibold uppercase tracking-wider text-white dark:bg-neutral-100 dark:text-neutral-900">
                    Recommandé
                  </span>
                )}

                <div>
                  <span className="text-[10px] font-mono font-medium text-neutral-500 uppercase tracking-wider">
                    {plan.target}
                  </span>
                  <h3 className="mt-1 text-base font-bold text-neutral-900 dark:text-neutral-100">
                    {plan.name}
                  </h3>
                  <div className="mt-3 flex items-baseline gap-1">
                    <span className="font-mono text-2xl font-extrabold text-neutral-900 dark:text-neutral-50">
                      {plan.monthlyPrice.toLocaleString('fr-FR')}
                    </span>
                    <span className="text-xs text-neutral-500">FCFA / mois</span>
                  </div>

                  <p className="mt-3 text-xs text-neutral-600 dark:text-neutral-400 leading-relaxed">
                    {plan.description}
                  </p>

                  <ul className="mt-4 space-y-2 border-t border-neutral-100 pt-3 text-xs text-neutral-600 dark:border-neutral-800 dark:text-neutral-300">
                    <li className="flex items-center gap-2">
                      <CheckCircle2 className="h-3.5 w-3.5 text-emerald-600" />
                      <span>
                        <strong>{plan.quotaPieces} pièces</strong> incluses par mois
                      </span>
                    </li>
                    <li className="flex items-center gap-2">
                      <CheckCircle2 className="h-3.5 w-3.5 text-emerald-600" />
                      <span>{plan.collectionFrequency}</span>
                    </li>
                    <li className="flex items-center gap-2">
                      <CheckCircle2 className="h-3.5 w-3.5 text-emerald-600" />
                      <span>Dépassement : {plan.overagePricePerPiece} FCFA / pièce</span>
                    </li>
                  </ul>
                </div>

                <div className="mt-5 pt-3 border-t border-neutral-100 dark:border-neutral-800 flex items-center justify-between">
                  <span className="text-[11px] font-mono text-emerald-600 dark:text-emerald-400">
                    ● Formule active
                  </span>
                  <button
                    onClick={() => {
                      if (showToast) showToast(`Souscription pour "${plan.name}" disponible au comptoir.`);
                    }}
                    className="rounded-lg bg-neutral-900 px-3 py-1.5 text-xs font-medium text-white hover:bg-black dark:bg-neutral-100 dark:text-neutral-900 dark:hover:bg-white"
                  >
                    Souscrire client
                  </button>
                </div>
              </div>
            ))}
          </div>
        </div>
      )}

      {/* VUE 3 : MODÈLE 4 — CONTRATS PROFESSIONNELS B2B */}
      {mainTab === 'contracts' && (
        <div className="space-y-4">
          <div className="rounded-xl border border-neutral-200 bg-neutral-50/50 p-4 dark:border-neutral-800 dark:bg-neutral-900/30 hover:border-neutral-950 dark:hover:border-white/70 ps-elevated">
            <h2 className="text-sm font-semibold text-neutral-900 dark:text-neutral-100">
              Modèle 4 — Contrats Professionnels & Partenariats B2B
            </h2>
            <p className="mt-1 text-xs text-neutral-500 dark:text-neutral-400">
              Tarification négociée au kilogramme, à la pièce ou au forfait pour hôtels, restaurants,
              cliniques et salons de beauté.
            </p>
          </div>

          <div className="overflow-hidden rounded-xl border border-neutral-200 bg-white shadow-2xs dark:border-neutral-800 dark:bg-[#0f0f0f]">
            <table className="w-full text-left text-xs">
              <thead>
                <tr className="border-b border-neutral-100 bg-neutral-50/75 text-[11px] font-medium text-neutral-500 dark:border-neutral-800 dark:bg-neutral-900/60 dark:text-neutral-400">
                  <th className="px-5 py-3">Établissement Client</th>
                  <th className="px-4 py-3">Secteur d'Activité</th>
                  <th className="px-4 py-3">Mode de Facturation</th>
                  <th className="px-4 py-3">Tarif Négocié</th>
                  <th className="px-4 py-3">Cycle de Facturation</th>
                  <th className="px-4 py-3">Contact & Téléphone</th>
                  <th className="px-4 py-3 text-center">Statut Contrat</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-neutral-100 dark:divide-neutral-800/80">
                {b2bContracts.map((contract) => (
                  <tr key={contract.id} className="hover:bg-neutral-900/[0.06] dark:hover:bg-neutral-900/50">
                    <td className="px-5 py-3.5 font-semibold text-neutral-900 dark:text-neutral-100">
                      {contract.clientName}
                    </td>
                    <td className="px-4 py-3.5">
                      <span className="inline-flex items-center rounded-md border border-neutral-200 bg-neutral-50 px-2 py-0.5 font-mono text-[11px] text-neutral-700 dark:border-neutral-800 dark:bg-neutral-900 dark:text-neutral-300 hover:border-neutral-950 dark:hover:border-white/70 ps-elevated">
                        {contract.businessType}
                      </span>
                    </td>
                    <td className="px-4 py-3.5 font-mono text-neutral-600 dark:text-neutral-400">
                      {contract.billingType === 'kilogram'
                        ? 'Au kilogramme'
                        : contract.billingType === 'piece'
                          ? 'À la pièce'
                          : 'Forfait mensuel'}
                    </td>
                    <td className="px-4 py-3.5 font-mono font-bold text-neutral-900 dark:text-neutral-100">
                      {contract.ratePerUnit.toLocaleString('fr-FR')} FCFA{' '}
                      {contract.billingType === 'kilogram'
                        ? '/ kg'
                        : contract.billingType === 'piece'
                          ? '/ pièce'
                          : '/ mois'}
                    </td>
                    <td className="px-4 py-3.5 text-neutral-600 dark:text-neutral-400">
                      {contract.billingCycle}
                    </td>
                    <td className="px-4 py-3.5 text-neutral-600 dark:text-neutral-400">
                      <div>{contract.contactPerson}</div>
                      <div className="font-mono text-[11px] text-neutral-400">{contract.phone}</div>
                    </td>
                    <td className="px-4 py-3.5 text-center">
                      <span className="inline-flex items-center gap-1 rounded-full px-2 py-0.5 text-[10px] font-medium bg-emerald-50 text-emerald-700 border border-emerald-200 dark:bg-emerald-950/40 dark:text-emerald-400 dark:border-emerald-800">
                        <span className="h-1.5 w-1.5 rounded-full bg-emerald-500" />
                        <span>Actif</span>
                      </span>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </div>
      )}

      {/* MODAL AJOUT PRESTATION DÉTAILLÉ */}
      {isAddModalOpen && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 p-4 backdrop-blur-xs">
          <div className="w-full max-w-lg rounded-xl border border-neutral-200 bg-white p-6 shadow-xl dark:border-neutral-800 dark:bg-[#111111] shadow-2xl shadow-neutral-950/25 dark:shadow-black/80 ring-1 ring-black/5 dark:ring-white/10">
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
                    Structure métier complète (Article, Soin, Modèle de facturation & Options).
                  </p>
                </div>
              </div>
              <button
                onClick={() => setIsAddModalOpen(false)}
                className="text-neutral-400 hover:text-neutral-950 dark:hover:text-neutral-200"
              >
                <X className="h-4 w-4" />
              </button>
            </div>

            <form onSubmit={handleSaveAdd} className="mt-4 space-y-3.5">
              <div className="grid grid-cols-2 gap-3">
                <div>
                  <label className="block text-xs font-medium text-neutral-700 dark:text-neutral-300">
                    Article (Vêtement / Linge) <span className="text-rose-500">*</span>
                  </label>
                  <input
                    type="text"
                    required
                    placeholder="Ex: Chemise, Boubou brodé, Drap..."
                    value={formArticle}
                    onChange={(e) => setFormArticle(e.target.value)}
                    className="mt-1 w-full rounded-lg border border-neutral-200 px-3 py-1.5 text-xs text-neutral-900 focus:border-neutral-900 focus:outline-hidden dark:border-neutral-800 dark:bg-neutral-900 dark:text-neutral-100 dark:focus:border-neutral-100"
                  />
                </div>

                <div>
                  <label className="block text-xs font-medium text-neutral-700 dark:text-neutral-300">
                    Traitement / Soin <span className="text-rose-500">*</span>
                  </label>
                  <input
                    type="text"
                    required
                    placeholder="Ex: Lavage + repassage, Repassage seul, Soin délicat..."
                    value={formTreatment}
                    onChange={(e) => setFormTreatment(e.target.value)}
                    className="mt-1 w-full rounded-lg border border-neutral-200 px-3 py-1.5 text-xs text-neutral-900 focus:border-neutral-900 focus:outline-hidden dark:border-neutral-800 dark:bg-neutral-900 dark:text-neutral-100 dark:focus:border-neutral-100"
                  />
                </div>
              </div>

              <div className="grid grid-cols-2 gap-3">
                <div>
                  <label className="block text-xs font-medium text-neutral-700 dark:text-neutral-300">
                    Catégorie Principale
                  </label>
                  <select
                    value={formCategory}
                    onChange={(e) => setFormCategory(e.target.value)}
                    className="mt-1 w-full rounded-lg border border-neutral-200 px-3 py-1.5 text-xs text-neutral-900 focus:border-neutral-900 focus:outline-hidden dark:border-neutral-800 dark:bg-neutral-900 dark:text-neutral-100"
                  >
                    <option value="Vêtements du quotidien">A. Vêtements du quotidien</option>
                    <option value="Vêtements traditionnels & cérémoniels">
                      B. Vêtements traditionnels & cérémoniels
                    </option>
                    <option value="Linge de maison">C. Linge de maison</option>
                    <option value="Services complémentaires & Options">
                      D. Services complémentaires & Options
                    </option>
                    <option value="Prestations spécialisées & Luxe">
                      E. Prestations spécialisées & Luxe
                    </option>
                  </select>
                </div>

                <div>
                  <label className="block text-xs font-medium text-neutral-700 dark:text-neutral-300">
                    Modèle de Facturation
                  </label>
                  <select
                    value={formPricingModel}
                    onChange={(e) => setFormPricingModel(e.target.value as any)}
                    className="mt-1 w-full rounded-lg border border-neutral-200 px-3 py-1.5 text-xs text-neutral-900 focus:border-neutral-900 focus:outline-hidden dark:border-neutral-800 dark:bg-neutral-900 dark:text-neutral-100"
                  >
                    <option value="piece">Modèle 1 : Paiement à la pièce</option>
                    <option value="kilogram">Modèle 2 : Facturation au kilogramme</option>
                  </select>
                </div>
              </div>

              <div className="grid grid-cols-3 gap-3">
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
                    className="mt-1 w-full rounded-lg border border-neutral-200 px-3 py-1.5 text-xs text-neutral-900 focus:border-neutral-900 focus:outline-hidden dark:border-neutral-800 dark:bg-neutral-900 dark:text-neutral-100 tabular-nums font-mono font-semibold"
                  />
                </div>

                <div>
                  <label className="block text-xs font-medium text-neutral-700 dark:text-neutral-300">
                    Unité
                  </label>
                  <select
                    value={formUnit}
                    onChange={(e) => setFormUnit(e.target.value)}
                    className="mt-1 w-full rounded-lg border border-neutral-200 px-3 py-1.5 text-xs text-neutral-900 focus:border-neutral-900 focus:outline-hidden dark:border-neutral-800 dark:bg-neutral-900 dark:text-neutral-100"
                  >
                    <option value="pièce">pièce</option>
                    <option value="ensemble">ensemble</option>
                    <option value="kg">kg (au poids)</option>
                    <option value="panneau">panneau</option>
                    <option value="m²">m²</option>
                    <option value="paire">la paire</option>
                    <option value="supplément">supplément</option>
                  </select>
                </div>

                <div>
                  <label className="block text-xs font-medium text-neutral-700 dark:text-neutral-300">
                    Délai (heures)
                  </label>
                  <input
                    type="number"
                    required
                    min={1}
                    value={formDuration}
                    onChange={(e) => setFormDuration(Number(e.target.value))}
                    className="mt-1 w-full rounded-lg border border-neutral-200 px-3 py-1.5 text-xs text-neutral-900 focus:border-neutral-900 focus:outline-hidden dark:border-neutral-800 dark:bg-neutral-900 dark:text-neutral-100 font-mono"
                  />
                </div>
              </div>

              {/* Options & Attributs modernes */}
              <div className="rounded-lg border border-neutral-200/80 bg-neutral-50 p-3 space-y-2 dark:border-neutral-800 dark:bg-neutral-900/40 hover:border-neutral-950 dark:hover:border-white/70 ps-elevated">
                <p className="text-[11px] font-semibold text-neutral-800 dark:text-neutral-200">
                  Options et Traitements Avancés :
                </p>
                <div className="grid grid-cols-2 gap-2 text-xs">
                  <label className="flex items-center gap-2 cursor-pointer">
                    <input
                      type="checkbox"
                      checked={formIsExpress}
                      onChange={(e) => setFormIsExpress(e.target.checked)}
                      className="rounded border-neutral-300 text-neutral-900"
                    />
                    <span>Option Express 24h</span>
                  </label>

                  <label className="flex items-center gap-2 cursor-pointer">
                    <input
                      type="checkbox"
                      checked={formIsPickup}
                      onChange={(e) => setFormIsPickup(e.target.checked)}
                      className="rounded border-neutral-300 text-neutral-900"
                    />
                    <span>Disponible en collecte</span>
                  </label>

                  <label className="flex items-center gap-2 cursor-pointer">
                    <input
                      type="checkbox"
                      checked={formIsOutsourced}
                      onChange={(e) => setFormIsOutsourced(e.target.checked)}
                      className="rounded border-neutral-300 text-neutral-900"
                    />
                    <span>Prestation externalisée</span>
                  </label>

                  <label className="flex items-center gap-2 cursor-pointer">
                    <input
                      type="checkbox"
                      checked={formRequiresQuote}
                      onChange={(e) => setFormRequiresQuote(e.target.checked)}
                      className="rounded border-neutral-300 text-neutral-900"
                    />
                    <span>Sur devis uniquement</span>
                  </label>
                </div>

                {formIsExpress && (
                  <div className="pt-2 border-t border-neutral-200/60 dark:border-neutral-800 flex items-center gap-2">
                    <span className="text-[11px] text-neutral-600 dark:text-neutral-400">
                      Supplément express :
                    </span>
                    <input
                      type="number"
                      value={formExpressSurcharge}
                      onChange={(e) => setFormExpressSurcharge(Number(e.target.value))}
                      className="w-24 rounded border border-neutral-200 px-2 py-0.5 text-xs font-mono dark:border-neutral-700 dark:bg-neutral-800"
                    />
                    <span className="text-[11px] font-mono">FCFA</span>
                  </div>
                )}
              </div>

              <div>
                <label className="block text-xs font-medium text-neutral-700 dark:text-neutral-300">
                  Consignes d'atelier & Description
                </label>
                <textarea
                  rows={2}
                  placeholder="Ex: Cintre obligatoire, protection broderies, traitement anti-décoloration..."
                  value={formDescription}
                  onChange={(e) => setFormDescription(e.target.value)}
                  className="mt-1 w-full rounded-lg border border-neutral-200 px-3 py-1.5 text-xs text-neutral-900 focus:border-neutral-900 focus:outline-hidden dark:border-neutral-800 dark:bg-neutral-900 dark:text-neutral-100 resize-none"
                />
              </div>

              <div className="flex items-center gap-2 pt-1">
                <input
                  type="checkbox"
                  id="formIsActiveCheck2"
                  checked={formIsActive}
                  onChange={(e) => setFormIsActive(e.target.checked)}
                  className="rounded border-neutral-300 text-neutral-900"
                />
                <label
                  htmlFor="formIsActiveCheck2"
                  className="text-xs text-neutral-700 dark:text-neutral-300"
                >
                  Activer immédiatement cette prestation dans l'ERP
                </label>
              </div>

              <div className="mt-5 flex justify-end gap-2 border-t border-neutral-100 pt-3 dark:border-neutral-800">
                <button
                  type="button"
                  onClick={() => setIsAddModalOpen(false)}
                  className="rounded-lg border border-neutral-200 px-3 py-1.5 text-xs font-medium text-neutral-700 hover:bg-neutral-900/[0.06] dark:border-neutral-700 dark:text-neutral-300 dark:hover:bg-neutral-800"
                >
                  Annuler
                </button>
                <button
                  type="submit"
                  disabled={isSubmitting}
                  className="rounded-lg bg-neutral-900 px-4 py-1.5 text-xs font-medium text-white hover:bg-black dark:bg-neutral-100 dark:text-neutral-900 dark:hover:bg-white disabled:opacity-50"
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
          <div className="w-full max-w-lg rounded-xl border border-neutral-200 bg-white p-6 shadow-xl dark:border-neutral-800 dark:bg-[#111111] shadow-2xl shadow-neutral-950/25 dark:shadow-black/80 ring-1 ring-black/5 dark:ring-white/10">
            <div className="flex items-center justify-between border-b border-neutral-100 pb-3 dark:border-neutral-800">
              <div className="flex items-center gap-2">
                <div className="flex h-7 w-7 items-center justify-center rounded-lg bg-neutral-900 text-white dark:bg-neutral-100 dark:text-neutral-900">
                  <Edit2 className="h-4 w-4" />
                </div>
                <div>
                  <h2 className="text-sm font-semibold text-neutral-900 dark:text-neutral-100">
                    Modifier la Prestation & Paramètres
                  </h2>
                  <p className="text-[11px] text-neutral-500 dark:text-neutral-400">
                    Mise à jour en temps réel dans PostgreSQL.
                  </p>
                </div>
              </div>
              <button
                onClick={() => setEditingItem(null)}
                className="text-neutral-400 hover:text-neutral-950 dark:hover:text-neutral-200"
              >
                <X className="h-4 w-4" />
              </button>
            </div>

            <form onSubmit={handleSaveEdit} className="mt-4 space-y-3.5">
              <div className="grid grid-cols-2 gap-3">
                <div>
                  <label className="block text-xs font-medium text-neutral-700 dark:text-neutral-300">
                    Article (Vêtement / Linge)
                  </label>
                  <input
                    type="text"
                    required
                    value={formArticle}
                    onChange={(e) => setFormArticle(e.target.value)}
                    className="mt-1 w-full rounded-lg border border-neutral-200 px-3 py-1.5 text-xs text-neutral-900 dark:border-neutral-800 dark:bg-neutral-900 dark:text-neutral-100"
                  />
                </div>

                <div>
                  <label className="block text-xs font-medium text-neutral-700 dark:text-neutral-300">
                    Traitement / Soin
                  </label>
                  <input
                    type="text"
                    required
                    value={formTreatment}
                    onChange={(e) => setFormTreatment(e.target.value)}
                    className="mt-1 w-full rounded-lg border border-neutral-200 px-3 py-1.5 text-xs text-neutral-900 dark:border-neutral-800 dark:bg-neutral-900 dark:text-neutral-100"
                  />
                </div>
              </div>

              <div className="grid grid-cols-2 gap-3">
                <div>
                  <label className="block text-xs font-medium text-neutral-700 dark:text-neutral-300">
                    Catégorie
                  </label>
                  <select
                    value={formCategory}
                    onChange={(e) => setFormCategory(e.target.value)}
                    className="mt-1 w-full rounded-lg border border-neutral-200 px-3 py-1.5 text-xs text-neutral-900 dark:border-neutral-800 dark:bg-neutral-900 dark:text-neutral-100"
                  >
                    <option value="Vêtements du quotidien">A. Vêtements du quotidien</option>
                    <option value="Vêtements traditionnels & cérémoniels">
                      B. Vêtements traditionnels & cérémoniels
                    </option>
                    <option value="Linge de maison">C. Linge de maison</option>
                    <option value="Services complémentaires & Options">
                      D. Services complémentaires & Options
                    </option>
                    <option value="Prestations spécialisées & Luxe">
                      E. Prestations spécialisées & Luxe
                    </option>
                  </select>
                </div>

                <div>
                  <label className="block text-xs font-medium text-neutral-700 dark:text-neutral-300">
                    Modèle de Facturation
                  </label>
                  <select
                    value={formPricingModel}
                    onChange={(e) => setFormPricingModel(e.target.value as any)}
                    className="mt-1 w-full rounded-lg border border-neutral-200 px-3 py-1.5 text-xs text-neutral-900 dark:border-neutral-800 dark:bg-neutral-900 dark:text-neutral-100"
                  >
                    <option value="piece">Modèle 1 : Paiement à la pièce</option>
                    <option value="kilogram">Modèle 2 : Facturation au kilogramme</option>
                  </select>
                </div>
              </div>

              <div className="grid grid-cols-3 gap-3">
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
                    className="mt-1 w-full rounded-lg border border-neutral-200 px-3 py-1.5 text-xs font-mono font-bold dark:border-neutral-800 dark:bg-neutral-900 dark:text-neutral-100"
                  />
                </div>

                <div>
                  <label className="block text-xs font-medium text-neutral-700 dark:text-neutral-300">
                    Unité
                  </label>
                  <select
                    value={formUnit}
                    onChange={(e) => setFormUnit(e.target.value)}
                    className="mt-1 w-full rounded-lg border border-neutral-200 px-3 py-1.5 text-xs dark:border-neutral-800 dark:bg-neutral-900 dark:text-neutral-100"
                  >
                    <option value="pièce">pièce</option>
                    <option value="ensemble">ensemble</option>
                    <option value="kg">kg (au poids)</option>
                    <option value="panneau">panneau</option>
                    <option value="m²">m²</option>
                    <option value="paire">la paire</option>
                    <option value="supplément">supplément</option>
                  </select>
                </div>

                <div>
                  <label className="block text-xs font-medium text-neutral-700 dark:text-neutral-300">
                    Délai (heures)
                  </label>
                  <input
                    type="number"
                    required
                    min={1}
                    value={formDuration}
                    onChange={(e) => setFormDuration(Number(e.target.value))}
                    className="mt-1 w-full rounded-lg border border-neutral-200 px-3 py-1.5 text-xs font-mono dark:border-neutral-800 dark:bg-neutral-900 dark:text-neutral-100"
                  />
                </div>
              </div>

              <div className="rounded-lg border border-neutral-200/80 bg-neutral-50 p-3 space-y-2 dark:border-neutral-800 dark:bg-neutral-900/40 hover:border-neutral-950 dark:hover:border-white/70 ps-elevated">
                <p className="text-[11px] font-semibold text-neutral-800 dark:text-neutral-200">
                  Options et Traitements Avancés :
                </p>
                <div className="grid grid-cols-2 gap-2 text-xs">
                  <label className="flex items-center gap-2 cursor-pointer">
                    <input
                      type="checkbox"
                      checked={formIsExpress}
                      onChange={(e) => setFormIsExpress(e.target.checked)}
                      className="rounded border-neutral-300 text-neutral-900"
                    />
                    <span>Option Express 24h</span>
                  </label>

                  <label className="flex items-center gap-2 cursor-pointer">
                    <input
                      type="checkbox"
                      checked={formIsPickup}
                      onChange={(e) => setFormIsPickup(e.target.checked)}
                      className="rounded border-neutral-300 text-neutral-900"
                    />
                    <span>Disponible en collecte</span>
                  </label>

                  <label className="flex items-center gap-2 cursor-pointer">
                    <input
                      type="checkbox"
                      checked={formIsOutsourced}
                      onChange={(e) => setFormIsOutsourced(e.target.checked)}
                      className="rounded border-neutral-300 text-neutral-900"
                    />
                    <span>Prestation externalisée</span>
                  </label>

                  <label className="flex items-center gap-2 cursor-pointer">
                    <input
                      type="checkbox"
                      checked={formRequiresQuote}
                      onChange={(e) => setFormRequiresQuote(e.target.checked)}
                      className="rounded border-neutral-300 text-neutral-900"
                    />
                    <span>Sur devis uniquement</span>
                  </label>
                </div>
              </div>

              <div>
                <label className="block text-xs font-medium text-neutral-700 dark:text-neutral-300">
                  Description / Consignes
                </label>
                <textarea
                  rows={2}
                  value={formDescription}
                  onChange={(e) => setFormDescription(e.target.value)}
                  className="mt-1 w-full rounded-lg border border-neutral-200 px-3 py-1.5 text-xs dark:border-neutral-800 dark:bg-neutral-900 dark:text-neutral-100 resize-none"
                />
              </div>

              <div className="flex items-center gap-2 pt-1">
                <input
                  type="checkbox"
                  id="formEditActiveCheck2"
                  checked={formIsActive}
                  onChange={(e) => setFormIsActive(e.target.checked)}
                  className="rounded border-neutral-300 text-neutral-900"
                />
                <label
                  htmlFor="formEditActiveCheck2"
                  className="text-xs text-neutral-700 dark:text-neutral-300"
                >
                  Prestation active et facturable au comptoir
                </label>
              </div>

              <div className="mt-5 flex items-center justify-between border-t border-neutral-100 pt-3 dark:border-neutral-800">
                <button
                  type="button"
                  onClick={() => {
                    const item = editingItem;
                    setEditingItem(null);
                    handleRequestDelete(item);
                  }}
                  className="inline-flex items-center gap-1.5 rounded-lg px-2.5 py-1.5 text-xs font-medium text-rose-600 hover:bg-rose-50 dark:text-rose-400 dark:hover:bg-rose-950/30 transition-colors"
                >
                  <Trash2 className="h-3.5 w-3.5" />
                  <span>Supprimer cette prestation</span>
                </button>
                <div className="flex items-center gap-2">
                  <button
                    type="button"
                    onClick={() => setEditingItem(null)}
                    className="rounded-lg border border-neutral-200 px-3 py-1.5 text-xs font-medium text-neutral-700 hover:bg-neutral-900/[0.06] dark:border-neutral-700 dark:text-neutral-300 dark:hover:bg-neutral-800"
                  >
                    Annuler
                  </button>
                  <button
                    type="submit"
                    disabled={isSubmitting}
                    className="rounded-lg bg-neutral-900 px-4 py-1.5 text-xs font-medium text-white hover:bg-black dark:bg-neutral-100 dark:text-neutral-900 dark:hover:bg-white disabled:opacity-50"
                  >
                    {isSubmitting ? 'Mise à jour...' : 'Mettre à jour'}
                  </button>
                </div>
              </div>
            </form>
          </div>
        </div>
      )}

      {/* MODAL DIALOGUE DE SUPPRESSION SÉCURISÉE (REQUÊTE DELETE VERS API GO / POSTGRESQL) */}
      {deletingItem && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/70 p-4 backdrop-blur-sm animate-in fade-in duration-150">
          <div className="relative w-full max-w-md overflow-hidden rounded-2xl border border-neutral-200 bg-white p-6 shadow-2xl dark:border-white/10 dark:bg-[#0e0e0e] shadow-2xl shadow-neutral-950/25 dark:shadow-black/80 ring-1 ring-black/5 dark:ring-white/10">
            {/* EN-TÊTE DU DIALOGUE */}
            <div className="flex items-start justify-between">
              <div className="flex items-center gap-3">
                <div className="flex h-10 w-10 shrink-0 items-center justify-center rounded-xl bg-rose-500/10 text-rose-600 dark:text-rose-400 border border-rose-500/20">
                  <AlertTriangle className="h-5 w-5" />
                </div>
                <div>
                  <h2 className="text-sm font-semibold text-neutral-900 dark:text-neutral-100">
                    Suppression Définitive de Prestation
                  </h2>
                  <p className="text-[11px] text-neutral-500 dark:text-neutral-400 font-mono">
                    DELETE /api/v1/services/{deletingItem.id}
                  </p>
                </div>
              </div>
              <button
                type="button"
                disabled={isDeleting}
                onClick={() => setDeletingItem(null)}
                className="rounded-lg p-1 text-neutral-400 hover:bg-neutral-950 hover:text-white dark:hover:text-neutral-200 transition-colors disabled:opacity-50 dark:hover:bg-neutral-100 dark:hover:text-neutral-950"
              >
                <X className="h-4 w-4" />
              </button>
            </div>

            {/* CARTE DÉTAILLÉE DE LA PRESTATION CIBLÉE */}
            <div className="mt-4 rounded-xl border border-neutral-200/80 bg-neutral-50/70 p-3.5 dark:border-neutral-800/80 dark:bg-neutral-900/50 hover:border-neutral-950 dark:hover:border-white/70 ps-elevated">
              <div className="flex items-center justify-between">
                <span className="inline-flex items-center rounded-md border border-neutral-200 bg-white px-2 py-0.5 text-[10px] font-medium text-neutral-700 dark:border-neutral-700 dark:bg-neutral-800 dark:text-neutral-300 hover:border-neutral-950 dark:hover:border-white/70 ps-elevated">
                  {deletingItem.category}
                </span>
                <span className="font-mono text-xs font-bold text-neutral-900 dark:text-neutral-100">
                  {deletingItem.price.toLocaleString('fr-FR')} FCFA / {deletingItem.unit}
                </span>
              </div>
              <h3 className="mt-2 text-sm font-bold text-neutral-900 dark:text-white">{deletingItem.name}</h3>
              {deletingItem.treatment && (
                <p className="text-xs text-neutral-500 dark:text-neutral-400">
                  Traitement : {deletingItem.treatment}
                </p>
              )}
            </div>

            {/* AVERTISSEMENTS & SÉCURITÉ COMPTABLE */}
            <div className="mt-4 space-y-2 text-xs">
              <div className="flex items-start gap-2 rounded-lg border border-emerald-200/60 bg-emerald-50/40 p-2.5 text-emerald-800 dark:border-emerald-900/40 dark:bg-emerald-950/20 dark:text-emerald-300">
                <CheckCircle2 className="h-4 w-4 shrink-0 text-emerald-600 dark:text-emerald-400 mt-0.5" />
                <p className="text-[11px] leading-relaxed">
                  <strong>Historique protégé :</strong> Les commandes et factures déjà émises conserveront le
                  libellé et le tarif historiques exacts enregistrés au moment du dépôt.
                </p>
              </div>

              <div className="flex items-start gap-2 rounded-lg border border-neutral-200/60 bg-neutral-50/50 p-2.5 text-neutral-600 dark:border-neutral-800 dark:bg-neutral-900/30 dark:text-neutral-400 hover:border-neutral-950 dark:hover:border-white/70 ps-elevated">
                <Info className="h-4 w-4 shrink-0 text-neutral-400 mt-0.5" />
                <p className="text-[11px] leading-relaxed">
                  Cette action supprimera l'article du catalogue actif via l'API Go. Pour suspendre
                  temporairement une prestation sans l'effacer, vous pouvez choisir de la{' '}
                  <strong>désactiver</strong>.
                </p>
              </div>
            </div>

            {/* BANNIÈRE D'ERREUR EN CAS D'ÉCHEC API */}
            {deleteError && (
              <div className="mt-3 flex items-start gap-2 rounded-lg border border-rose-300/80 bg-rose-50/80 p-2.5 text-rose-800 dark:border-rose-900/60 dark:bg-rose-950/30 dark:text-rose-300">
                <AlertCircle className="h-4 w-4 shrink-0 text-rose-600 mt-0.5" />
                <p className="text-[11px] leading-relaxed">
                  <strong>Échec de la suppression :</strong> {deleteError}
                </p>
              </div>
            )}

            {/* BOUTONS D'ACTION DU DIALOGUE */}
            <div className="mt-6 flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between border-t border-neutral-100 pt-4 dark:border-neutral-800">
              <button
                type="button"
                disabled={isDeleting}
                onClick={handleDeactivateInstead}
                className="rounded-lg border border-neutral-200 px-3 py-1.5 text-xs font-medium text-neutral-700 hover:bg-neutral-900/[0.06] dark:border-neutral-700 dark:text-neutral-300 dark:hover:bg-neutral-800 transition-colors disabled:opacity-50 text-center"
                title="Conserver dans la base mais retirer de la vue comptoir"
              >
                Désactiver plutôt
              </button>

              <div className="flex items-center justify-end gap-2">
                <button
                  type="button"
                  disabled={isDeleting}
                  onClick={() => setDeletingItem(null)}
                  className="rounded-lg border border-neutral-200 px-3 py-1.5 text-xs font-medium text-neutral-600 hover:bg-neutral-900/[0.06] dark:border-neutral-700 dark:text-neutral-300 dark:hover:bg-neutral-800 transition-colors disabled:opacity-50"
                >
                  Annuler
                </button>
                <button
                  type="button"
                  disabled={isDeleting}
                  onClick={handleConfirmDelete}
                  className="inline-flex items-center gap-1.5 rounded-lg bg-rose-600 px-3.5 py-1.5 text-xs font-medium text-white hover:bg-rose-700 dark:bg-rose-600 dark:hover:bg-rose-700 transition-colors disabled:opacity-50 shadow-xs"
                >
                  {isDeleting ? (
                    <>
                      <Loader2 className="h-3.5 w-3.5 animate-spin" />
                      <span>Suppression en cours...</span>
                    </>
                  ) : (
                    <>
                      <Trash2 className="h-3.5 w-3.5" />
                      <span>Supprimer définitivement</span>
                    </>
                  )}
                </button>
              </div>
            </div>
          </div>
        </div>
      )}

      {/* MODAL AJUSTEUR DE GRILLE TARIFAIRE EN MASSE */}
      {isBulkAdjustOpen && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 p-4 backdrop-blur-xs">
          <div className="w-full max-w-md rounded-xl border border-neutral-200 bg-white p-6 shadow-xl dark:border-neutral-800 dark:bg-[#111111] shadow-2xl shadow-neutral-950/25 dark:shadow-black/80 ring-1 ring-black/5 dark:ring-white/10">
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
                className="text-neutral-400 hover:text-neutral-950 dark:hover:text-neutral-200"
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
                  className="mt-1 w-full rounded-lg border border-neutral-200 px-3 py-1.5 text-xs text-neutral-900 dark:border-neutral-800 dark:bg-neutral-900 dark:text-neutral-100"
                >
                  <option value="all">Toutes les catégories ({services.length} prestations)</option>
                  <option value="Vêtements du quotidien">A. Vêtements du quotidien</option>
                  <option value="Vêtements traditionnels & cérémoniels">
                    B. Vêtements traditionnels & cérémoniels
                  </option>
                  <option value="Linge de maison">C. Linge de maison</option>
                  <option value="Services complémentaires & Options">
                    D. Services complémentaires & Options
                  </option>
                  <option value="Prestations spécialisées & Luxe">E. Prestations spécialisées & Luxe</option>
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
                          : 'border-neutral-200 hover:bg-neutral-900/[0.06] dark:border-neutral-700 dark:text-neutral-300 dark:hover:bg-neutral-800'
                      }`}
                    >
                      {pct > 0 ? `+${pct}%` : `${pct}%`}
                    </button>
                  ))}
                </div>
              </div>

              <div className="flex justify-end gap-2 border-t border-neutral-100 pt-3 dark:border-neutral-800">
                <button
                  type="button"
                  onClick={() => setIsBulkAdjustOpen(false)}
                  className="rounded-lg border border-neutral-200 px-3 py-1.5 text-xs font-medium text-neutral-700 hover:bg-neutral-900/[0.06] dark:border-neutral-700 dark:text-neutral-300 dark:hover:bg-neutral-800"
                >
                  Annuler
                </button>
                <button
                  type="button"
                  onClick={handleApplyBulkAdjust}
                  disabled={isSyncing}
                  className="rounded-lg bg-neutral-900 px-4 py-1.5 text-xs font-medium text-white hover:bg-black dark:bg-neutral-100 dark:text-neutral-900 dark:hover:bg-white disabled:opacity-50"
                >
                  {isSyncing ? 'Mise à jour...' : 'Appliquer & Synchroniser DB'}
                </button>
              </div>
            </div>
          </div>
        </div>
      )}

      {/* MODAL REÇU TARIFAIRE OFFICIEL COMPTOIR (FORMAT TICKET 80MM / 58MM POS) */}
      {isPrintModalOpen && (
        <CounterReceiptModal
          services={services}
          quoteItems={quoteItems}
          quoteTotal={quoteTotal}
          includeExpressInQuote={includeExpressInQuote}
          includePickupInQuote={includePickupInQuote}
          onClose={() => setIsPrintModalOpen(false)}
        />
      )}

      {/* MODULE DE GESTION DE CATALOGUE AVEC SYNCHRONISATION POSTGRESQL DIRECTE */}
      <CatalogManagerModal
        isOpen={isCatalogManagerOpen}
        onClose={() => setIsCatalogManagerOpen(false)}
        services={services}
        currentUser={currentUser}
        token={token}
        onAddService={onAddService}
        onUpdateService={onUpdateService}
        onDeleteService={onDeleteService}
        onSyncWithDB={onSyncWithDB}
        showToast={showToast}
      />
    </div>
  );
};
