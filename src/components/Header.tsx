import React from 'react';
import { Menu, Plus, ArrowRightLeft } from 'lucide-react';
import { ViewMode, Tenant, HealthCheckResult, User } from '../types';
import { ThemeToggle } from './ThemeToggle';

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
      default:
        return 'Tableau de bord';
    }
  };

  return (
    <header className="sticky top-0 z-30 flex h-16 w-full items-center justify-between border-b border-neutral-200 bg-white/95 px-4 backdrop-blur-xs transition-colors dark:border-neutral-800 dark:bg-[#080808]/95 md:px-8 ps-elevated-static">
      {/* Zone 1: Mobile toggle + Breadcrumb trail */}
      <div className="flex items-center gap-3">
        <button
          onClick={onOpenMobileMenu}
          className="rounded-md p-1.5 text-neutral-600 hover:bg-neutral-950 hover:text-white dark:text-neutral-400 md:hidden dark:hover:bg-neutral-100 dark:hover:text-neutral-950"
          aria-label="Ouvrir le menu"
        >
          <Menu className="h-5 w-5" />
        </button>

        <div className="flex items-center gap-2 text-xs font-medium text-neutral-500 dark:text-neutral-400">
          <span className="hidden sm:inline font-medium text-neutral-700 dark:text-neutral-300">
            {tenant ? tenant.name : 'PressingOS SaaS'}
          </span>
          <span className="hidden sm:inline text-neutral-300 dark:text-neutral-700">/</span>
          <span className="font-semibold text-neutral-900 dark:text-neutral-100">{getBreadcrumbTitle()}</span>
        </div>
      </div>

      {/* Zone 2: Navigation meta info */}
      <div className="hidden lg:flex items-center gap-3 text-xs">
        <div className="flex items-center gap-2 text-neutral-600 bg-neutral-50 px-2.5 py-1 rounded-md border border-neutral-200/60 dark:bg-neutral-900 dark:border-neutral-800 dark:text-neutral-400 hover:border-neutral-950 dark:hover:border-white/70 ps-elevated">
          <span className="h-1.5 w-1.5 rounded-full bg-emerald-500" />
          <span className="font-mono text-[11px] text-neutral-700 dark:text-neutral-300">Session active</span>
          <span className="text-neutral-300 dark:text-neutral-700">·</span>
          <span className="text-[11px] text-neutral-700 dark:text-neutral-300 font-medium capitalize">
            Rôle : {currentUser.role}
          </span>
        </div>
      </div>

      {/* Zone 3: Primary action + Theme toggle + User switcher */}
      <div className="flex items-center gap-2 sm:gap-3 shrink-0">
        {/* Bascule de Thème Sombre / Clair */}
        <ThemeToggle />

        {currentUser.role !== 'operator' && (
          <button
            onClick={onNewOrderClick}
            className="flex items-center gap-1.5 rounded-md bg-neutral-900 px-3 py-1.5 text-xs font-medium text-white transition-colors hover:bg-black shadow-xs whitespace-nowrap shrink-0 dark:bg-neutral-100 dark:text-neutral-900 dark:hover:bg-white"
          >
            <Plus className="h-3.5 w-3.5" />
            <span className="hidden sm:inline">Nouveau Dépôt</span>
            <span className="sm:hidden">Dépôt</span>
          </button>
        )}

        {/* Profile Card & Role Switcher */}
        <button
          onClick={onOpenLoginModal}
          className="flex items-center gap-2 pl-2 border-l border-neutral-200 dark:border-neutral-800 hover:opacity-80 transition-opacity"
          title="Changer de compte ou de rôle"
        >
          <img
            src="/src/assets/images/manager_avatar_1791699869703.jpg"
            alt={currentUser.full_name}
            referrerPolicy="no-referrer"
            className="h-8 w-8 rounded-full object-cover border border-neutral-200 dark:border-neutral-700"
          />
          <div className="hidden xl:block text-left text-xs leading-tight">
            <div className="font-medium text-neutral-900 dark:text-neutral-100 flex items-center gap-1">
              <span>{currentUser.full_name.split(' ')[0]}</span>
              <ArrowRightLeft className="h-2.5 w-2.5 text-neutral-400 dark:text-neutral-500" />
            </div>
            <div className="text-[10px] text-neutral-500 dark:text-neutral-400 font-mono uppercase">
              {currentUser.role}
            </div>
          </div>
        </button>
      </div>
    </header>
  );
};
