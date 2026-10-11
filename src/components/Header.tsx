import React from 'react';
import { Menu, Plus, RefreshCw, Terminal, ArrowRightLeft, Shield } from 'lucide-react';
import { ViewMode, Tenant, HealthCheckResult, User } from '../types';

interface HeaderProps {
  currentView: ViewMode;
  tenant?: Tenant;
  currentUser: User;
  onOpenLoginModal: () => void;
  health: HealthCheckResult | null;
  isChecking: boolean;
  onRefreshHealth: () => void;
  onOpenMobileMenu: () => void;
  onNewOrderClick: () => void;
}

export const Header: React.FC<HeaderProps> = ({
  currentView,
  tenant,
  currentUser,
  onOpenLoginModal,
  health,
  isChecking,
  onRefreshHealth,
  onOpenMobileMenu,
  onNewOrderClick,
}) => {
  const getBreadcrumbTitle = () => {
    switch (currentView) {
      case 'dashboard':
        return 'Tableau de bord opérationnel';
      case 'orders':
        return 'Suivi du linge & Dépôts';
      case 'customers':
        return 'Annuaire des clients';
      case 'services':
        return 'Grille des tarifs (FCFA)';
      case 'superadmin':
        return 'SuperAdmin — Plateforme & Abonnements';
      case 'system-health':
        return 'Diagnostics Serveur, JWT & PostgreSQL';
      case 'docs':
        return 'Guide technique Mac & Architecture';
      default:
        return 'Tableau de bord';
    }
  };

  return (
    <header className="sticky top-0 z-30 flex h-16 w-full items-center justify-between border-b border-neutral-200 bg-white/95 px-4 backdrop-blur-xs md:px-8">
      {/* Zone 1: Mobile toggle + Breadcrumb trail */}
      <div className="flex items-center gap-3">
        <button
          onClick={onOpenMobileMenu}
          className="rounded-md p-1.5 text-neutral-600 hover:bg-neutral-100 md:hidden"
          aria-label="Ouvrir le menu"
        >
          <Menu className="h-5 w-5" />
        </button>

        <div className="flex items-center gap-2 text-xs font-medium text-neutral-500">
          <span className="hidden sm:inline font-medium text-neutral-700">
            {tenant ? tenant.name : 'PressingOS SaaS'}
          </span>
          <span className="hidden sm:inline text-neutral-300">/</span>
          <span className="font-semibold text-neutral-900">{getBreadcrumbTitle()}</span>
        </div>
      </div>

      {/* Zone 2: Navigation meta info & Live API status */}
      <div className="hidden lg:flex items-center gap-4 text-xs">
        <div className="flex items-center gap-2 text-neutral-600 bg-neutral-50 px-2.5 py-1 rounded-md border border-neutral-200/60">
          <Terminal className="h-3.5 w-3.5 text-neutral-500" />
          <span className="font-mono text-[11px] text-neutral-700">JWT Auth + Chi</span>
          <span className="text-neutral-300">·</span>
          <span className="font-mono text-[11px] text-neutral-700">Rôle: {currentUser.role}</span>
        </div>

        <button
          onClick={onRefreshHealth}
          disabled={isChecking}
          title="Tester la connexion à l'API Go"
          className="flex items-center gap-1.5 text-[11px] text-neutral-600 hover:text-neutral-900 transition-colors disabled:opacity-50"
        >
          <RefreshCw className={`h-3 w-3 ${isChecking ? 'animate-spin text-neutral-900' : ''}`} />
          <span>{isChecking ? 'Vérification...' : 'Sonder l’API'}</span>
          {health?.latencyMs !== undefined && (
            <span className="font-mono text-[10px] text-neutral-400">({health.latencyMs}ms)</span>
          )}
        </button>
      </div>

      {/* Zone 3: Primary action + User switcher */}
      <div className="flex items-center gap-3 shrink-0">
        {currentUser.role !== 'operator' && (
          <button
            onClick={onNewOrderClick}
            className="flex items-center gap-1.5 rounded-md bg-neutral-900 px-3 py-1.5 text-xs font-medium text-white transition-colors hover:bg-neutral-800 shadow-xs whitespace-nowrap shrink-0"
          >
            <Plus className="h-3.5 w-3.5" />
            <span>Nouveau Dépôt</span>
          </button>
        )}

        {/* Profile Card & Role Switcher */}
        <button
          onClick={onOpenLoginModal}
          className="flex items-center gap-2 pl-2 border-l border-neutral-200 hover:opacity-80 transition-opacity"
          title="Changer de compte ou de rôle"
        >
          <img
            src="/src/assets/images/manager_avatar_1791699869703.jpg"
            alt={currentUser.full_name}
            referrerPolicy="no-referrer"
            className="h-8 w-8 rounded-full object-cover border border-neutral-200"
          />
          <div className="hidden xl:block text-left text-xs leading-tight">
            <div className="font-medium text-neutral-900 flex items-center gap-1">
              <span>{currentUser.full_name.split(' ')[0]}</span>
              <ArrowRightLeft className="h-2.5 w-2.5 text-neutral-400" />
            </div>
            <div className="text-[10px] text-neutral-500 font-mono uppercase">{currentUser.role}</div>
          </div>
        </button>
      </div>
    </header>
  );
};
