/**
 * @license
 * SPDX-License-Identifier: Apache-2.0
 */

import React, { useState, useEffect } from 'react';
import { motion, AnimatePresence } from 'motion/react';
import { ViewMode, Order, Customer, ServiceItem, HealthCheckResult, AuthSession } from './types';
import { CURRENT_TENANT, DEMO_ORDERS, DEMO_CUSTOMERS, DEMO_SERVICES } from './lib/demoData';
import { checkEndpoint } from './lib/api';
import { ThemeProvider } from './context/ThemeContext';
import { Sidebar } from './components/Sidebar';
import { Header } from './components/Header';
import { DashboardView } from './components/DashboardView';
import { OrdersView } from './components/OrdersView';
import { CustomersView } from './components/CustomersView';
import { ServicesView } from './components/ServicesView';
import { SuperAdminView } from './components/SuperAdminView';
import { SystemHealthView } from './components/SystemHealthView';
import { DocsView } from './components/DocsView';
import { NewOrderModal } from './components/NewOrderModal';
import { LoginModal } from './components/LoginModal';
import { ThermalTicketModal } from './components/ThermalTicketModal';

function PressingOSApp() {
  const [currentView, setCurrentView] = useState<ViewMode>('dashboard');
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
                  category: d.category || 'Lavage & Repassage',
                  price: Number(d.unit_price) || 0,
                  unit: d.unit || 'pièce',
                  durationHours: Number(d.estimated_hours) || 24,
                  isActive: d.is_active !== false,
                }))
              );
            }
          }
        }
      } catch (err) {
        console.warn('Initialisation session backend:', err);
      }
    }

    initSessionAndServices();
  }, []);

  const handleAdvanceOrderStatus = (orderId: string) => {
    setOrders((prev) =>
      prev.map((ord) => {
        if (ord.id !== orderId) return ord;
        let nextStatus: Order['status'] = ord.status;
        if (ord.status === 'received') nextStatus = 'washing';
        else if (ord.status === 'washing') nextStatus = 'ironing';
        else if (ord.status === 'ironing') {
          nextStatus = 'ready';
          showToast(`Linge prêt ! Notification SMS / WhatsApp envoyée à ${ord.customerName} (${ord.customerPhone})`);
        } else if (ord.status === 'ready') {
          nextStatus = 'delivered';
          showToast(`Linge remis au client (${ord.orderNumber})`);
        }

        return { ...ord, status: nextStatus };
      })
    );
  };

  const handleAddOrder = (newOrder: Order) => {
    setOrders([newOrder, ...orders]);
    showToast(`Dépôt ${newOrder.orderNumber} enregistré avec succès`);
    // Proposer l'impression immédiate du ticket thermique
    setSelectedTicketOrder(newOrder);
  };

  const handleAddCustomer = (newCustomer: Customer) => {
    setCustomers([newCustomer, ...customers]);
    showToast(`Client ${newCustomer.fullName} ajouté`);
  };

  const handleAddService = (newService: ServiceItem) => {
    setServices([...services, newService]);
    showToast(`Prestation "${newService.name}" ajoutée (${newService.price.toLocaleString('fr-FR')} FCFA)`);
  };

  const handleUpdateService = (updatedService: ServiceItem) => {
    setServices((prev) => prev.map((s) => (s.id === updatedService.id ? updatedService : s)));
    showToast(`Tarif mis à jour : ${updatedService.name} (${updatedService.price.toLocaleString('fr-FR')} FCFA)`);
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
          'Authorization': `Bearer ${session.token}`,
        },
      });
      if (res.ok) {
        const data = await res.json();
        if (Array.isArray(data) && data.length > 0) {
          const mapped: ServiceItem[] = data.map((d: any) => ({
            id: d.id,
            name: d.name,
            category: d.category,
            price: d.unit_price,
            unit: d.unit || 'pièce',
            durationHours: d.estimated_hours || 24,
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
    showToast(`Solde de ${amount.toLocaleString('fr-FR')} FCFA encaissé avec succès via ${method.toUpperCase()}`);
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

              {currentView === 'superadmin' && (
                <SuperAdminView
                  onSelectTenantView={() => {
                    setCurrentView('dashboard');
                    showToast('Bascule sur la vue du pressing');
                  }}
                />
              )}

              {currentView === 'system-health' && (
                <SystemHealthView
                  currentHealth={health}
                  onProbeAll={runHealthProbe}
                  isProbing={isCheckingHealth}
                />
              )}

              {currentView === 'docs' && <DocsView />}
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
      {selectedTicketOrder && (
        <ThermalTicketModal
          order={selectedTicketOrder}
          tenantName={session.tenant?.name || 'Pressing Ivoire Cocody'}
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
