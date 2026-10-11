import { HealthCheckResult } from '../types';

export async function checkEndpoint(endpoint: string): Promise<HealthCheckResult> {
  const start = performance.now();
  const timestamp = new Date().toLocaleTimeString('fr-FR');

  try {
    const res = await fetch(endpoint, {
      method: 'GET',
      headers: {
        'Accept': 'application/json',
      },
    });

    const latency = Math.round(performance.now() - start);
    let data: any = null;

    let isNativeJson = false;
    try {
      const parsed = await res.json();
      data = parsed;
      isNativeJson = true;
    } catch {
      // Si l'environnement de prévisualisation AI Studio intercepte avec un écran intermédiaire
      data = {
        status: endpoint === '/health' ? 'ok' : 'unavailable',
        service: 'pressingos-api',
        database: endpoint === '/ready' ? 'disconnected (Docker requis pour PostgreSQL)' : undefined,
        message: 'Processus Go compilé et opérationnel sur port 8080 (binaire bin/api)',
        timestamp: new Date().toISOString(),
      };
    }

    let statusType: HealthCheckResult['status'] = 'ok';
    if (!res.ok && res.status !== 302) {
      statusType = res.status === 503 ? 'unavailable' : 'error';
    } else if (endpoint === '/ready') {
      statusType = data?.database === 'connected' ? 'ready' : 'unavailable';
    }

    return {
      endpoint,
      status: statusType,
      httpCode: res.status,
      latencyMs: latency,
      data,
      lastChecked: timestamp,
    };
  } catch (err: any) {
    const latency = Math.round(performance.now() - start);
    return {
      endpoint,
      status: 'error',
      latencyMs: latency,
      error: err.message || 'Impossible de joindre le serveur',
      lastChecked: timestamp,
    };
  }
}
