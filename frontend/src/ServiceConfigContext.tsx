import React, { createContext, useContext, useState, useEffect } from 'react';
import { ServiceConfig } from './types';

const ServiceConfigContext = createContext<ServiceConfig | null>(null);

interface ServiceConfigProviderProps {
  config?: ServiceConfig;
  children: React.ReactNode;
}

export const ServiceConfigProvider: React.FC<ServiceConfigProviderProps> = ({ config, children }) => {
  const [serviceConfig, setServiceConfig] = useState<ServiceConfig | null>(config || null);

  useEffect(() => {
    if (config) {
      setServiceConfig(config);
      return;
    }

    // Load from YAML config if not provided (standalone mode)
    const loadConfig = async () => {
      try {
        const isProd = window.location.hostname !== 'localhost' && window.location.hostname !== '127.0.0.1';
        const configFile = isProd ? '/csd-devtrack.production.yaml' : '/csd-devtrack.yaml';
        const response = await fetch(configFile);
        if (response.ok) {
          const text = await response.text();
          // Simple YAML parsing for graphql_url
          const graphqlMatch = text.match(/graphql_url:\s*["']?([^"'\n]+)["']?/);
          const coreMatch = text.match(/core_graphql_url:\s*["']?([^"'\n]+)["']?/);
          setServiceConfig({
            graphqlUrl: graphqlMatch?.[1] || 'http://localhost:9094/devtrack/api/latest/query',
            coreGraphqlUrl: coreMatch?.[1] || 'http://localhost:8080/core/api/latest/query',
          });
        }
      } catch (error) {
        console.error('Failed to load service config:', error);
        // Default fallback
        setServiceConfig({
          graphqlUrl: 'http://localhost:9094/devtrack/api/latest/query',
          coreGraphqlUrl: 'http://localhost:8080/core/api/latest/query',
        });
      }
    };
    loadConfig();
  }, [config]);

  if (!serviceConfig) {
    return null; // Or a loading spinner
  }

  return (
    <ServiceConfigContext.Provider value={serviceConfig}>
      {children}
    </ServiceConfigContext.Provider>
  );
};

export const useServiceConfig = (): ServiceConfig => {
  const config = useContext(ServiceConfigContext);
  if (!config) {
    throw new Error('useServiceConfig must be used within ServiceConfigProvider');
  }
  return config;
};

export const useGraphQLUrl = (): string => {
  return useServiceConfig().graphqlUrl;
};

export const useCoreGraphQLUrl = (): string => {
  return useServiceConfig().coreGraphqlUrl;
};
