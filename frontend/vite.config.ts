import { defineConfig, Plugin } from 'vite';
import react from '@vitejs/plugin-react';
import federation from '@originjs/vite-plugin-federation';
import path from 'path';
import fs from 'fs';
import yaml from 'yaml';

// Load frontend config from YAML
interface FrontendConfig {
  frontend?: {
    dev?: {
      port?: number;
      host?: string;
    };
    url?: string;
  };
}

function loadConfig(): FrontendConfig {
  const configPath = path.resolve(__dirname, 'csd-devtrack.yaml');
  if (fs.existsSync(configPath)) {
    const content = fs.readFileSync(configPath, 'utf-8');
    return yaml.parse(content) as FrontendConfig;
  }
  return {};
}

const config = loadConfig();
const DEV_HOST = config.frontend?.dev?.host || '127.0.0.1';
const DEV_PORT = config.frontend?.dev?.port || 4044;
const FRONTEND_URL = config.frontend?.url || `http://${DEV_HOST}:${DEV_PORT}`;

// Path to csd-core frontend source
const CSD_CORE_FRONTEND = path.resolve(__dirname, '../../csd-core/frontend');

/**
 * Load exports from csd-core's exports.json at CONFIG time (build time).
 */
interface CSDExports {
  UI: string[];
  Providers: string[];
  Utils: string[];
}

function loadCSDCoreExports(): CSDExports {
  const exportsPath = path.resolve(CSD_CORE_FRONTEND, 'src/shared/federation/exports.json');
  if (fs.existsSync(exportsPath)) {
    const content = fs.readFileSync(exportsPath, 'utf-8');
    return JSON.parse(content);
  }
  console.warn('[csd-core-external] exports.json not found, using empty exports');
  return { UI: [], Providers: [], Utils: [] };
}

// Load exports at config time
const CSD_EXPORTS = loadCSDCoreExports();

/**
 * Generate virtual module code that loads from federation shared scope.
 */
function generateModuleLoader(moduleName: string, exports: string[]): string {
  const exportStatements = exports.map(name => `export const ${name} = mod.${name};`).join('\n          ');

  return `
          async function getModule() {
            const shared = globalThis.__federation_shared__?.default?.['csd_core/${moduleName}'];
            if (!shared) {
              throw new Error('csd_core/${moduleName} not found in shared scope - is csd-core host running?');
            }
            const version = Object.keys(shared)[0];
            if (!version) {
              throw new Error('csd_core/${moduleName} has no versions in shared scope');
            }
            const entry = shared[version];
            if (entry.loaded && typeof entry.get === 'function') {
              const factory = await entry.get();
              return typeof factory === 'function' ? factory() : factory;
            }
            throw new Error('csd_core/${moduleName} not properly loaded in shared scope');
          }
          const mod = await getModule();
          ${exportStatements}
          export default mod;
        `;
}

/**
 * Plugin to handle csd_core/* imports.
 */
function csdCoreExternalPlugin(): Plugin {
  const CSD_MODULES = ['csd_core/UI', 'csd_core/Providers', 'csd_core/Utils', 'csd_core/Types'];

  return {
    name: 'csd-core-external',
    enforce: 'pre',
    resolveId(id) {
      if (CSD_MODULES.includes(id)) {
        return `\0${id}`;
      }
      return null;
    },
    load(id) {
      if (id === '\0csd_core/UI') {
        return generateModuleLoader('UI', CSD_EXPORTS.UI);
      }
      if (id === '\0csd_core/Providers') {
        return generateModuleLoader('Providers', CSD_EXPORTS.Providers);
      }
      if (id === '\0csd_core/Utils') {
        return generateModuleLoader('Utils', CSD_EXPORTS.Utils);
      }
      if (id === '\0csd_core/Types') {
        return `export default {};`;
      }
      return null;
    },
  };
}

// https://vitejs.dev/config/
export default defineConfig(({ mode }) => {
  return {
    base: `${FRONTEND_URL}/`,

    plugins: [
      csdCoreExternalPlugin(),
      react(),
      federation({
        name: 'csd_devtrack',
        filename: 'remoteEntry.js',

        // Expose modules for csd-core to load
        exposes: {
          './Routes': './src/Routes.tsx',
          './Translations': './src/translations/generated/index.ts',
          './AppInfo': './src/appInfo.ts',
        },

        remotes: {},

        // Shared dependencies - use singleton from host
        shared: {
          react: { singleton: true, requiredVersion: '^19.0.0', import: false },
          'react-dom': { singleton: true, requiredVersion: '^19.0.0', import: false },
          'react-router-dom': { singleton: true, requiredVersion: '^7.0.0', import: false },
          '@apollo/client': { singleton: true, requiredVersion: '^3.0.0', import: false },
          '@mui/material': { singleton: true, requiredVersion: '^7.0.0', import: false },
          '@emotion/react': { singleton: true, requiredVersion: '^11.0.0', import: false },
          '@emotion/styled': { singleton: true, requiredVersion: '^11.0.0', import: false },
        },
      }),
    ],

    resolve: {
      alias: {
        '@devtrack': path.resolve(__dirname, './src/modules/devtrack'),
        '@': path.resolve(__dirname, './src'),
      },
    },

    server: {
      port: DEV_PORT,
      host: DEV_HOST,
      cors: {
        origin: '*',
        methods: ['GET', 'POST', 'PUT', 'DELETE', 'OPTIONS'],
        allowedHeaders: ['Content-Type', 'Authorization'],
      },
      headers: {
        'Access-Control-Allow-Origin': '*',
        'Access-Control-Allow-Methods': 'GET, POST, PUT, DELETE, OPTIONS',
        'Access-Control-Allow-Headers': 'Content-Type, Authorization',
      },
    },

    preview: {
      port: DEV_PORT,
      host: DEV_HOST,
      cors: {
        origin: '*',
        methods: ['GET', 'POST', 'PUT', 'DELETE', 'OPTIONS'],
        allowedHeaders: ['Content-Type', 'Authorization'],
      },
      headers: {
        'Access-Control-Allow-Origin': '*',
        'Access-Control-Allow-Methods': 'GET, POST, PUT, DELETE, OPTIONS',
        'Access-Control-Allow-Headers': 'Content-Type, Authorization',
      },
    },

    build: {
      outDir: 'build',
      target: 'esnext',
      minify: false,
      cssCodeSplit: false,
    },

    css: {
      devSourcemap: true,
    },
  };
});
