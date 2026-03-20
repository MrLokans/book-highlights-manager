import { Page } from '@playwright/test';
import { spawn, ChildProcess } from 'child_process';
import { createServer } from 'net';
import { mkdtempSync, cpSync, mkdirSync, rmSync, existsSync } from 'fs';
import { join } from 'path';
import { tmpdir } from 'os';

const PROJECT_ROOT = join(__dirname, '..', '..');
const SEED_DB = join(__dirname, '..', 'fixtures', 'seed.db');
const DEFAULT_BINARY = join(__dirname, '..', 'bin', 'highlights-exporter');

// 32 bytes base64-encoded: "testencryptionkey1234567890ABCDE"
const TEST_ENCRYPTION_KEY = 'dGVzdGVuY3J5cHRpb25rZXkxMjM0NTY3ODkwQUJDREU=';

export interface AppInstance {
  url: string;
  tmpDir: string;
  stop: () => Promise<void>;
}

async function getFreePort(): Promise<number> {
  return new Promise((resolve, reject) => {
    const server = createServer();
    server.listen(0, '127.0.0.1', () => {
      const addr = server.address();
      if (addr && typeof addr === 'object') {
        const port = addr.port;
        server.close(() => resolve(port));
      } else {
        reject(new Error('Failed to get free port'));
      }
    });
    server.on('error', reject);
  });
}

async function waitForHealth(url: string, timeoutMs = 10_000): Promise<void> {
  const start = Date.now();
  while (Date.now() - start < timeoutMs) {
    try {
      const resp = await fetch(`${url}/health`);
      if (resp.ok) return;
    } catch {
      // Server not ready yet
    }
    await new Promise(r => setTimeout(r, 100));
  }
  throw new Error(`App did not become healthy within ${timeoutMs}ms at ${url}`);
}

export async function startApp(): Promise<AppInstance> {
  const externalUrl = process.env.E2E_BASE_URL;
  if (externalUrl) {
    return {
      url: externalUrl,
      tmpDir: '',
      stop: async () => {},
    };
  }

  const binaryPath = process.env.E2E_BINARY_PATH || DEFAULT_BINARY;
  if (!existsSync(binaryPath)) {
    throw new Error(
      `Go binary not found at ${binaryPath}. Run "go build -o e2e/bin/highlights-exporter ." from project root.`
    );
  }

  if (!existsSync(SEED_DB)) {
    throw new Error(
      `Seed database not found at ${SEED_DB}. Run "make seed-e2e" first.`
    );
  }

  const tmpDir = mkdtempSync(join(tmpdir(), 'e2e-'));
  const dbPath = join(tmpDir, 'app.db');
  const vaultDir = join(tmpDir, 'vault');
  const auditDir = join(tmpDir, 'audit');

  cpSync(SEED_DB, dbPath);
  mkdirSync(vaultDir, { recursive: true });
  mkdirSync(auditDir, { recursive: true });

  const port = await getFreePort();
  const url = `http://127.0.0.1:${port}`;

  const proc: ChildProcess = spawn(binaryPath, [], {
    env: {
      ...process.env,
      DATABASE_PATH: dbPath,
      PORT: String(port),
      HOST: '127.0.0.1',
      OBSIDIAN_VAULT_DIR: vaultDir,
      OBSIDIAN_EXPORT_DIR: vaultDir,
      AUDIT_DIR: auditDir,
      AUTH_MODE: 'local',
      TOKEN_ENCRYPTION_KEY: TEST_ENCRYPTION_KEY,
      TEMPLATES_PATH: join(PROJECT_ROOT, 'templates'),
      STATIC_PATH: join(PROJECT_ROOT, 'static'),
      AUTH_SECURE_COOKIES: 'false',
    },
    stdio: ['ignore', 'pipe', 'pipe'],
  });

  // Collect stdout and stderr for error reporting
  let stderr = '';
  let stdout = '';
  proc.stderr?.on('data', (chunk) => {
    stderr += chunk.toString();
  });
  proc.stdout?.on('data', (chunk) => {
    stdout += chunk.toString();
  });

  proc.on('error', (err) => {
    throw new Error(`Failed to start app: ${err.message}\nStdout: ${stdout}\nStderr: ${stderr}`);
  });

  try {
    await waitForHealth(url);
  } catch (err) {
    proc.kill('SIGKILL');
    rmSync(tmpDir, { recursive: true, force: true });
    throw new Error(
      `App failed to start.\nStdout:\n${stdout}\nStderr:\n${stderr}\nOriginal error: ${err}`
    );
  }

  return {
    url,
    tmpDir,
    stop: async () => {
      if (proc.exitCode === null) {
        proc.kill('SIGTERM');
        await new Promise<void>((resolve) => {
          const timer = setTimeout(() => {
            proc.kill('SIGKILL');
            resolve();
          }, 5_000);
          proc.on('exit', () => {
            clearTimeout(timer);
            resolve();
          });
        });
      }
      rmSync(tmpDir, { recursive: true, force: true });
    },
  };
}
