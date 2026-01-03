import { useCallback } from 'react';
import { useServiceConfig } from '../../ServiceConfigContext';

interface GraphQLResponse<T> {
  data?: T;
  errors?: Array<{ message: string }>;
}

export function useGraphQL() {
  const { graphqlUrl, coreGraphqlUrl } = useServiceConfig();

  const getToken = useCallback((): string | null => {
    try {
      return sessionStorage.getItem('csd_access_token');
    } catch {
      return null;
    }
  }, []);

  const request = useCallback(async <T = unknown>(
    query: string,
    variables?: Record<string, unknown>
  ): Promise<T> => {
    const token = getToken();
    const headers: Record<string, string> = {
      'Content-Type': 'application/json',
    };
    if (token) {
      headers['Authorization'] = `Bearer ${token}`;
    }

    const response = await fetch(graphqlUrl, {
      method: 'POST',
      headers,
      body: JSON.stringify({ query, variables }),
    });

    if (!response.ok) {
      throw new Error(`HTTP error: ${response.status}`);
    }

    const result: GraphQLResponse<T> = await response.json();
    if (result.errors?.length) {
      throw new Error(result.errors[0].message);
    }

    return result.data as T;
  }, [graphqlUrl, getToken]);

  const coreRequest = useCallback(async <T = unknown>(
    query: string,
    variables?: Record<string, unknown>
  ): Promise<T> => {
    const token = getToken();
    const headers: Record<string, string> = {
      'Content-Type': 'application/json',
    };
    if (token) {
      headers['Authorization'] = `Bearer ${token}`;
    }

    const response = await fetch(coreGraphqlUrl, {
      method: 'POST',
      headers,
      body: JSON.stringify({ query, variables }),
    });

    if (!response.ok) {
      throw new Error(`HTTP error: ${response.status}`);
    }

    const result: GraphQLResponse<T> = await response.json();
    if (result.errors?.length) {
      throw new Error(result.errors[0].message);
    }

    return result.data as T;
  }, [coreGraphqlUrl, getToken]);

  return { request, coreRequest };
}
