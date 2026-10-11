import React, { useState } from 'react';
import { BookOpen, Terminal, Database, Code, Copy, Check } from 'lucide-react';

export const DocsView: React.FC = () => {
  const [activeTab, setActiveTab] = useState<'install' | 'arch' | 'db' | 'api'>('install');
  const [copiedIndex, setCopiedIndex] = useState<number | null>(null);

  const copyToClipboard = (text: string, index: number) => {
    navigator.clipboard.writeText(text);
    setCopiedIndex(index);
    setTimeout(() => setCopiedIndex(null), 2000);
  };

  return (
    <div className="space-y-6">
      {/* Top Header */}
      <div className="rounded-lg border border-neutral-200 bg-white p-5">
        <h1 className="text-lg font-semibold tracking-tight text-neutral-900">
          Documentation Technique & Guide Débutant macOS
        </h1>
        <p className="mt-0.5 text-xs text-neutral-500">
          Toutes les explications et commandes pour démarrer, tester et déployer PressingOS sur votre MacBook avec VS Code.
        </p>

        {/* Tab switch */}
        <div className="mt-4 flex flex-wrap gap-2 border-b border-neutral-100 pb-3">
          {[
            { id: 'install', label: '1. Démarrage Mac (Terminal)', icon: Terminal },
            { id: 'arch', label: '2. Architecture SaaS & Multi-Tenant', icon: Code },
            { id: 'db', label: '3. Schéma PostgreSQL & Migrations', icon: Database },
            { id: 'api', label: '4. Spécification API REST (Go)', icon: BookOpen },
          ].map((tab) => {
            const Icon = tab.icon;
            const isActive = activeTab === tab.id;
            return (
              <button
                key={tab.id}
                onClick={() => setActiveTab(tab.id as any)}
                className={`flex items-center gap-1.5 rounded-md px-3 py-1.5 text-xs font-medium transition-colors ${
                  isActive
                    ? 'bg-neutral-900 text-white'
                    : 'bg-neutral-50 text-neutral-600 hover:bg-neutral-100 hover:text-neutral-900'
                }`}
              >
                <Icon className="h-3.5 w-3.5" />
                <span>{tab.label}</span>
              </button>
            );
          })}
        </div>
      </div>

      {/* Content depending on active tab */}
      {activeTab === 'install' && (
        <div className="space-y-4">
          <div className="rounded-lg border border-neutral-200 bg-white p-5">
            <h2 className="text-sm font-semibold text-neutral-900">
              Procédure Pas-à-Pas sur votre MacBook
            </h2>
            <p className="mt-1 text-xs text-neutral-600">
              Ouvrez le terminal de VS Code (<kbd className="rounded bg-neutral-100 px-1 py-0.5 font-mono text-[10px]">Ctrl + ~</kbd>) et suivez ces 3 étapes simples :
            </p>

            <div className="mt-4 space-y-4">
              {/* Step 1 */}
              <div className="rounded-md border border-neutral-100 bg-neutral-50 p-3.5">
                <div className="flex items-center justify-between">
                  <span className="text-xs font-semibold text-neutral-900">
                    Étape 1 : Créer le fichier d'environnement
                  </span>
                  <button
                    onClick={() => copyToClipboard('cp .env.example .env', 1)}
                    className="flex items-center gap-1 text-[11px] text-neutral-500 hover:text-neutral-900"
                  >
                    {copiedIndex === 1 ? <Check className="h-3 w-3 text-emerald-600" /> : <Copy className="h-3 w-3" />}
                    <span>{copiedIndex === 1 ? 'Copié !' : 'Copier'}</span>
                  </button>
                </div>
                <pre className="mt-2 rounded bg-neutral-900 p-2 font-mono text-[11px] text-neutral-100">
                  cp .env.example .env
                </pre>
              </div>

              {/* Step 2 */}
              <div className="rounded-md border border-neutral-100 bg-neutral-50 p-3.5">
                <div className="flex items-center justify-between">
                  <span className="text-xs font-semibold text-neutral-900">
                    Étape 2 : Lancer PostgreSQL avec Docker
                  </span>
                  <button
                    onClick={() => copyToClipboard('docker compose up -d', 2)}
                    className="flex items-center gap-1 text-[11px] text-neutral-500 hover:text-neutral-900"
                  >
                    {copiedIndex === 2 ? <Check className="h-3 w-3 text-emerald-600" /> : <Copy className="h-3 w-3" />}
                    <span>{copiedIndex === 2 ? 'Copié !' : 'Copier'}</span>
                  </button>
                </div>
                <pre className="mt-2 rounded bg-neutral-900 p-2 font-mono text-[11px] text-neutral-100">
                  docker compose up -d
                </pre>
                <p className="mt-1.5 text-[11px] text-neutral-500">
                  Crée la base de données <code className="font-mono text-neutral-700">pressing_os</code>, monte le volume persistant et initialise les tables.
                </p>
              </div>

              {/* Step 3 */}
              <div className="rounded-md border border-neutral-100 bg-neutral-50 p-3.5">
                <div className="flex items-center justify-between">
                  <span className="text-xs font-semibold text-neutral-900">
                    Étape 3 : Lancer les tests et l'API Go
                  </span>
                  <button
                    onClick={() => copyToClipboard('cd backend && go test ./... && go run cmd/api/main.go', 3)}
                    className="flex items-center gap-1 text-[11px] text-neutral-500 hover:text-neutral-900"
                  >
                    {copiedIndex === 3 ? <Check className="h-3 w-3 text-emerald-600" /> : <Copy className="h-3 w-3" />}
                    <span>{copiedIndex === 3 ? 'Copié !' : 'Copier'}</span>
                  </button>
                </div>
                <pre className="mt-2 rounded bg-neutral-900 p-2 font-mono text-[11px] text-neutral-100">
                  cd backend && go test ./... && go run cmd/api/main.go
                </pre>
                <p className="mt-1.5 text-[11px] text-neutral-500">
                  Exécute les tests unitaires et démarre l'API sur <code className="font-mono text-neutral-700">http://localhost:8080</code>.
                </p>
              </div>
            </div>
          </div>
        </div>
      )}

      {activeTab === 'arch' && (
        <div className="rounded-lg border border-neutral-200 bg-white p-5 space-y-4">
          <h2 className="text-sm font-semibold text-neutral-900">
            Principes d'Architecture & Monolithe Modulaire
          </h2>
          <p className="text-xs text-neutral-600 leading-relaxed">
            Pour respecter l'échéance de 5 jours tout en garantissant un niveau de robustesse élevé, PressingOS est architecturé en <strong>Monolithe Modulaire</strong> plutôt qu'en microservices :
          </p>
          <ul className="list-disc pl-5 text-xs text-neutral-600 space-y-2">
            <li>
              <strong>Un seul processus exécutable Go</strong> : Pas de complexité de déploiement multi-conteneurs pour l'API.
            </li>
            <li>
              <strong>Isolation stricte des données (Multi-Tenant)</strong> : Chaque table métier contient une colonne <code className="font-mono text-neutral-800">tenant_id UUID</code>. Un pressing A ne verra jamais les clients ni le chiffre d'affaires d'un pressing B.
            </li>
            <li>
              <strong>Sécurité des secrets</strong> : Mots de passe masqués dans les logs grâce à la méthode <code className="font-mono text-neutral-800">RedactedDatabaseURL()</code>.
            </li>
            <li>
              <strong>Arrêt propre (Graceful Shutdown)</strong> : En cas de redémarrage, le serveur libère toutes les connexions sans couper de transactions financières.
            </li>
          </ul>
        </div>
      )}

      {activeTab === 'db' && (
        <div className="rounded-lg border border-neutral-200 bg-white p-5 space-y-4">
          <h2 className="text-sm font-semibold text-neutral-900">
            Schéma PostgreSQL & Fichiers de Migration
          </h2>
          <p className="text-xs text-neutral-600">
            Les tables relationnelles sont versionnées dans <code className="font-mono text-neutral-800">backend/migrations/</code> :
          </p>
          <div className="grid grid-cols-1 gap-3 sm:grid-cols-2 text-xs">
            <div className="rounded border border-neutral-100 bg-neutral-50 p-3">
              <span className="font-mono font-semibold text-neutral-900">tenants</span>
              <p className="text-neutral-500 text-[11px] mt-1">Pressings indépendants (Nom, adresse Abidjan, devise XOF, statut)</p>
            </div>
            <div className="rounded border border-neutral-100 bg-neutral-50 p-3">
              <span className="font-mono font-semibold text-neutral-900">users</span>
              <p className="text-neutral-500 text-[11px] mt-1">Employés, caissiers, gérants et super-administrateur</p>
            </div>
            <div className="rounded border border-neutral-100 bg-neutral-50 p-3">
              <span className="font-mono font-semibold text-neutral-900">customers</span>
              <p className="text-neutral-500 text-[11px] mt-1">Fichier clients du pressing (nom, téléphone, quartier)</p>
            </div>
            <div className="rounded border border-neutral-100 bg-neutral-50 p-3">
              <span className="font-mono font-semibold text-neutral-900">services</span>
              <p className="text-neutral-500 text-[11px] mt-1">Grille tarifaire (Chemise, Costume, Robe, Couette en FCFA)</p>
            </div>
            <div className="rounded border border-neutral-100 bg-neutral-50 p-3">
              <span className="font-mono font-semibold text-neutral-900">orders & order_items</span>
              <p className="text-neutral-500 text-[11px] mt-1">Commandes de linge, tickets, étapes et détail des pièces</p>
            </div>
            <div className="rounded border border-neutral-100 bg-neutral-50 p-3">
              <span className="font-mono font-semibold text-neutral-900">payments</span>
              <p className="text-neutral-500 text-[11px] mt-1">Encaissements en espèces, Wave, Orange Money et MTN MoMo</p>
            </div>
          </div>
        </div>
      )}

      {activeTab === 'api' && (
        <div className="rounded-lg border border-neutral-200 bg-white p-5 space-y-4">
          <h2 className="text-sm font-semibold text-neutral-900">
            Spécification des Endpoints de l'API REST Go
          </h2>
          <div className="space-y-3 text-xs">
            <div className="border-b border-neutral-100 pb-2.5">
              <div className="flex items-center gap-2">
                <span className="font-mono text-emerald-700 bg-emerald-50 px-1.5 py-0.5 rounded font-medium">GET</span>
                <span className="font-mono font-semibold text-neutral-900">/health</span>
                <span className="text-neutral-400">· 200 OK</span>
              </div>
              <p className="mt-1 text-neutral-500 text-[11px]">Vérifie que l'API Go tourne et écoute.</p>
            </div>

            <div className="border-b border-neutral-100 pb-2.5">
              <div className="flex items-center gap-2">
                <span className="font-mono text-emerald-700 bg-emerald-50 px-1.5 py-0.5 rounded font-medium">GET</span>
                <span className="font-mono font-semibold text-neutral-900">/ready</span>
                <span className="text-neutral-400">· 200 OK ou 503 Unavailable</span>
              </div>
              <p className="mt-1 text-neutral-500 text-[11px]">Vérifie la liaison réelle avec PostgreSQL (timeout 5s).</p>
            </div>

            <div>
              <div className="flex items-center gap-2">
                <span className="font-mono text-emerald-700 bg-emerald-50 px-1.5 py-0.5 rounded font-medium">GET</span>
                <span className="font-mono font-semibold text-neutral-900">/api/v1/system/info</span>
                <span className="text-neutral-400">· 200 OK</span>
              </div>
              <p className="mt-1 text-neutral-500 text-[11px]">Retourne les métadonnées de l'architecture logicielle.</p>
            </div>
          </div>
        </div>
      )}
    </div>
  );
};
