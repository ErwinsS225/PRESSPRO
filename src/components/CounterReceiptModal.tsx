import React, { useState, useMemo } from 'react';
import {
  Printer,
  X,
  Receipt,
  Copy,
  Check,
  QrCode,
  Sparkles,
  Layers,
  ShoppingBag,
  FileText,
  CheckCircle2,
} from 'lucide-react';
import { ServiceItem } from '../types';

export type ReceiptFormat = '80mm' | '58mm';
export type ReceiptContentMode = 'essentials' | 'all' | 'category' | 'quote';

interface CounterReceiptModalProps {
  services: ServiceItem[];
  onClose: () => void;
  initialFormat?: ReceiptFormat;
  quoteItems?: { [serviceId: string]: number };
  quoteTotal?: number;
  includeExpressInQuote?: boolean;
  includePickupInQuote?: boolean;
  tenantName?: string;
  tenantAddress?: string;
  tenantPhone?: string;
}

export const CounterReceiptModal: React.FC<CounterReceiptModalProps> = ({
  services,
  onClose,
  initialFormat = '80mm',
  quoteItems = {},
  quoteTotal = 0,
  includeExpressInQuote = false,
  includePickupInQuote = false,
  tenantName = 'PRESSING IVOIRE COCODY',
  tenantAddress = 'Cocody Angré 8ème Tranche, Abidjan',
  tenantPhone = '+225 07 00 11 22 33 / +225 05 12 34 56 78',
}) => {
  // Format du reçu : 80mm (Supermarché / Pharmacie) par défaut
  const [format, setFormat] = useState<ReceiptFormat>(initialFormat);

  const hasQuote = Object.keys(quoteItems).length > 0;
  const [contentMode, setContentMode] = useState<ReceiptContentMode>(hasQuote ? 'quote' : 'essentials');
  const [selectedCategory, setSelectedCategory] = useState<string>('Vêtements du quotidien');
  const [showLegalNotice, setShowLegalNotice] = useState(true);
  const [showBarcode, setShowBarcode] = useState(true);
  const [copied, setCopied] = useState(false);

  // Date et heure actuelles de Côte d'Ivoire
  const now = new Date();
  const dateFormatted = now.toLocaleDateString('fr-FR', {
    day: '2-digit',
    month: '2-digit',
    year: 'numeric',
  });
  const timeFormatted = now.toLocaleTimeString('fr-FR', {
    hour: '2-digit',
    minute: '2-digit',
  });

  // Top 15 prestations essentielles les plus demandées à Abidjan (comme en pharmacie/supermarché)
  const essentialKeywords = [
    'chemise',
    'pantalon',
    'costume',
    'robe simple',
    'robe de soirée',
    't-shirt',
    'boubou brodé',
    'bazin',
    'drap',
    'couette',
    'repassage',
    'lavage & pliage',
    'rideau',
    'veste',
    'baskets',
  ];

  const filteredServices = useMemo(() => {
    const activeList = services.filter((s) => s.isActive !== false);

    if (contentMode === 'quote' && hasQuote) {
      return activeList.filter((s) => quoteItems[s.id] && quoteItems[s.id] > 0);
    }

    if (contentMode === 'category') {
      return activeList.filter((s) => s.category === selectedCategory);
    }

    if (contentMode === 'essentials') {
      // Prioriser les articles fréquents
      const essentials = activeList.filter((s) => {
        const lower = s.name.toLowerCase();
        return essentialKeywords.some((kw) => lower.includes(kw));
      });
      return essentials.length > 0 ? essentials.slice(0, 16) : activeList.slice(0, 16);
    }

    // Mode "all"
    return activeList;
  }, [services, contentMode, selectedCategory, quoteItems, hasQuote]);

  // Liste des catégories distinctes disponibles
  const categoriesList = useMemo(() => {
    const set = new Set<string>();
    services.forEach((s) => {
      if (s.category) set.add(s.category);
    });
    return Array.from(set);
  }, [services]);

  // Déclencher l'impression thermique
  const handlePrint = () => {
    window.print();
  };

  // Copier le reçu au format texte clair (pour envoi WhatsApp au client)
  const handleCopyText = () => {
    let text = `================================\n`;
    text += `       ${tenantName}\n`;
    text += `    Blanchisserie & Soin Textile\n`;
    text += `  ${tenantAddress}\n`;
    text += `  Tél: ${tenantPhone}\n`;
    text += `================================\n`;
    text += `EXTRAIT DU BARÈME COMPTOIR (FCFA)\n`;
    text += `Date : ${dateFormatted} ${timeFormatted}\n`;
    text += `--------------------------------\n`;

    if (contentMode === 'quote') {
      text += `ESTIMATION DEVIS CLIENT :\n`;
      filteredServices.forEach((item) => {
        const qty = quoteItems[item.id] || 1;
        const subtotal = item.price * qty;
        text += `${qty}x ${item.name}\n   -> ${subtotal.toLocaleString('fr-FR')} FCFA\n`;
      });
      if (includeExpressInQuote) text += `Option Express 24h incluse (+50%)\n`;
      if (includePickupInQuote) text += `Forfait Collecte / Livraison (+1 500 FCFA)\n`;
      text += `--------------------------------\n`;
      text += `TOTAL ESTIMÉ : ${quoteTotal.toLocaleString('fr-FR')} FCFA TTC\n`;
    } else {
      filteredServices.forEach((item) => {
        text += `${item.name.padEnd(24, ' ')} : ${item.price.toLocaleString('fr-FR')} F/${item.unit}\n`;
      });
    }

    text += `--------------------------------\n`;
    text += `Tarifs TTC • Conforme réglementation CI\n`;
    text += `Réclamations sous 48h avec ticket original\n`;
    text += `================================\n`;

    navigator.clipboard.writeText(text);
    setCopied(true);
    setTimeout(() => setCopied(false), 2500);
  };

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/70 p-3 backdrop-blur-xs">
      <div className="flex flex-col w-full max-w-4xl max-h-[95vh] rounded-2xl border border-neutral-200 bg-white shadow-2xl dark:border-neutral-800 dark:bg-[#111111] overflow-hidden">
        {/* EN-TÊTE DE LA MODALE */}
        <div className="flex items-center justify-between border-b border-neutral-100 px-6 py-4 dark:border-neutral-800 bg-neutral-50/50 dark:bg-[#141414]">
          <div className="flex items-center gap-3">
            <div className="flex h-9 w-9 items-center justify-center rounded-xl bg-neutral-900 text-white shadow-xs dark:bg-neutral-100 dark:text-neutral-900">
              <Receipt className="h-5 w-5" />
            </div>
            <div>
              <div className="flex items-center gap-2">
                <h2 className="text-sm font-bold text-neutral-900 dark:text-neutral-100">
                  Reçu Tarifaire de Comptoir
                </h2>
                <span className="rounded-full bg-emerald-100 px-2 py-0.5 font-mono text-[10px] font-semibold text-emerald-800 dark:bg-emerald-950/60 dark:text-emerald-300">
                  Format Rouleau POS
                </span>
              </div>
              <p className="text-[11px] text-neutral-500 dark:text-neutral-400">
                Format ticket de caisse thermique (pharmacie, boulangerie & supermarché) conforme à
                l'affichage commercial ivoirien.
              </p>
            </div>
          </div>

          <div className="flex items-center gap-2">
            <button
              onClick={handleCopyText}
              className="hidden sm:flex items-center gap-1.5 rounded-lg border border-neutral-200 bg-white px-3 py-1.5 text-xs font-medium text-neutral-700 hover:bg-neutral-900/[0.06] dark:hover:bg-white/[0.06] dark:border-neutral-700 dark:bg-neutral-800 dark:text-neutral-200 hover:border-neutral-950 dark:hover:border-white/70 ps-elevated"
              title="Copier le texte du ticket pour WhatsApp"
            >
              {copied ? <Check className="h-3.5 w-3.5 text-emerald-600" /> : <Copy className="h-3.5 w-3.5" />}
              <span>{copied ? 'Copié !' : 'Copier texte (WhatsApp)'}</span>
            </button>

            <button
              onClick={handlePrint}
              className="flex items-center gap-1.5 rounded-lg bg-neutral-900 px-4 py-1.5 text-xs font-semibold text-white shadow-xs hover:bg-black dark:bg-neutral-100 dark:text-neutral-900 dark:hover:bg-white transition-colors"
            >
              <Printer className="h-3.5 w-3.5" />
              <span>Imprimer Reçu ({format})</span>
            </button>

            <button
              onClick={onClose}
              className="rounded-lg p-1.5 text-neutral-400 hover:bg-neutral-950 hover:text-white dark:hover:text-neutral-200 dark:hover:bg-neutral-100 dark:hover:text-neutral-950"
            >
              <X className="h-4 w-4" />
            </button>
          </div>
        </div>

        {/* CORPS : PANNEAU DE CONFIGURATION + APERÇU ROULEAU THERMIQUE */}
        <div className="flex flex-col md:flex-row flex-1 overflow-hidden">
          {/* COLONNE GAUCHE : OPTIONS DE FORMAT ET DE CONTENU */}
          <div className="w-full md:w-80 shrink-0 border-b md:border-b-0 md:border-r border-neutral-100 p-5 dark:border-neutral-800 space-y-5 overflow-y-auto bg-neutral-50/30 dark:bg-[#121212]">
            {/* 1. Sélecteur de format de papier */}
            <div className="space-y-2">
              <label className="block text-xs font-semibold text-neutral-900 dark:text-neutral-100">
                Largeur du Rouleau Thermique
              </label>
              <div className="grid grid-cols-2 gap-2">
                <button
                  type="button"
                  onClick={() => setFormat('80mm')}
                  className={`flex flex-col items-center justify-center rounded-xl border p-2.5 text-center transition-all ${
                    format === '80mm'
                      ? 'border-neutral-900 bg-white font-bold text-neutral-900 shadow-xs dark:border-neutral-100 dark:bg-neutral-800 dark:text-neutral-100 ring-1 ring-neutral-900 dark:ring-neutral-100'
                      : 'border-neutral-200 bg-white/50 text-neutral-600 hover:border-neutral-950 hover:bg-neutral-950 hover:text-white dark:border-neutral-800 dark:bg-neutral-900/50 dark:text-neutral-400 dark:hover:border-neutral-100 dark:hover:bg-neutral-100 dark:hover:text-neutral-950'
                  }`}
                >
                  <span className="font-mono text-xs font-bold">80 mm</span>
                  <span className="text-[10px] text-neutral-500 dark:text-neutral-400 mt-0.5">
                    Pharmacie & Supermarché
                  </span>
                  <span className="mt-1 text-[9px] font-semibold text-emerald-600 dark:text-emerald-400">
                    Standard Recommandé
                  </span>
                </button>

                <button
                  type="button"
                  onClick={() => setFormat('58mm')}
                  className={`flex flex-col items-center justify-center rounded-xl border p-2.5 text-center transition-all ${
                    format === '58mm'
                      ? 'border-neutral-900 bg-white font-bold text-neutral-900 shadow-xs dark:border-neutral-100 dark:bg-neutral-800 dark:text-neutral-100 ring-1 ring-neutral-900 dark:ring-neutral-100'
                      : 'border-neutral-200 bg-white/50 text-neutral-600 hover:border-neutral-950 hover:bg-neutral-950 hover:text-white dark:border-neutral-800 dark:bg-neutral-900/50 dark:text-neutral-400 dark:hover:border-neutral-100 dark:hover:bg-neutral-100 dark:hover:text-neutral-950'
                  }`}
                >
                  <span className="font-mono text-xs font-bold">58 mm</span>
                  <span className="text-[10px] text-neutral-500 dark:text-neutral-400 mt-0.5">
                    Boulangerie & Mini TPE
                  </span>
                  <span className="mt-1 text-[9px] text-neutral-400">Compact Caisse</span>
                </button>
              </div>
            </div>

            {/* 2. Mode de contenu à imprimer */}
            <div className="space-y-2">
              <label className="block text-xs font-semibold text-neutral-900 dark:text-neutral-100">
                Contenu du Ticket
              </label>
              <div className="space-y-1.5">
                <button
                  type="button"
                  onClick={() => setContentMode('essentials')}
                  className={`flex w-full items-center justify-between rounded-lg border px-3 py-2 text-left text-xs transition-colors ${
                    contentMode === 'essentials'
                      ? 'border-neutral-900 bg-white font-semibold text-neutral-900 dark:border-neutral-100 dark:bg-neutral-800 dark:text-neutral-100'
                      : 'border-neutral-200 bg-white/50 text-neutral-600 hover:border-neutral-950 hover:bg-neutral-950 hover:text-white dark:border-neutral-800 dark:bg-neutral-900/40 dark:text-neutral-400 dark:hover:border-neutral-100 dark:hover:bg-neutral-100 dark:hover:text-neutral-950'
                  }`}
                >
                  <div className="flex items-center gap-2">
                    <Sparkles className="h-3.5 w-3.5 text-amber-500" />
                    <span>Top 15 Tarifs Essentiels</span>
                  </div>
                  <span className="text-[10px] font-mono text-neutral-400">1 ticket court</span>
                </button>

                <button
                  type="button"
                  onClick={() => setContentMode('category')}
                  className={`flex w-full items-center justify-between rounded-lg border px-3 py-2 text-left text-xs transition-colors ${
                    contentMode === 'category'
                      ? 'border-neutral-900 bg-white font-semibold text-neutral-900 dark:border-neutral-100 dark:bg-neutral-800 dark:text-neutral-100'
                      : 'border-neutral-200 bg-white/50 text-neutral-600 hover:border-neutral-950 hover:bg-neutral-950 hover:text-white dark:border-neutral-800 dark:bg-neutral-900/40 dark:text-neutral-400 dark:hover:border-neutral-100 dark:hover:bg-neutral-100 dark:hover:text-neutral-950'
                  }`}
                >
                  <div className="flex items-center gap-2">
                    <Layers className="h-3.5 w-3.5 text-blue-500" />
                    <span>Par Catégorie Ciblée</span>
                  </div>
                  <span className="text-[10px] font-mono text-neutral-400">Sélectif</span>
                </button>

                {contentMode === 'category' && (
                  <div className="pl-4 pt-1">
                    <select
                      value={selectedCategory}
                      onChange={(e) => setSelectedCategory(e.target.value)}
                      className="w-full rounded-lg border border-neutral-200 bg-white px-2.5 py-1.5 text-xs text-neutral-900 dark:border-neutral-700 dark:bg-neutral-800 dark:text-neutral-100 hover:border-neutral-950 dark:hover:border-white/70 ps-elevated"
                    >
                      {categoriesList.map((c) => (
                        <option key={c} value={c}>
                          {c}
                        </option>
                      ))}
                    </select>
                  </div>
                )}

                <button
                  type="button"
                  onClick={() => setContentMode('all')}
                  className={`flex w-full items-center justify-between rounded-lg border px-3 py-2 text-left text-xs transition-colors ${
                    contentMode === 'all'
                      ? 'border-neutral-900 bg-white font-semibold text-neutral-900 dark:border-neutral-100 dark:bg-neutral-800 dark:text-neutral-100'
                      : 'border-neutral-200 bg-white/50 text-neutral-600 hover:border-neutral-950 hover:bg-neutral-950 hover:text-white dark:border-neutral-800 dark:bg-neutral-900/40 dark:text-neutral-400 dark:hover:border-neutral-100 dark:hover:bg-neutral-100 dark:hover:text-neutral-950'
                  }`}
                >
                  <div className="flex items-center gap-2">
                    <FileText className="h-3.5 w-3.5 text-neutral-500" />
                    <span>Catalogue Intégral ({services.length})</span>
                  </div>
                  <span className="text-[10px] font-mono text-neutral-400">Complet</span>
                </button>

                {hasQuote && (
                  <button
                    type="button"
                    onClick={() => setContentMode('quote')}
                    className={`flex w-full items-center justify-between rounded-lg border px-3 py-2 text-left text-xs transition-colors ${
                      contentMode === 'quote'
                        ? 'border-emerald-600 bg-emerald-50/80 font-semibold text-emerald-900 dark:border-emerald-400 dark:bg-emerald-950/60 dark:text-emerald-100'
                        : 'border-neutral-200 bg-white/50 text-neutral-600 hover:border-neutral-950 hover:bg-neutral-950 hover:text-white dark:border-neutral-800 dark:bg-neutral-900/40 dark:text-neutral-400 dark:hover:border-neutral-100 dark:hover:bg-neutral-100 dark:hover:text-neutral-950'
                    }`}
                  >
                    <div className="flex items-center gap-2">
                      <ShoppingBag className="h-3.5 w-3.5 text-emerald-600" />
                      <span>Ticket Devis Client Actif</span>
                    </div>
                    <span className="text-[10px] font-mono font-bold text-emerald-700 dark:text-emerald-300">
                      {quoteTotal.toLocaleString('fr-FR')} F
                    </span>
                  </button>
                )}
              </div>
            </div>

            {/* 3. Options de conformité et affichage */}
            <div className="space-y-2 border-t border-neutral-200/80 pt-4 dark:border-neutral-800">
              <label className="block text-xs font-semibold text-neutral-900 dark:text-neutral-100">
                Options d'Affichage du Reçu
              </label>
              <div className="space-y-2 text-xs">
                <label className="flex items-center gap-2 cursor-pointer">
                  <input
                    type="checkbox"
                    checked={showLegalNotice}
                    onChange={(e) => setShowLegalNotice(e.target.checked)}
                    className="rounded border-neutral-300 text-neutral-900 focus:ring-0"
                  />
                  <span className="text-neutral-700 dark:text-neutral-300">
                    Mentions réglementaires CI & délais
                  </span>
                </label>

                <label className="flex items-center gap-2 cursor-pointer">
                  <input
                    type="checkbox"
                    checked={showBarcode}
                    onChange={(e) => setShowBarcode(e.target.checked)}
                    className="rounded border-neutral-300 text-neutral-900 focus:ring-0"
                  />
                  <span className="text-neutral-700 dark:text-neutral-300">
                    Code-barres & QR code de conformité
                  </span>
                </label>
              </div>
            </div>

            {/* Renseignements établissement */}
            <div className="rounded-xl border border-neutral-200/80 bg-white p-3 dark:border-neutral-800 dark:bg-neutral-900 text-[11px] space-y-1 hover:border-neutral-950 dark:hover:border-white/70 ps-elevated">
              <div className="font-semibold text-neutral-900 dark:text-neutral-100 flex items-center gap-1.5">
                <CheckCircle2 className="h-3 w-3 text-emerald-600" />
                <span>Réglementation Commerciale</span>
              </div>
              <p className="text-neutral-500 dark:text-neutral-400 leading-relaxed text-[10px]">
                En Côte d'Ivoire, l'affichage obligatoire des prix et tarifs peut être assuré au comptoir par
                voie de barème imprimé et mis à disposition immédiate du consommateur.
              </p>
            </div>
          </div>

          {/* COLONNE DROITE : APERÇU HAUTE FIDÉLITÉ DU ROULEAU DE CAISSE */}
          <div className="flex-1 overflow-y-auto p-6 flex flex-col items-center justify-start bg-neutral-200/60 dark:bg-[#0a0a0a]">
            <div className="text-center mb-3">
              <span className="text-[11px] font-mono text-neutral-500 dark:text-neutral-400">
                Aperçu fidèle du rouleau thermique • Prêt pour imprimante EPSON / Star / Xprinter
              </span>
            </div>

            {/* LE ROULEAU THERMIQUE IMPRIMABLE */}
            <div
              id="printable-receipt-container"
              className={`relative bg-white text-neutral-950 font-mono shadow-2xl transition-all duration-200 ${
                format === '80mm'
                  ? 'w-full max-w-[340px] receipt-width-80mm text-[11px] p-5 border border-neutral-300 rounded-sm'
                  : 'w-full max-w-[260px] receipt-width-58mm text-[10px] p-3.5 border border-neutral-300 rounded-sm'
              }`}
            >
              {/* DÉCOUPE DENTELÉE SUPÉRIEURE (DENTS DE SCIE ROULEAU THERMIQUE) */}
              <div className="absolute -top-1.5 left-0 right-0 h-2 bg-radial-[circle_at_10px_-4px] from-transparent to-white bg-[length:14px_10px] bg-repeat-x" />

              {/* EN-TÊTE DU TICKET */}
              <div className="text-center space-y-0.5">
                <div className="text-neutral-400 text-[10px] select-none tracking-widest">
                  {format === '58mm' ? '=====================' : '==============================='}
                </div>
                <h3 className="font-extrabold text-xs sm:text-sm uppercase tracking-wider text-black">
                  {tenantName}
                </h3>
                <div className="text-[10px] font-sans text-neutral-600">
                  Blanchisserie & Soin Textile Haute Précision
                </div>
                <div className="text-[9px] text-neutral-600">{tenantAddress}</div>
                <div className="text-[9px] text-neutral-600 font-sans">{tenantPhone}</div>
                <div className="text-[8px] text-neutral-500">RCCM : CI-ABJ-2024-B-10492 | CC : 2314980 R</div>
                <div className="text-neutral-400 text-[10px] select-none tracking-widest pt-0.5">
                  {format === '58mm' ? '---------------------' : '-------------------------------'}
                </div>
              </div>

              {/* MÉTADONNÉES DU REÇU */}
              <div className="mt-1 space-y-0.5 text-[9px] text-neutral-600">
                <div className="flex justify-between">
                  <span>DATE : {dateFormatted}</span>
                  <span>HEURE : {timeFormatted}</span>
                </div>
                <div className="flex justify-between">
                  <span>CAISSE : N° 01 (COMPTOIR)</span>
                  <span className="font-bold text-neutral-900">DEVISE : XOF (FCFA)</span>
                </div>
                <div className="text-center font-bold text-neutral-900 pt-1 text-[10px] uppercase">
                  {contentMode === 'quote'
                    ? '*** DEVIS ESTIMATION CLIENT ***'
                    : contentMode === 'essentials'
                      ? '*** EXTRAIT DU BARÈME COMPTOIR ***'
                      : contentMode === 'category'
                        ? `*** BARÈME : ${selectedCategory.toUpperCase()} ***`
                        : '*** BARÈME OFFICIEL GÉNÉRAL ***'}
                </div>
                <div className="text-neutral-400 text-[10px] select-none tracking-widest">
                  {format === '58mm' ? '---------------------' : '-------------------------------'}
                </div>
              </div>

              {/* CORPS DU TICKET : LISTE DES PRESTATIONS */}
              {contentMode === 'quote' ? (
                /* VUE TICKET DEVIS DU CLIENT */
                <div className="my-2 space-y-2">
                  <div className="flex justify-between font-bold text-[10px] border-b border-neutral-300 pb-1">
                    <span>ARTICLE / QTÉ</span>
                    <span>TOTAL TTC</span>
                  </div>
                  {filteredServices.map((item) => {
                    const qty = quoteItems[item.id] || 1;
                    const subtotal = item.price * qty;
                    return (
                      <div key={item.id} className="space-y-0.5">
                        <div className="flex justify-between items-baseline">
                          <span className="font-semibold text-neutral-900 truncate pr-1">
                            {qty}x {item.name}
                          </span>
                          <span className="font-bold tabular-nums whitespace-nowrap">
                            {subtotal.toLocaleString('fr-FR')} F
                          </span>
                        </div>
                        <div className="flex justify-between text-[9px] text-neutral-500 pl-2">
                          <span>
                            Tarif unitaire : {item.price.toLocaleString('fr-FR')} F / {item.unit}
                          </span>
                          <span>Délai : {item.durationHours}h</span>
                        </div>
                      </div>
                    );
                  })}

                  {includeExpressInQuote && (
                    <div className="flex justify-between text-[10px] pt-1 text-amber-900 font-medium">
                      <span>Traitement Express 24h (+50%)</span>
                      <span className="font-bold tabular-nums">
                        +
                        {Math.round((quoteTotal - (includePickupInQuote ? 1500 : 0)) / 3).toLocaleString(
                          'fr-FR'
                        )}{' '}
                        F
                      </span>
                    </div>
                  )}

                  {includePickupInQuote && (
                    <div className="flex justify-between text-[10px] text-blue-900 font-medium">
                      <span>Collecte & Livraison Abidjan</span>
                      <span className="font-bold tabular-nums">+1 500 F</span>
                    </div>
                  )}

                  <div className="text-neutral-400 text-[10px] select-none tracking-widest pt-1">
                    {format === '58mm' ? '=====================' : '==============================='}
                  </div>

                  <div className="space-y-1 pt-1">
                    <div className="flex justify-between text-xs font-black">
                      <span>TOTAL ESTIMÉ TTC :</span>
                      <span className="tabular-nums text-sm font-extrabold">
                        {quoteTotal.toLocaleString('fr-FR')} FCFA
                      </span>
                    </div>
                    <div className="flex justify-between text-[9px] text-neutral-600">
                      <span>Acompte recommandé (50%) :</span>
                      <span className="tabular-nums font-semibold">
                        {Math.round(quoteTotal * 0.5).toLocaleString('fr-FR')} FCFA
                      </span>
                    </div>
                    <div className="flex justify-between text-[9px] text-neutral-600">
                      <span>Solde au retrait :</span>
                      <span className="tabular-nums font-semibold">
                        {Math.round(quoteTotal * 0.5).toLocaleString('fr-FR')} FCFA
                      </span>
                    </div>
                  </div>
                </div>
              ) : (
                /* VUE GRILLE TARIFAIRE DE COMPTOIR */
                <div className="my-2 space-y-1">
                  <div className="flex justify-between font-bold text-[9px] text-neutral-500 border-b border-neutral-300 pb-0.5">
                    <span>PRESTATION</span>
                    <span>PRIX TTC</span>
                  </div>

                  <div className="divide-y divide-dashed divide-neutral-200">
                    {filteredServices.map((item) => (
                      <div key={item.id} className="py-1 flex justify-between items-baseline gap-1">
                        <div className="min-w-0 pr-1">
                          <div className="font-bold text-neutral-900 leading-tight">{item.name}</div>
                          <div className="text-[9px] text-neutral-500 flex items-center gap-1.5">
                            <span>{item.unit}</span>
                            <span>•</span>
                            <span>{item.category}</span>
                            {item.isExpressAvailable && (
                              <>
                                <span>•</span>
                                <span className="text-emerald-700 font-medium">Express dispo</span>
                              </>
                            )}
                          </div>
                        </div>
                        <div className="font-mono font-black tabular-nums text-right shrink-0">
                          {item.price.toLocaleString('fr-FR')} F
                        </div>
                      </div>
                    ))}
                  </div>

                  <div className="text-neutral-400 text-[10px] select-none tracking-widest pt-2">
                    {format === '58mm' ? '---------------------' : '-------------------------------'}
                  </div>

                  {/* RÉSUMÉ DU BARÈME */}
                  <div className="py-1 text-[9px] text-neutral-600 space-y-0.5">
                    <div className="flex justify-between">
                      <span>Prestations répertoriées :</span>
                      <span className="font-bold">{filteredServices.length}</span>
                    </div>
                    <div className="flex justify-between">
                      <span>Tarif moyen calculé :</span>
                      <span className="font-bold">
                        {Math.round(
                          filteredServices.reduce((acc, s) => acc + s.price, 0) /
                            (filteredServices.length || 1)
                        ).toLocaleString('fr-FR')}{' '}
                        FCFA
                      </span>
                    </div>
                    <div className="flex justify-between">
                      <span>Régime fiscal :</span>
                      <span>TVA 18% incluse (TTC)</span>
                    </div>
                  </div>
                </div>
              )}

              {/* SÉPARATEUR DOUBLE */}
              <div className="text-neutral-400 text-[10px] select-none tracking-widest">
                {format === '58mm' ? '=====================' : '==============================='}
              </div>

              {/* CODE-BARRES ET QR CODE */}
              {showBarcode && (
                <div className="my-2.5 flex flex-col items-center justify-center text-center space-y-1">
                  {/* Simulation code-barres thermique haute fidélité */}
                  <div className="h-7 w-44 bg-[repeating-linear-gradient(90deg,#000,#000_2px,transparent_2px,transparent_4px,#000_4px,#000_7px,transparent_7px,transparent_9px)]" />
                  <span className="font-mono text-[8px] tracking-widest text-neutral-700">
                    * CI-TARIF-{dateFormatted.replace(/\//g, '')}-01 *
                  </span>
                  <div className="flex items-center gap-1.5 text-[8px] text-neutral-500 font-sans">
                    <QrCode className="h-3 w-3 text-neutral-800" />
                    <span>Scannez pour consulter la grille en ligne</span>
                  </div>
                </div>
              )}

              {/* MENTIONS LÉGALES ET DÉLAIS */}
              {showLegalNotice && (
                <div className="space-y-1 text-center text-[8px] text-neutral-600 leading-tight border-t border-neutral-200 pt-2 font-sans">
                  <p className="font-bold text-neutral-800 uppercase">
                    Conditions Générales de Dépôt & Retrait
                  </p>
                  <p>1. Réclamations recevables sous 48h sur présentation du reçu original.</p>
                  <p>2. Vêtements non réclamés conservés pendant 90 jours conformément à la loi.</p>
                  <p>3. Supplément express 24h (+50%) • Forfait collecte Abidjan : 1 500 FCFA.</p>
                </div>
              )}

              {/* PIED DE REÇU : REMERCIEMENT */}
              <div className="mt-2 text-center text-[9px] font-bold tracking-wider text-black pt-1">
                *** MERCI DE VOTRE VISITE ***
              </div>
              <div className="text-center text-[8px] text-neutral-400 pb-1">
                PressingOS CI • Système de Gestion Agréé
              </div>

              {/* DÉCOUPE DENTELÉE INFÉRIEURE */}
              <div className="absolute -bottom-1.5 left-0 right-0 h-2 bg-radial-[circle_at_10px_14px] from-transparent to-white bg-[length:14px_10px] bg-repeat-x" />
            </div>
          </div>
        </div>

        {/* PIED DE LA MODALE */}
        <div className="flex items-center justify-between border-t border-neutral-100 px-6 py-3 dark:border-neutral-800 bg-neutral-50/50 dark:bg-[#141414]">
          <div className="flex items-center gap-2 text-xs text-neutral-500 dark:text-neutral-400">
            <span className="h-2 w-2 rounded-full bg-emerald-500 animate-pulse" />
            <span>
              Format :{' '}
              {format === '80mm'
                ? 'Reçu 80 mm (Pharmacie / Supermarché)'
                : 'Reçu 58 mm (Boulangerie / Mini TPE)'}
            </span>
            <span>•</span>
            <span>{filteredServices.length} prestation(s) sur le ticket</span>
          </div>

          <div className="flex items-center gap-2">
            <button
              onClick={onClose}
              className="rounded-lg border border-neutral-200 px-3.5 py-1.5 text-xs font-medium text-neutral-700 hover:bg-neutral-950 hover:text-white dark:border-neutral-700 dark:text-neutral-300 dark:hover:bg-neutral-100 dark:hover:text-neutral-950"
            >
              Fermer
            </button>
            <button
              onClick={handlePrint}
              className="flex items-center gap-1.5 rounded-lg bg-neutral-900 px-4 py-1.5 text-xs font-semibold text-white hover:bg-black dark:bg-neutral-100 dark:text-neutral-900 dark:hover:bg-white shadow-xs"
            >
              <Printer className="h-3.5 w-3.5" />
              <span>Imprimer le Reçu ({format})</span>
            </button>
          </div>
        </div>
      </div>
    </div>
  );
};
