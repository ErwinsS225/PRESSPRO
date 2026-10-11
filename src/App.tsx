/**
 * @license
 * SPDX-License-Identifier: Apache-2.0
 */

import React, { useState, useEffect } from 'react';
import { motion, AnimatePresence } from 'motion/react';
import { ViewMode, Order, Customer, ServiceItem, HealthCheckResult, AuthSession } from './types';
import { CURRENT_TENANT, DEMO_ORDERS, DEMO_CUSTOMERS, DEMO_SERVICES } from './lib/demoData';
import { checkEndpoint } from './lib/api';
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

export default function App() {
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
  const [health, setHealth] = useState<HealthCheckResult | null>(null);
  const [isCheckingHealth, setIsCheckingHealth] = useState(false);
  const [toastMessage, setToastMessage] = useState<string | null>(null);

  const showToast = (msg: string) => {
    setToastMessage(msg);
    setTimeout(() => {
      setToastMessage((prev) => (prev === msg ? null : prev));
    }, 3200);
  };

  // Sonde initiale de santé de l'API Go au montage
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
  }, []);

  const handleAdvanceOrderStatus = (orderId: string) => {
    setOrders((prev) =>
      prev.map((ord) => {
        if (ord.id !== orderId) return ord;
        let nextStatus: Order['status'] = ord.status;
        if (ord.status === 'received') nextStatus = 'washing';
        else if (ord.status === 'washing') nextStatus = 'ironing';
        else if (ord.status === 'ironing') nextStatus = 'ready';
        else if (ord.status === 'ready') nextStatus = 'delivered';

        showToast(`Ticket ${ord.orderNumber} mis à jour : ${nextStatus}`);
        return { ...ord, status: nextStatus };
      })
    );
  };

  const handleAddOrder = (newOrder: Order) => {
    setOrders([newOrder, ...orders]);
    showToast(`Dépôt ${newOrder.orderNumber} enregistré avec succès`);
  };

  const handleAddCustomer = (newCustomer: Customer) => {
    setCustomers([newCustomer, ...customers]);
    showToast(`Client ${newCustomer.fullName} ajouté`);
  };

  const handleAddService = (newService: ServiceItem) => {
    setServices([...services, newService]);
    showToast(`Prestation ${newService.name} ajoutée`);
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
    <div className="flex min-h-screen bg-neutral-50/40 text-neutral-900 font-sans antialiased selection:bg-neutral-900 selection:text-white">
      {/* Toast Notification */}
      <AnimatePresence>
        {toastMessage && (
          <motion.div
            initial={{ opacity: 0, y: 15 }}
            animate={{ opacity: 1, y: 0 }}
            exit={{ opacity: 0, y: 15 }}
            transition={{ duration: 0.18 }}
            className="fixed bottom-5 right-5 z-50 flex items-center gap-2 rounded-lg bg-neutral-900 px-4 py-2.5 text-xs font-medium text-white shadow-lg border border-neutral-800"
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
                />
              )}

              {currentView === 'customers' && (
                <CustomersView customers={customers} onAddCustomer={handleAddCustomer} />
              )}

              {currentView === 'services' && (
                <ServicesView services={services} onAddService={handleAddService} />
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
    </div>
  );
}
