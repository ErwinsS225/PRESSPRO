import React, { useState } from 'react';
import { X, Shield, ArrowRight, CheckCircle2 } from 'lucide-react';
import { PRESET_ACCOUNTS, loginUser } from '../lib/authApi';
import { AuthSession } from '../types';

interface LoginModalProps {
  currentSession: AuthSession;
  onClose: () => void;
  onLoginSuccess: (session: AuthSession) => void;
}

export const LoginModal: React.FC<LoginModalProps> = ({ currentSession, onClose, onLoginSuccess }) => {
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);

  const handleSelectPreset = async (presetEmail: string, presetPass: string) => {
    setError(null);
    setLoading(true);
    try {
      const session = await loginUser(presetEmail, presetPass);
      onLoginSuccess(session);
      onClose();
    } catch (err: any) {
      setError(err.message || 'Erreur de connexion');
    } finally {
      setLoading(false);
    }
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!email || !password) return;
    setError(null);
    setLoading(true);
    try {
      const session = await loginUser(email, password);
      onLoginSuccess(session);
      onClose();
    } catch (err: any) {
      setError(err.message || 'Identifiants invalides');
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-neutral-900/40 dark:bg-black/75 p-4 backdrop-blur-xs">
      <div className="w-full max-w-md rounded-lg border border-neutral-200 bg-white p-6 shadow-xl dark:border-neutral-800 dark:bg-[#111111]">
        <div className="flex items-center justify-between border-b border-neutral-100 dark:border-neutral-800 pb-3.5">
          <div className="flex items-center gap-2">
            <div className="flex h-7 w-7 items-center justify-center rounded-md bg-neutral-900 text-white dark:bg-neutral-100 dark:text-neutral-900">
              <Shield className="h-4 w-4" />
            </div>
            <div>
              <h2 className="text-sm font-semibold text-neutral-900 dark:text-neutral-100">
                Changer de Rôle ou d’Utilisateur
              </h2>
              <p className="text-[11px] text-neutral-500 dark:text-neutral-400">
                Authentification JWT signée & Isolation Multi-Tenant
              </p>
            </div>
          </div>
          <button
            onClick={onClose}
            className="rounded p-1 text-neutral-400 hover:text-neutral-700 dark:text-neutral-500 dark:hover:text-neutral-200"
          >
            <X className="h-4 w-4" />
          </button>
        </div>

        {error && (
          <div className="mt-3 rounded-md bg-rose-50 border border-rose-200 dark:bg-rose-950/40 dark:border-rose-900 p-2.5 text-xs text-rose-700 dark:text-rose-300">
            {error}
          </div>
        )}

        {/* 1-Click Role Switcher */}
        <div className="mt-4">
          <label className="text-[11px] font-medium tracking-wider uppercase text-neutral-400 dark:text-neutral-500">
            Comptes de Démonstration (1 Clic)
          </label>
          <div className="mt-2 space-y-2">
            {PRESET_ACCOUNTS.map((acc) => {
              const isCurrent = currentSession.user.email === acc.email;
              return (
                <button
                  key={acc.email}
                  type="button"
                  onClick={() => handleSelectPreset(acc.email, acc.password)}
                  disabled={loading}
                  className={`group flex w-full flex-col rounded-md border p-3 text-left transition-colors ${
                    isCurrent
                      ? 'border-neutral-900 bg-neutral-50 dark:border-neutral-100 dark:bg-neutral-800/60'
                      : 'border-neutral-200 hover:border-neutral-400 hover:bg-neutral-50/60 dark:border-neutral-800 dark:hover:border-neutral-700 dark:hover:bg-neutral-900'
                  }`}
                >
                  <div className="flex items-center justify-between">
                    <div className="flex items-center gap-2">
                      <span className="font-semibold text-xs text-neutral-900 dark:text-neutral-100">
                        {acc.label}
                      </span>
                      <span className="font-mono text-[10px] text-neutral-500 bg-white px-1.5 py-0.5 rounded border border-neutral-200 dark:bg-neutral-900 dark:border-neutral-700 dark:text-neutral-400">
                        {acc.role}
                      </span>
                    </div>
                    {isCurrent ? (
                      <span className="flex items-center gap-1 font-mono text-[10px] text-emerald-700 dark:text-emerald-400 font-medium">
                        <CheckCircle2 className="h-3 w-3" /> Actif
                      </span>
                    ) : (
                      <ArrowRight className="h-3 w-3 text-neutral-400 group-hover:text-neutral-900 dark:text-neutral-500 dark:group-hover:text-neutral-200 group-hover:translate-x-0.5 transition-transform" />
                    )}
                  </div>
                  <p className="mt-1 text-[11px] text-neutral-600 dark:text-neutral-400 leading-snug">
                    {acc.description}
                  </p>
                  <div className="mt-1.5 flex items-center gap-2 text-[10px] font-mono text-neutral-400 dark:text-neutral-500">
                    <span>{acc.email}</span>
                    <span>·</span>
                    <span>{acc.tenantName}</span>
                  </div>
                </button>
              );
            })}
          </div>
        </div>

        {/* Divider */}
        <div className="relative my-4">
          <div className="absolute inset-0 flex items-center">
            <div className="w-full border-t border-neutral-200 dark:border-neutral-800" />
          </div>
          <div className="relative flex justify-center text-xs uppercase">
            <span className="bg-white dark:bg-[#111111] px-2 text-[10px] text-neutral-400 dark:text-neutral-500 font-medium">
              Ou se connecter manuellement
            </span>
          </div>
        </div>

        {/* Manual Form */}
        <form onSubmit={handleSubmit} className="space-y-3">
          <div>
            <label className="block text-[11px] font-medium text-neutral-700 dark:text-neutral-300">
              Adresse Email
            </label>
            <input
              type="email"
              required
              placeholder="votre-email@pressing.ci"
              value={email}
              onChange={(e) => setEmail(e.target.value)}
              className="mt-1 w-full rounded-md border border-neutral-200 px-3 py-1.5 text-xs text-neutral-900 focus:border-neutral-900 focus:outline-hidden dark:border-neutral-800 dark:bg-neutral-900 dark:text-neutral-100 dark:focus:border-neutral-100"
            />
          </div>

          <div>
            <label className="block text-[11px] font-medium text-neutral-700 dark:text-neutral-300">
              Mot de passe
            </label>
            <input
              type="password"
              required
              placeholder="••••••••"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              className="mt-1 w-full rounded-md border border-neutral-200 px-3 py-1.5 text-xs text-neutral-900 focus:border-neutral-900 focus:outline-hidden dark:border-neutral-800 dark:bg-neutral-900 dark:text-neutral-100 dark:focus:border-neutral-100"
            />
          </div>

          <div className="mt-4 flex justify-end gap-2 pt-2">
            <button
              type="button"
              onClick={onClose}
              className="rounded-md border border-neutral-200 px-3 py-1.5 text-xs font-medium text-neutral-700 hover:bg-neutral-50 dark:border-neutral-700 dark:text-neutral-300 dark:hover:bg-neutral-800"
            >
              Fermer
            </button>
            <button
              type="submit"
              disabled={loading}
              className="rounded-md bg-neutral-900 px-3 py-1.5 text-xs font-medium text-white hover:bg-neutral-800 dark:bg-neutral-100 dark:text-neutral-900 dark:hover:bg-white disabled:opacity-50"
            >
              {loading ? 'Connexion...' : 'Se connecter'}
            </button>
          </div>
        </form>
      </div>
    </div>
  );
};
