import express from 'express';
import 'dotenv/config';
import { spawn, ChildProcess } from 'child_process';
import path from 'path';
import { fileURLToPath } from 'url';
import http from 'http';

const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);

const app = express();
const PORT = process.env.PORT ? parseInt(process.env.PORT, 10) : 3000;
const GO_PORT = 8085;

let goProcess: ChildProcess | null = null;

// Démarrage du binaire Go en arrière-plan
function startGoBackend() {
  const binaryPath = path.resolve(__dirname, 'backend', 'bin', 'api');
  try {
    goProcess = spawn(binaryPath, [], {
      cwd: path.resolve(__dirname, 'backend'),
      env: {
        ...process.env,
        PORT: GO_PORT.toString(),
        SERVER_ENV: 'development',
      },
      stdio: ['ignore', 'inherit', 'inherit'],
    });

    goProcess.on('error', (err) => {
      console.warn('[Proxy] Impossible de lancer le binaire Go:', err.message);
    });

    goProcess.on('exit', (code, signal) => {
      console.log(`[Proxy] Processus Go terminé (code ${code}, signal ${signal})`);
    });
  } catch (err) {
    console.warn('[Proxy] Erreur au lancement du binaire Go:', err);
  }
}

startGoBackend();

// Arrêt gracieux du processus enfant Go
process.on('SIGINT', () => {
  if (goProcess) goProcess.kill('SIGTERM');
  process.exit(0);
});

process.on('SIGTERM', () => {
  if (goProcess) goProcess.kill('SIGTERM');
  process.exit(0);
});

// Proxy HTTP transparent vers l'API Go pour /health, /ready et /api/*
function forwardToGo(req: express.Request, res: express.Response) {
  const options: http.RequestOptions = {
    hostname: '127.0.0.1',
    port: GO_PORT,
    path: req.originalUrl,
    method: req.method,
    headers: {
      ...req.headers,
      host: `127.0.0.1:${GO_PORT}`,
    },
  };

  const proxyReq = http.request(options, (proxyRes) => {
    res.writeHead(proxyRes.statusCode || 500, proxyRes.headers);
    proxyRes.pipe(res);
  });

  proxyReq.on('error', (err) => {
    // Si le processus Go est en cours de compilation ou indisponible
    if (req.path === '/health') {
      res.status(503).json({
        status: 'starting',
        service: 'pressingos-api',
        message: 'Le processus Go initialise ses connexions...',
        error: err.message,
      });
      return;
    }
    if (req.path === '/ready') {
      res.status(503).json({
        status: 'unavailable',
        service: 'pressingos-api',
        database: 'disconnected',
        error: 'Backend Go non joignable: ' + err.message,
      });
      return;
    }
    res.status(502).json({
      error: 'Erreur passerelle vers le backend Go',
      details: err.message,
    });
  });

  req.pipe(proxyReq);
}

app.use(['/health', '/ready', '/api'], forwardToGo);

// Middleware Vite pour le frontend React
async function startServer() {
  if (process.env.NODE_ENV !== 'production') {
    const { createServer: createViteServer } = await import('vite');
    const vite = await createViteServer({
      server: { middlewareMode: true },
      appType: 'spa',
    });
    // Routage SPA : servir index.html sur /verify/:tenant/:order (QR scannés).
    app.use((req, _res, next) => {
      if (req.method === 'GET' && req.path.startsWith('/verify/')) {
        req.url = '/';
      }
      next();
    });
    app.use(vite.middlewares);
  } else {
    app.use(express.static(path.resolve(__dirname, 'dist')));
    app.get('*', (_req, res) => {
      res.sendFile(path.resolve(__dirname, 'dist', 'index.html'));
    });
  }

  app.listen(PORT, '0.0.0.0', () => {
    console.log(`[PressingOS] Serveur Full-Stack actif sur http://0.0.0.0:${PORT}`);
    console.log(`[PressingOS] API Go intégrée sur le port interne ${GO_PORT}`);
  });
}

startServer().catch((err) => {
  console.error('[PressingOS] Échec du démarrage du serveur:', err);
});
