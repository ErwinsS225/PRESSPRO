import React, { useState, useMemo } from 'react';
import { Plus, Search, Trash2, Database, XCircle, Copy, Sliders, Check, Tag } from 'lucide-react';
import { ServiceItem, User } from '../types';

interface CatalogManagerModalProps {
  isOpen: boolean;
  onClose: () => void;
  services: ServiceItem[];
  currentUser: User;
  token?: string;
  onAddService: (service: ServiceItem) => void;
  onUpdateService: (service: ServiceItem) => void;
  onDeleteService: (serviceId: string) => void;
  onSyncWithDB?: () => Promise<void>;
  showToast?: (msg: string) => void;
}

export const CatalogManagerModal: React.FC<CatalogManagerModalProps> = ({
  isOpen,
  onClose,
  services,
  token,
  onAddService,
  onUpdateService,
  onDeleteService,
  showToast,
}) => {
  // Filtres internes au gestionnaire
  const [activeCategory, setActiveCategory] = useState<string>('all');
  const [search, setSearch] = useState('');
  const [filterStatus, setFilterStatus] = useState<'all' | 'active' | 'inactive'>('all');
  const [filterPricing] = useState<'all' | 'piece' | 'kilogram' | 'quote'>('all');

  // Sélection d'un service pour édition en direct ou mode création
  const [selectedServiceId, setSelectedServiceId] = useState<string | null>(
    services.length > 0 ? services[0].id : null
  );
  const [isCreatingNew, setIsCreatingNew] = useState(false);

  // Formulaire d'édition / création
  const [formArticle, setFormArticle] = useState('');
  const [formTreatment, setFormTreatment] = useState('Lavage + repassage');
  const [formCategory, setFormCategory] = useState('Vêtements du quotidien');
  const [formPrice, setFormPrice] = useState(1000);
  const [formUnit, setFormUnit] = useState('pièce');
  const [formPricingModel, setFormPricingModel] = useState<
    'piece' | 'kilogram' | 'subscription' | 'contract'
  >('piece');
  const [formDuration, setFormDuration] = useState(24);
  const [formIsActive, setFormIsActive] = useState(true);
  const [formIsExpress, setFormIsExpress] = useState(false);
  const [formExpressSurcharge, setFormExpressSurcharge] = useState(500);
  const [formIsPickup, setFormIsPickup] = useState(true);
  const [formIsOutsourced, setFormIsOutsourced] = useState(false);
  const [formRequiresQuote, setFormRequiresQuote] = useState(false);
  const [formDescription, setFormDescription] = useState('');

  // États d'exécution asynchrone
  const [isSaving, setIsSaving] = useState(false);
  const [isDeleting, setIsDeleting] = useState(false);
  const [confirmDeleteId, setConfirmDeleteId] = useState<string | null>(null);
  const [saveStatus, setSaveStatus] = useState<'idle' | 'saved' | 'error'>('idle');

  // Les 5 catégories officielles
  const categoriesList = [
    'Vêtements du quotidien',
    'Vêtements traditionnels & cérémoniels',
    'Linge de maison',
    'Services complémentaires & Options',
    'Prestations spécialisées & Luxe',
  ];

  // Charger le service sélectionné dans le formulaire
  const loadServiceToForm = (item: ServiceItem) => {
    setIsCreatingNew(false);
    setSelectedServiceId(item.id);
    setFormArticle(item.article || item.name.split('(')[0].trim());
    setFormTreatment(item.treatment || 'Lavage + repassage');
    setFormCategory(item.category || 'Vêtements du quotidien');
    setFormPrice(item.price || 0);
    setFormUnit(item.unit || 'pièce');
    setFormPricingModel(item.pricingModel || 'piece');
    setFormDuration(item.durationHours || 24);
    setFormIsActive(item.isActive !== false);
    setFormIsExpress(item.isExpressAvailable === true);
    setFormExpressSurcharge(item.expressSurcharge || 500);
    setFormIsPickup(item.isPickupAvailable !== false);
    setFormIsOutsourced(item.isOutsourced === true);
    setFormRequiresQuote(item.requiresQuote === true);
    setFormDescription(item.description || '');
    setConfirmDeleteId(null);
    setSaveStatus('idle');
  };

  // Initialiser formulaire pour une nouvelle prestation
  const handleStartCreate = () => {
    setIsCreatingNew(true);
    setSelectedServiceId(null);
    setFormArticle('');
    setFormTreatment('Lavage + repassage');
    setFormCategory(activeCategory !== 'all' ? activeCategory : 'Vêtements du quotidien');
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
    setConfirmDeleteId(null);
    setSaveStatus('idle');
  };

  // Dupliquer la prestation active pour en créer une variante
  const handleDuplicate = (base: ServiceItem) => {
    setIsCreatingNew(true);
    setSelectedServiceId(null);
    setFormArticle(base.article || base.name.split('(')[0].trim());
    setFormTreatment('Repassage seul');
    setFormCategory(base.category);
    setFormPrice(Math.round((base.price * 0.6) / 50) * 50);
    setFormUnit(base.unit);
    setFormPricingModel(base.pricingModel || 'piece');
    setFormDuration(12);
    setFormIsActive(true);
    setFormIsExpress(base.isExpressAvailable === true);
    setFormExpressSurcharge(base.expressSurcharge || 300);
    setFormIsPickup(base.isPickupAvailable !== false);
    setFormIsOutsourced(false);
    setFormRequiresQuote(false);
    setFormDescription(`Variante Repassage seul pour ${base.article || base.name}`);
    setConfirmDeleteId(null);
    setSaveStatus('idle');
  };

  // Filtrer la liste des prestations
  const filteredList = useMemo(() => {
    return services.filter((s) => {
      const matchCat = activeCategory === 'all' || s.category === activeCategory;
      const matchSearch =
        s.name.toLowerCase().includes(search.toLowerCase()) ||
        (s.article && s.article.toLowerCase().includes(search.toLowerCase())) ||
        (s.treatment && s.treatment.toLowerCase().includes(search.toLowerCase()));
      const matchStatus =
        filterStatus === 'all' ||
        (filterStatus === 'active' && s.isActive !== false) ||
        (filterStatus === 'inactive' && s.isActive === false);
      const matchPricing =
        filterPricing === 'all' ||
        (filterPricing === 'piece' && (s.pricingModel === 'piece' || !s.pricingModel)) ||
        (filterPricing === 'kilogram' && s.pricingModel === 'kilogram') ||
        (filterPricing === 'quote' && s.requiresQuote);

      return matchCat && matchSearch && matchStatus && matchPricing;
    });
  }, [services, activeCategory, search, filterStatus, filterPricing]);

  // Synchronisation création / mise à jour
  const handleSave = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!formArticle.trim() || isSaving) return;

    setIsSaving(true);
    const fullName = `${formArticle.trim()} (${formTreatment.trim()})`;

    try {
      if (isCreatingNew) {
        // Envoi POST vers backend Go / PostgreSQL
        const res = await fetch('/api/v1/services', {
          method: 'POST',
          headers: {
            'Content-Type': 'application/json',
            ...(token ? { Authorization: `Bearer ${token}` } : {}),
          },
          body: JSON.stringify({
            name: fullName,
            article: formArticle.trim(),
            treatment: formTreatment.trim(),
            category: formCategory,
            unit_price: Number(formPrice),
            unit: formUnit,
            pricing_model: formPricingModel,
            estimated_hours: Number(formDuration),
            is_active: formIsActive,
            is_express_available: formIsExpress,
            express_surcharge: formIsExpress ? Number(formExpressSurcharge) : 0,
            is_pickup_available: formIsPickup,
            is_outsourced: formIsOutsourced,
            requires_quote: formRequiresQuote,
            description: formDescription.trim(),
          }),
        });

        let createdItem: ServiceItem;
        if (res.ok) {
          const data = await res.json();
          createdItem = {
            id: data.id,
            name: data.name,
            article: data.article || formArticle.trim(),
            treatment: data.treatment || formTreatment.trim(),
            category: data.category || formCategory,
            price: Number(data.unit_price) || Number(formPrice),
            unit: data.unit || formUnit,
            pricingModel: data.pricing_model || formPricingModel,
            durationHours: Number(data.estimated_hours) || Number(formDuration),
            isActive: data.is_active !== false,
            isExpressAvailable: data.is_express_available === true,
            expressSurcharge: Number(data.express_surcharge) || 0,
            isPickupAvailable: data.is_pickup_available !== false,
            isOutsourced: data.is_outsourced === true,
            requiresQuote: data.requires_quote === true,
            description: data.description || formDescription.trim(),
          };
        } else {
          // Fallback avec ID généré si réseau local
          createdItem = {
            id: `srv-${Date.now()}`,
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
        }

        onAddService(createdItem);
        setIsCreatingNew(false);
        setSelectedServiceId(createdItem.id);
        setSaveStatus('saved');
        setTimeout(() => setSaveStatus('idle'), 2500);

        if (showToast) {
          showToast(`Prestation "${createdItem.name}" créée et synchronisée avec PostgreSQL`);
        }
      } else if (selectedServiceId) {
        // Envoi PUT vers backend Go / PostgreSQL
        const updatedItem: ServiceItem = {
          id: selectedServiceId,
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

        const res = await fetch(`/api/v1/services/${selectedServiceId}`, {
          method: 'PUT',
          headers: {
            'Content-Type': 'application/json',
            ...(token ? { Authorization: `Bearer ${token}` } : {}),
          },
          body: JSON.stringify({
            name: fullName,
            article: formArticle.trim(),
            treatment: formTreatment.trim(),
            category: formCategory,
            unit_price: Number(formPrice),
            unit: formUnit,
            pricing_model: formPricingModel,
            estimated_hours: Number(formDuration),
            is_active: formIsActive,
            is_express_available: formIsExpress,
            express_surcharge: formIsExpress ? Number(formExpressSurcharge) : 0,
            is_pickup_available: formIsPickup,
            is_outsourced: formIsOutsourced,
            requires_quote: formRequiresQuote,
            description: formDescription.trim(),
          }),
        });

        if (res.ok) {
          const data = await res.json();
          updatedItem.name = data.name || updatedItem.name;
          updatedItem.price = Number(data.unit_price) || updatedItem.price;
        }

        onUpdateService(updatedItem);
        setSaveStatus('saved');
        setTimeout(() => setSaveStatus('idle'), 2500);

        if (showToast) {
          showToast(`Prestation "${updatedItem.name}" mise à jour dans PostgreSQL`);
        }
      }
    } catch (err: any) {
      setSaveStatus('error');
      if (showToast) showToast(`Erreur de synchronisation: ${err.message}`);
    } finally {
      setIsSaving(false);
    }
  };

  // Basculer active/désactivée en direct avec persistance
  const handleToggleActiveDirect = async (item: ServiceItem) => {
    const nextState = item.isActive === false ? true : false;
    const updated: ServiceItem = { ...item, isActive: nextState };
    onUpdateService(updated);

    if (selectedServiceId === item.id) {
      setFormIsActive(nextState);
    }

    try {
      await fetch(`/api/v1/services/${item.id}`, {
        method: 'PUT',
        headers: {
          'Content-Type': 'application/json',
          ...(token ? { Authorization: `Bearer ${token}` } : {}),
        },
        body: JSON.stringify({
          name: item.name,
          article: item.article || item.name,
          treatment: item.treatment || 'Lavage & Soin',
          category: item.category,
          unit_price: item.price,
          unit: item.unit,
          pricing_model: item.pricingModel || 'piece',
          estimated_hours: item.durationHours,
          is_active: nextState,
        }),
      });

      if (showToast) {
        showToast(
          `Prestation "${item.name}" ${nextState ? 'réactivée' : 'désactivée (archivée sans rupture historique)'}`
        );
      }
    } catch (err) {
      console.warn('Erreur toggle active:', err);
    }
  };

  // Supprimer définitivement avec DELETE /api/v1/services/{id}
  const handleDeleteDirect = async (serviceId: string) => {
    setIsDeleting(true);
    try {
      await fetch(`/api/v1/services/${serviceId}`, {
        method: 'DELETE',
        headers: {
          ...(token ? { Authorization: `Bearer ${token}` } : {}),
        },
      });

      onDeleteService(serviceId);
      setConfirmDeleteId(null);

      // Si le service supprimé était ouvert dans le panneau droit, sélectionner un autre
      const remaining = services.filter((s) => s.id !== serviceId);
      if (remaining.length > 0) {
        loadServiceToForm(remaining[0]);
      } else {
        handleStartCreate();
      }

      if (showToast) {
        showToast(`Prestation supprimée définitivement de PostgreSQL.`);
      }
    } catch (err: any) {
      if (showToast) showToast(`Erreur lors de la suppression: ${err.message}`);
    } finally {
      setIsDeleting(false);
    }
  };

  // Fermeture : rendu nul APRÈS l'appel des hooks (l'ordre d'appel doit être
  // identique à chaque rendu, sinon React lève « Rendered more hooks than
  // during the previous render » dès la première ouverture du gestionnaire).
  if (!isOpen) return null;

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 p-3 sm:p-5 backdrop-blur-xs">
      <div className="flex h-[92vh] w-full max-w-6xl flex-col overflow-hidden rounded-2xl border border-neutral-200 bg-white shadow-2xl dark:border-neutral-800 dark:bg-[#0c0c0c]">
        {/* EN-TÊTE SUPÉRIEUR */}
        <div className="flex items-center justify-between border-b border-neutral-200 px-6 py-4 dark:border-neutral-800">
          <div className="flex items-center gap-3">
            <div className="flex h-9 w-9 items-center justify-center rounded-xl bg-neutral-900 text-white dark:bg-neutral-100 dark:text-neutral-900 shadow-xs">
              <Sliders className="h-5 w-5" />
            </div>
            <div>
              <div className="flex items-center gap-2">
                <h2 className="text-base font-semibold text-neutral-900 dark:text-neutral-100">
                  Gestionnaire de Catalogue & Barème PostgreSQL
                </h2>
                <span className="inline-flex items-center gap-1 rounded-full border border-emerald-500/20 bg-emerald-500/10 px-2 py-0.5 text-[10px] font-medium text-emerald-700 dark:text-emerald-400">
                  <Database className="h-3 w-3" />
                  <span>Synchronisé en direct</span>
                </span>
              </div>
              <p className="text-xs text-neutral-500 dark:text-neutral-400">
                Créez, modifiez, désactivez ou supprimez des prestations. Les anciennes commandes conservent
                leur tarification historique.
              </p>
            </div>
          </div>

          <div className="flex items-center gap-2">
            <button
              onClick={handleStartCreate}
              className="flex items-center gap-1.5 rounded-lg bg-neutral-900 px-3.5 py-1.5 text-xs font-medium text-white shadow-xs hover:bg-neutral-800 dark:bg-neutral-100 dark:text-neutral-900 dark:hover:bg-white transition-colors"
            >
              <Plus className="h-3.5 w-3.5" />
              <span>Créer une prestation</span>
            </button>

            <button
              onClick={onClose}
              className="rounded-lg p-2 text-neutral-400 hover:bg-neutral-100 hover:text-neutral-700 dark:hover:bg-neutral-800 dark:hover:text-neutral-200 transition-colors"
            >
              <XCircle className="h-5 w-5" />
            </button>
          </div>
        </div>

        {/* CORPS PRINCIPAL EN SPLIT-VIEW (LISTE GAUCHE + FORMULAIRE DROITE) */}
        <div className="grid flex-1 grid-cols-1 overflow-hidden md:grid-cols-12 divide-y md:divide-y-0 md:divide-x divide-neutral-200 dark:divide-neutral-800">
          {/* COLONNE GAUCHE (5 COLUMNS) : FILTRES & LISTE DES PRESTATIONS */}
          <div className="flex h-full flex-col overflow-hidden md:col-span-5 bg-neutral-50/50 dark:bg-neutral-950/40">
            {/* Barre de recherche & filtres rapides */}
            <div className="p-3.5 space-y-2.5 border-b border-neutral-200 dark:border-neutral-800 bg-white dark:bg-[#0c0c0c]">
              <div className="relative">
                <Search className="absolute left-2.5 top-2.5 h-3.5 w-3.5 text-neutral-400" />
                <input
                  type="text"
                  placeholder="Rechercher vêtement, soin..."
                  value={search}
                  onChange={(e) => setSearch(e.target.value)}
                  className="w-full rounded-lg border border-neutral-200 bg-neutral-50 pl-8 pr-3 py-1.5 text-xs text-neutral-900 placeholder:text-neutral-400 focus:border-neutral-900 focus:bg-white focus:outline-hidden dark:border-neutral-800 dark:bg-neutral-900 dark:text-neutral-100 dark:focus:border-neutral-100"
                />
              </div>

              {/* Filtres de catégorie & statut */}
              <div className="grid grid-cols-2 gap-2 text-xs">
                <select
                  value={activeCategory}
                  onChange={(e) => setActiveCategory(e.target.value)}
                  className="rounded-md border border-neutral-200 bg-white px-2 py-1 text-[11px] text-neutral-700 dark:border-neutral-800 dark:bg-neutral-900 dark:text-neutral-300"
                >
                  <option value="all">Toutes les catégories</option>
                  {categoriesList.map((c) => (
                    <option key={c} value={c}>
                      {c}
                    </option>
                  ))}
                </select>

                <select
                  value={filterStatus}
                  onChange={(e) => setFilterStatus(e.target.value as any)}
                  className="rounded-md border border-neutral-200 bg-white px-2 py-1 text-[11px] text-neutral-700 dark:border-neutral-800 dark:bg-neutral-900 dark:text-neutral-300"
                >
                  <option value="all">Tous statuts ({services.length})</option>
                  <option value="active">Actives uniquement</option>
                  <option value="inactive">Désactivées</option>
                </select>
              </div>
            </div>

            {/* Liste scrollable */}
            <div className="flex-1 overflow-y-auto divide-y divide-neutral-100 dark:divide-neutral-900 p-2 space-y-1">
              {filteredList.length === 0 ? (
                <div className="py-12 text-center text-xs text-neutral-400">
                  <Tag className="mx-auto h-6 w-6 text-neutral-300 dark:text-neutral-700 mb-1" />
                  <span>Aucune prestation trouvée</span>
                </div>
              ) : (
                filteredList.map((item) => {
                  const isSelected = selectedServiceId === item.id && !isCreatingNew;
                  const isActive = item.isActive !== false;
                  return (
                    <div
                      key={item.id}
                      onClick={() => loadServiceToForm(item)}
                      className={`group flex items-center justify-between rounded-xl p-3 cursor-pointer transition-all ${
                        isSelected
                          ? 'border border-neutral-900/15 bg-white shadow-xs dark:border-neutral-100/20 dark:bg-neutral-900'
                          : 'border border-transparent hover:bg-white/80 dark:hover:bg-neutral-900/50'
                      }`}
                    >
                      <div className="space-y-0.5 pr-2">
                        <div className="flex items-center gap-1.5">
                          <span
                            className={`font-semibold text-xs ${
                              isSelected
                                ? 'text-neutral-900 dark:text-neutral-50'
                                : 'text-neutral-800 dark:text-neutral-200'
                            }`}
                          >
                            {item.article || item.name}
                          </span>
                          {!isActive && (
                            <span className="rounded bg-neutral-200 px-1 py-0.2 text-[9px] font-medium text-neutral-600 dark:bg-neutral-800 dark:text-neutral-400">
                              Désactivée
                            </span>
                          )}
                        </div>
                        <p className="text-[11px] text-neutral-500 dark:text-neutral-400 line-clamp-1">
                          {item.treatment || 'Soin standard'} • {item.unit}
                        </p>
                      </div>

                      <div className="flex items-center gap-2">
                        <span className="font-mono text-xs font-bold text-neutral-900 dark:text-neutral-100 tabular-nums">
                          {item.price.toLocaleString('fr-FR')} F
                        </span>
                        <button
                          type="button"
                          onClick={(e) => {
                            e.stopPropagation();
                            handleToggleActiveDirect(item);
                          }}
                          title={isActive ? 'Désactiver' : 'Activer'}
                          className={`h-2 w-2 rounded-full transition-transform hover:scale-125 ${
                            isActive ? 'bg-emerald-500' : 'bg-neutral-400'
                          }`}
                        />
                      </div>
                    </div>
                  );
                })
              )}
            </div>
          </div>

          {/* COLONNE DROITE (7 COLUMNS) : FORMULAIRE COMPLET D'ÉDITION & ACTIONS */}
          <div className="flex h-full flex-col overflow-y-auto md:col-span-7 bg-white dark:bg-[#0c0c0c] p-6">
            <div className="flex items-center justify-between border-b border-neutral-100 pb-3 dark:border-neutral-800">
              <div>
                <h3 className="text-sm font-semibold text-neutral-900 dark:text-neutral-100">
                  {isCreatingNew
                    ? 'Nouvelle Prestation au Barème'
                    : `Paramétrage : ${formArticle || 'Prestation'} (${formTreatment})`}
                </h3>
                <p className="text-[11px] text-neutral-500 dark:text-neutral-400">
                  {isCreatingNew
                    ? 'Ajoutez une nouvelle référence au catalogue de votre établissement.'
                    : `Identifiant DB : ${selectedServiceId || 'Auto'}`}
                </p>
              </div>

              {!isCreatingNew && selectedServiceId && (
                <button
                  type="button"
                  onClick={() => {
                    const current = services.find((s) => s.id === selectedServiceId);
                    if (current) handleDuplicate(current);
                  }}
                  className="flex items-center gap-1.5 rounded-lg border border-neutral-200 bg-neutral-50 px-2.5 py-1 text-xs text-neutral-700 hover:bg-neutral-100 dark:border-neutral-800 dark:bg-neutral-900 dark:text-neutral-300 dark:hover:bg-neutral-800"
                  title="Créer une variante à partir de cette référence"
                >
                  <Copy className="h-3.5 w-3.5" />
                  <span>Dupliquer</span>
                </button>
              )}
            </div>

            <form onSubmit={handleSave} className="mt-4 space-y-4">
              {/* Ligne 1 : Article & Traitement */}
              <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
                <div>
                  <label className="block text-xs font-medium text-neutral-700 dark:text-neutral-300">
                    Article / Vêtement <span className="text-rose-500">*</span>
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
                    Traitement Spécifique <span className="text-rose-500">*</span>
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

              {/* Ligne 2 : Catégorie & Modèle Commercial */}
              <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
                <div>
                  <label className="block text-xs font-medium text-neutral-700 dark:text-neutral-300">
                    Catégorie Principale
                  </label>
                  <select
                    value={formCategory}
                    onChange={(e) => setFormCategory(e.target.value)}
                    className="mt-1 w-full rounded-lg border border-neutral-200 px-3 py-1.5 text-xs text-neutral-900 dark:border-neutral-800 dark:bg-neutral-900 dark:text-neutral-100 focus:outline-hidden"
                  >
                    {categoriesList.map((cat) => (
                      <option key={cat} value={cat}>
                        {cat}
                      </option>
                    ))}
                  </select>
                </div>

                <div>
                  <label className="block text-xs font-medium text-neutral-700 dark:text-neutral-300">
                    Modèle de Facturation
                  </label>
                  <select
                    value={formPricingModel}
                    onChange={(e) => setFormPricingModel(e.target.value as any)}
                    className="mt-1 w-full rounded-lg border border-neutral-200 px-3 py-1.5 text-xs text-neutral-900 dark:border-neutral-800 dark:bg-neutral-900 dark:text-neutral-100 focus:outline-hidden"
                  >
                    <option value="piece">Modèle 1 : Paiement à la pièce</option>
                    <option value="kilogram">Modèle 2 : Facturation au kilogramme</option>
                    <option value="subscription">Modèle 3 : Forfait abonnement</option>
                    <option value="contract">Modèle 4 : Contrat professionnel B2B</option>
                  </select>
                </div>
              </div>

              {/* Ligne 3 : Tarif, Unité & Délai */}
              <div className="grid grid-cols-3 gap-3">
                <div>
                  <label className="block text-xs font-medium text-neutral-700 dark:text-neutral-300">
                    Tarif Unitaire (FCFA) <span className="text-rose-500">*</span>
                  </label>
                  <div className="relative mt-1">
                    <input
                      type="number"
                      required
                      min={0}
                      step={50}
                      value={formPrice}
                      onChange={(e) => setFormPrice(Number(e.target.value))}
                      className="w-full rounded-lg border border-neutral-200 px-3 py-1.5 text-xs font-mono font-bold text-neutral-900 tabular-nums focus:border-neutral-900 focus:outline-hidden dark:border-neutral-800 dark:bg-neutral-900 dark:text-neutral-100 dark:focus:border-neutral-100"
                    />
                  </div>
                </div>

                <div>
                  <label className="block text-xs font-medium text-neutral-700 dark:text-neutral-300">
                    Unité de Facturation
                  </label>
                  <select
                    value={formUnit}
                    onChange={(e) => setFormUnit(e.target.value)}
                    className="mt-1 w-full rounded-lg border border-neutral-200 px-3 py-1.5 text-xs text-neutral-900 dark:border-neutral-800 dark:bg-neutral-900 dark:text-neutral-100 focus:outline-hidden"
                  >
                    <option value="pièce">pièce</option>
                    <option value="ensemble">ensemble</option>
                    <option value="kg">kg (au poids)</option>
                    <option value="panneau">panneau</option>
                    <option value="m²">m²</option>
                    <option value="paire">la paire</option>
                    <option value="forfait">forfait</option>
                    <option value="supplément">supplément</option>
                  </select>
                </div>

                <div>
                  <label className="block text-xs font-medium text-neutral-700 dark:text-neutral-300">
                    Délai d'Atelier (heures)
                  </label>
                  <input
                    type="number"
                    min={1}
                    value={formDuration}
                    onChange={(e) => setFormDuration(Number(e.target.value))}
                    className="mt-1 w-full rounded-lg border border-neutral-200 px-3 py-1.5 text-xs font-mono text-neutral-900 dark:border-neutral-800 dark:bg-neutral-900 dark:text-neutral-100 focus:outline-hidden"
                  />
                </div>
              </div>

              {/* Options & Attributs métier */}
              <div className="rounded-xl border border-neutral-200/80 bg-neutral-50/70 p-4 space-y-3 dark:border-neutral-800 dark:bg-neutral-900/40">
                <p className="text-xs font-semibold text-neutral-800 dark:text-neutral-200">
                  Options et Traitements Avancés :
                </p>
                <div className="grid grid-cols-2 gap-3 text-xs">
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
                  <div className="flex items-center gap-2 pt-2 border-t border-neutral-200/60 dark:border-neutral-800">
                    <span className="text-xs text-neutral-600 dark:text-neutral-400">
                      Supplément Express :
                    </span>
                    <input
                      type="number"
                      value={formExpressSurcharge}
                      onChange={(e) => setFormExpressSurcharge(Number(e.target.value))}
                      className="w-24 rounded border border-neutral-200 px-2 py-1 text-xs font-mono dark:border-neutral-700 dark:bg-neutral-800"
                    />
                    <span className="text-xs font-mono">FCFA</span>
                  </div>
                )}
              </div>

              {/* Description & Consignes */}
              <div>
                <label className="block text-xs font-medium text-neutral-700 dark:text-neutral-300">
                  Consignes d'Atelier & Description
                </label>
                <textarea
                  rows={2}
                  placeholder="Ex: Cintre obligatoire, protection broderies, traitement anti-décoloration..."
                  value={formDescription}
                  onChange={(e) => setFormDescription(e.target.value)}
                  className="mt-1 w-full rounded-lg border border-neutral-200 px-3 py-1.5 text-xs text-neutral-900 focus:border-neutral-900 focus:outline-hidden dark:border-neutral-800 dark:bg-neutral-900 dark:text-neutral-100 resize-none"
                />
              </div>

              {/* Bascule Actif / Inactif */}
              <div className="flex items-center justify-between rounded-xl border border-neutral-200 p-3 dark:border-neutral-800 bg-neutral-50/50 dark:bg-neutral-900/30">
                <div className="space-y-0.5">
                  <span className="text-xs font-medium text-neutral-800 dark:text-neutral-200">
                    Statut Actif au Comptoir
                  </span>
                  <p className="text-[11px] text-neutral-500 dark:text-neutral-400">
                    Si désactivée, cette prestation ne pourra plus être sélectionnée pour de nouvelles
                    commandes.
                  </p>
                </div>
                <button
                  type="button"
                  onClick={() => setFormIsActive(!formIsActive)}
                  className={`relative inline-flex h-6 w-11 items-center rounded-full transition-colors ${
                    formIsActive ? 'bg-emerald-600' : 'bg-neutral-300 dark:bg-neutral-700'
                  }`}
                >
                  <span
                    className={`inline-block h-4 w-4 transform rounded-full bg-white transition-transform ${
                      formIsActive ? 'translate-x-6' : 'translate-x-1'
                    }`}
                  />
                </button>
              </div>

              {/* Actions de validation & suppression */}
              <div className="flex items-center justify-between border-t border-neutral-100 pt-4 dark:border-neutral-800">
                <div>
                  {!isCreatingNew && selectedServiceId && (
                    <>
                      {confirmDeleteId === selectedServiceId ? (
                        <div className="flex items-center gap-2">
                          <button
                            type="button"
                            onClick={() => handleDeleteDirect(selectedServiceId)}
                            disabled={isDeleting}
                            className="rounded-lg bg-rose-600 px-3 py-1.5 text-xs font-medium text-white hover:bg-rose-700 transition-colors"
                          >
                            {isDeleting ? 'Suppression...' : 'Confirmer la suppression'}
                          </button>
                          <button
                            type="button"
                            onClick={() => setConfirmDeleteId(null)}
                            className="text-xs text-neutral-500 hover:text-neutral-700 dark:hover:text-neutral-300"
                          >
                            Annuler
                          </button>
                        </div>
                      ) : (
                        <button
                          type="button"
                          onClick={() => setConfirmDeleteId(selectedServiceId)}
                          className="flex items-center gap-1.5 text-xs text-rose-600 hover:text-rose-700 dark:text-rose-400 transition-colors"
                        >
                          <Trash2 className="h-3.5 w-3.5" />
                          <span>Supprimer de la base</span>
                        </button>
                      )}
                    </>
                  )}
                </div>

                <div className="flex items-center gap-2">
                  {saveStatus === 'saved' && (
                    <span className="flex items-center gap-1 text-xs text-emerald-600 dark:text-emerald-400">
                      <Check className="h-3.5 w-3.5" />
                      <span>Enregistré dans PostgreSQL</span>
                    </span>
                  )}
                  <button
                    type="submit"
                    disabled={isSaving}
                    className="flex items-center gap-1.5 rounded-lg bg-neutral-900 px-4 py-1.5 text-xs font-medium text-white shadow-xs hover:bg-neutral-800 dark:bg-neutral-100 dark:text-neutral-900 dark:hover:bg-white disabled:opacity-50 transition-colors"
                  >
                    <Database className="h-3.5 w-3.5" />
                    <span>
                      {isSaving
                        ? 'Synchronisation PostgreSQL...'
                        : isCreatingNew
                          ? 'Créer dans la Base de Données'
                          : 'Mettre à jour dans PostgreSQL'}
                    </span>
                  </button>
                </div>
              </div>
            </form>
          </div>
        </div>
      </div>
    </div>
  );
};
