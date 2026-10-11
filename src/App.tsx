/**
 * @license
 * SPDX-License-Identifier: Apache-2.0
 */

import React, { useState, useEffect } from 'react';
import { motion, AnimatePresence } from 'motion/react';
import { ViewMode, Order, Customer, ServiceItem, HealthCheckResult, AuthSession } from './types';
import { canAccessSuperadmin } from './lib/permissions';
import { CURRENT_TENANT, DEMO_ORDERS, DEMO_CUSTOMERS, DEMO_SERVICES } from './lib/demoData';
import { checkEndpoint } from './lib/api';
import {
  fetchOrders,
  createOrderOnBackend,
  updateOrderStatusOnBackend,
  addOrderPaymentOnBackend,
} from './lib/ordersApi';
import { fetchCustomers, createCustomerOnBackend } from './lib/customersApi';
import { ThemeProvider } from './context/ThemeContext';
import { Sidebar } from './components/Sidebar';
import { Header } from './components/Header';
import { DashboardView } from './components/DashboardView';
import { OrdersView } from './components/OrdersView';
import { CustomersView } from './components/CustomersView';
import { ServicesView } from './components/ServicesView';
import { BillingView } from './components/BillingView';
import { SuperAdminView } from './components/SuperAdminView';
import { TeamView } from './components/TeamView';
import { BranchesView } from './components/BranchesView';
import { ExpensesView } from './components/ExpensesView';
import { ReportsView } from './components/ReportsView';
import { NewOrderModal } from './components/NewOrderModal';
import { LoginModal } from './components/LoginModal';
import { ThermalTicketModal } from './components/ThermalTicketModal';
import { TicketVerificationView } from './components/TicketVerificationView';

function PressingOSApp() {
  const [currentView, setCurrentView] = useState<ViewMode>('dashboard');
  const [verificationParams, setVerificationParams] = useState<{ tenantId: string; orderId: string } | null>(
    null
  );
  const [session, setSession] = useState<AuthSession>({
    token: 'jwt-default-session-token',
    user: {
      id: 'usr-gerant-001',
      tenant_id: CURRENT_TENANT.id,
      email: 'gerant@pressing-ivoire.ci',
      full_name: 'M. Bamba Seydou',
      role: 'owner',
      status: 'active',
    },
    tenant: CURRENT_TENANT,
  });

  // Contrôle d'accès à l'Espace Éditeur SaaS (Super Administration Globale).
  const isSuperadmin = canAccessSuperadmin(session.user);

  const [orders, setOrders] = useState<Order[]>(DEMO_ORDERS);
  const [customers, setCustomers] = useState<Customer[]>(DEMO_CUSTOMERS);
  const [services, setServices] = useState<ServiceItem[]>(DEMO_SERVICES);
  const [isMobileMenuOpen, setIsMobileMenuOpen] = useState(false);
  const [isNewOrderModalOpen, setIsNewOrderModalOpen] = useState(false);
  const [isLoginModalOpen, setIsLoginModalOpen] = useState(false);
  const [selectedTicketOrder, setSelectedTicketOrder] = useState<Order | null>(null);
  const [health, setHealth] = useState<HealthCheckResult | null>(null);
  const [isCheckingHealth, setIsCheckingHealth] = useState(false);
  const [toastMessage, setToastMessage] = useState<string | null>(null);

  const showToast = (msg: string) => {
    setToastMessage(msg);
    setTimeout(() => {
      setToastMessage((prev) => (prev === msg ? null : prev));
    }, 3200);
  };

  const runHealthProbe = async () => {
    setIsCheckingHealth(true);
    try {
      const res = await checkEndpoint('/health');
      setHealth(res);
      if (res.status === 'ok') {
        showToast('API Go active sur :8080 (Liveness vérifié)');
      }
    } catch {
      // silencieux
    } finally {
      setIsCheckingHealth(false);
    }
  };

  useEffect(() => {
    // Parser l'URL pour détecter une page de vérification de ticket
    const path = window.location.pathname;
    const verifyMatch = path.match(/\/verify\/([^/]+)\/([^/]+)/);
    if (verifyMatch) {
      setVerificationParams({
        tenantId: verifyMatch[1],
        orderId: verifyMatch[2],
      });
      return; // Ne pas charger l'application normale si c'est une page de vérification
    }

    runHealthProbe();

    // Authentification automatique du gérant pour obtenir un jeton JWT valide
    async function initSessionAndServices() {
      try {
        const res = await fetch('/api/v1/auth/login', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ email: 'gerant@pressing-ivoire.ci', password: 'pass123' }),
        });
        if (res.ok) {
          const data = await res.json();
          const validToken = data.token;
          setSession({
            token: validToken,
            user: data.user,
            tenant: data.tenant || CURRENT_TENANT,
          });

          // Récupération des prestations réelles depuis le backend Go
          const srvRes = await fetch('/api/v1/services', {
            headers: {
              Authorization: `Bearer ${validToken}`,
            },
          });
          if (srvRes.ok) {
            const srvData = await srvRes.json();
            if (Array.isArray(srvData) && srvData.length > 0) {
              setServices(
                srvData.map((d: any) => ({
                  id: d.id,
                  name: d.name,
                  article: d.article || d.name,
                  treatment: d.treatment || 'Lavage & Soin',
                  category: d.category || 'Vêtements du quotidien',
                  price: Number(d.unit_price) || 0,
                  unit: d.unit || 'pièce',
                  pricingModel: d.pricing_model || 'piece',
                  durationHours: Number(d.estimated_hours) || 24,
                  isActive: d.is_active !== false,
                  isExpressAvailable: d.is_express_available === true,
                  expressSurcharge: Number(d.express_surcharge) || 0,
                  isPickupAvailable: d.is_pickup_available !== false,
                  isOutsourced: d.is_outsourced === true,
                  requiresQuote: d.requires_quote === true,
                  description: d.description || '',
                }))
              );
            }
          }

          // Charger commandes et clients depuis PostgreSQL
          const [ordRes, custRes] = await Promise.all([fetchOrders(validToken), fetchCustomers(validToken)]);
          if (ordRes.fromBackend && ordRes.orders.length > 0) setOrders(ordRes.orders);
          if (custRes.fromBackend && custRes.customers.length > 0) setCustomers(custRes.customers);
        }
      } catch (err) {
        console.warn('Initialisation session backend:', err);
      }
    }

    initSessionAndServices();
  }, []);

  const handleAdvanceOrderStatus = (orderId: string) => {
    const ord = orders.find((o) => o.id === orderId);
    if (!ord) return;

    let nextStatus: Order['status'] = ord.status;
    if (ord.status === 'received') nextStatus = 'washing';
    else if (ord.status === 'washing') nextStatus = 'ironing';
    else if (ord.status === 'ironing') {
      nextStatus = 'ready';
      showToast(
        `Linge prêt ! Notification SMS / WhatsApp envoyée à ${ord.customerName} (${ord.customerPhone})`
      );
    } else if (ord.status === 'ready') {
      nextStatus = 'delivered';
      showToast(`Linge remis au client (${ord.orderNumber})`);
    } else {
      return; // Déjà au dernier statut
    }

    // Persistance best-effort (les IDs démo non-UUID restent locaux)
    if (/^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i.test(orderId)) {
      updateOrderStatusOnBackend(orderId, nextStatus, session.token);
    }

    setOrders((prev) => prev.map((o) => (o.id === orderId ? { ...o, status: nextStatus } : o)));
  };

  const handleAddOrder = async (newOrder: Order) => {
    const { order: persisted, synced, error } = await createOrderOnBackend(newOrder, session.token);
    const finalOrder = synced && persisted ? persisted : newOrder;
    setOrders([finalOrder, ...orders]);
    if (synced && persisted) {
      showToast(`Dépôt ${persisted.orderNumber} enregistré dans PostgreSQL`);
    } else {
      showToast(
        `Dépôt ${newOrder.orderNumber} enregistré localement (non synchronisé${error ? ` : ${error}` : ''})`
      );
    }
    // Proposer l'impression immédiate du ticket thermique
    setSelectedTicketOrder(finalOrder);
  };

  const handleAddCustomer = async (newCustomer: Customer) => {
    const { customer: persisted, synced, error } = await createCustomerOnBackend(newCustomer, session.token);
    const finalCustomer = synced && persisted ? persisted : newCustomer;
    setCustomers([finalCustomer, ...customers]);
    if (synced && persisted) {
      showToast(`Client ${finalCustomer.fullName} enregistré dans PostgreSQL`);
    } else {
      showToast(
        `Client ${newCustomer.fullName} ajouté${error ? ` — ${error}` : ' localement (non synchronisé)'}`
      );
    }
  };

  const handleAddService = (newService: ServiceItem) => {
    setServices([...services, newService]);
    showToast(`Prestation "${newService.name}" ajoutée (${newService.price.toLocaleString('fr-FR')} FCFA)`);
  };

  const handleUpdateService = (updatedService: ServiceItem) => {
    setServices((prev) => prev.map((s) => (s.id === updatedService.id ? updatedService : s)));
    showToast(
      `Tarif mis à jour : ${updatedService.name} (${updatedService.price.toLocaleString('fr-FR')} FCFA)`
    );
  };

  const handleDeleteService = (serviceId: string) => {
    const item = services.find((s) => s.id === serviceId);
    setServices((prev) => prev.filter((s) => s.id !== serviceId));
    showToast(`Prestation "${item?.name || 'Sélectionnée'}" supprimée du catalogue`);
  };

  const handleSyncServicesWithDB = async () => {
    try {
      const res = await fetch('/api/v1/services', {
        headers: {
          Authorization: `Bearer ${session.token}`,
        },
      });
      if (res.ok) {
        const data = await res.json();
        if (Array.isArray(data) && data.length > 0) {
          const mapped: ServiceItem[] = data.map((d: any) => ({
            id: d.id,
            name: d.name,
            article: d.article || d.name,
            treatment: d.treatment || 'Lavage & Soin',
            category: d.category || 'Vêtements du quotidien',
            price: Number(d.unit_price) || 0,
            unit: d.unit || 'pièce',
            pricingModel: d.pricing_model || 'piece',
            durationHours: Number(d.estimated_hours) || 24,
            isActive: d.is_active !== false,
            isExpressAvailable: d.is_express_available === true,
            expressSurcharge: Number(d.express_surcharge) || 0,
            isPickupAvailable: d.is_pickup_available !== false,
            isOutsourced: d.is_outsourced === true,
            requiresQuote: d.requires_quote === true,
            description: d.description || '',
          }));
          setServices(mapped);
          showToast(`Catalogue synchronisé (${mapped.length} prestations depuis PostgreSQL)`);
          return;
        }
      }
    } catch (err) {
      console.warn('Erreur sync catalogue:', err);
    }
    showToast('Grille tarifaire synchronisée avec PostgreSQL');
  };

  const handleSettleBalance = (orderId: string, amount: number, method: string) => {
    // Persistance best-effort du paiement (les IDs démo non-UUID restent locaux)
    if (/^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i.test(orderId)) {
      addOrderPaymentOnBackend(orderId, amount, method, session.token).then((ok) => {
        if (!ok) console.warn('[App] Échec enregistrement du paiement côté serveur');
      });
    }

    setOrders((prev) =>
      prev.map((ord) => {
        if (ord.id !== orderId) return ord;
        const newPaid = ord.paidAmount + amount;
        return {
          ...ord,
          paidAmount: newPaid,
          paymentStatus: 'paid',
        };
      })
    );
    showToast(
      `Solde de ${amount.toLocaleString('fr-FR')} FCFA encaissé avec succès via ${method.toUpperCase()}`
    );
    // Mettre à jour la commande ouverte si active
    if (selectedTicketOrder && selectedTicketOrder.id === orderId) {
      setSelectedTicketOrder({
        ...selectedTicketOrder,
        paidAmount: selectedTicketOrder.paidAmount + amount,
        paymentStatus: 'paid',
      });
    }
  };

  const handleLoginSuccess = (newSession: AuthSession) => {
    setSession(newSession);
    if (newSession.user.role === 'superadmin') {
      setCurrentView('superadmin');
      showToast('Connecté en SuperAdmin SaaS (Administration Globale)');
    } else {
      setCurrentView('dashboard');
      showToast(`Connecté en tant que ${newSession.user.full_name} (${newSession.user.role})`);
    }
  };

  // Si on est sur une page de vérification, on affiche seulement cette vue
  if (verificationParams) {
    return (
      <TicketVerificationView tenantId={verificationParams.tenantId} orderId={verificationParams.orderId} />
    );
  }

  return (
    <div className="flex min-h-screen bg-neutral-50/40 text-neutral-900 font-sans antialiased selection:bg-neutral-900 selection:text-white dark:bg-[#050505] dark:text-neutral-100 dark:selection:bg-white dark:selection:text-black transition-colors duration-150">
      {/* Toast Notification */}
      <AnimatePresence>
        {toastMessage && (
          <motion.div
            initial={{ opacity: 0, y: 15 }}
            animate={{ opacity: 1, y: 0 }}
            exit={{ opacity: 0, y: 15 }}
            transition={{ duration: 0.18 }}
            className="fixed bottom-5 right-5 z-50 flex items-center gap-2 rounded-lg bg-neutral-900 px-4 py-2.5 text-xs font-medium text-white shadow-lg border border-neutral-800 dark:bg-neutral-100 dark:text-neutral-900 dark:border-neutral-200"
          >
            <span>{toastMessage}</span>
          </motion.div>
        )}
      </AnimatePresence>

      {/* Sidebar Navigation */}
      <Sidebar
        currentView={currentView}
        onSelectView={setCurrentView}
        tenant={session.tenant}
        currentUser={session.user}
        onOpenLoginModal={() => setIsLoginModalOpen(true)}
        health={health}
        isOpenMobile={isMobileMenuOpen}
        onCloseMobile={() => setIsMobileMenuOpen(false)}
      />

      {/* Main Content Area */}
      <div className="flex flex-1 flex-col min-w-0">
        <Header
          currentView={currentView}
          tenant={session.tenant}
          currentUser={session.user}
          onOpenLoginModal={() => setIsLoginModalOpen(true)}
          health={health}
          isChecking={isCheckingHealth}
          onRefreshHealth={runHealthProbe}
          onOpenMobileMenu={() => setIsMobileMenuOpen(true)}
          onNewOrderClick={() => setIsNewOrderModalOpen(true)}
        />

        <main className="flex-1 p-4 md:p-8 max-w-7xl w-full mx-auto">
          <AnimatePresence mode="wait">
            <motion.div
              key={currentView}
              initial={{ opacity: 0, y: 6 }}
              animate={{ opacity: 1, y: 0 }}
              exit={{ opacity: 0, y: -6 }}
              transition={{ duration: 0.15, ease: 'easeOut' }}
            >
              {currentView === 'dashboard' && (
                <DashboardView
                  orders={orders}
                  health={health}
                  onNavigate={setCurrentView}
                  onNewOrder={() => setIsNewOrderModalOpen(true)}
                />
              )}

              {currentView === 'orders' && (
                <OrdersView
                  orders={orders}
                  onAdvanceStatus={handleAdvanceOrderStatus}
                  onNewOrder={() => setIsNewOrderModalOpen(true)}
                  onOpenThermalTicket={(ord) => setSelectedTicketOrder(ord)}
                />
              )}

              {currentView === 'customers' && (
                <CustomersView customers={customers} onAddCustomer={handleAddCustomer} />
              )}

              {currentView === 'services' && (
                <ServicesView
                  services={services}
                  currentUser={session.user}
                  token={session.token}
                  onAddService={handleAddService}
                  onUpdateService={handleUpdateService}
                  onDeleteService={handleDeleteService}
                  onSyncWithDB={handleSyncServicesWithDB}
                  showToast={showToast}
                />
              )}

              {currentView === 'team' && (
                <TeamView currentUser={session.user} token={session.token} showToast={showToast} />
              )}

              {currentView === 'branches' && (
                <BranchesView currentUser={session.user} token={session.token} showToast={showToast} />
              )}

              {currentView === 'expenses' && (
                <ExpensesView currentUser={session.user} token={session.token} showToast={showToast} />
              )}

              {currentView === 'reports' && <ReportsView currentUser={session.user} token={session.token} />}

              {currentView === 'billing' && <BillingView token={session.token} showToast={showToast} />}

              {currentView === 'superadmin' && isSuperadmin && (
                <SuperAdminView
                  currentUser={session.user}
                  token={session.token}
                  showToast={showToast}
                  onSelectTenantView={() => {
                    setCurrentView('dashboard');
                    showToast('Bascule sur la vue du pressing');
                  }}
                />
              )}
            </motion.div>
          </AnimatePresence>
        </main>
      </div>

      {/* Modal Nouveau Dépôt */}
      {isNewOrderModalOpen && (
        <NewOrderModal
          customers={customers}
          services={services}
          onClose={() => setIsNewOrderModalOpen(false)}
          onSubmit={handleAddOrder}
        />
      )}

      {/* Modal Changement de Compte / Connexion */}
      {isLoginModalOpen && (
        <LoginModal
          currentSession={session}
          onClose={() => setIsLoginModalOpen(false)}
          onLoginSuccess={handleLoginSuccess}
        />
      )}

      {/* Modal Ticket Thermique (80mm) */}
      {selectedTicketOrder && session.tenant && (
        <ThermalTicketModal
          order={selectedTicketOrder}
          tenant={session.tenant}
          onClose={() => setSelectedTicketOrder(null)}
          onSettleBalance={handleSettleBalance}
        />
      )}
    </div>
  );
}

export default function App() {
  return (
    <ThemeProvider>
      <PressingOSApp />
    </ThemeProvider>
  );
}
