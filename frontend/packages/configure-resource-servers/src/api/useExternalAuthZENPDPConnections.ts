// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useQuery, type UseQueryResult} from '@tanstack/react-query';
import {useConfig} from '@thunderid/contexts';
import {useThunderID} from '@thunderid/react';
import ResourceServerQueryKeys from '../constants/resource-server-query-keys';

export interface ExternalAuthZENPDPConnectionSummary {
  id: string;
  name: string;
  description?: string;
}

export default function useExternalAuthZENPDPConnections(): UseQueryResult<ExternalAuthZENPDPConnectionSummary[]> {
  const {http} = useThunderID();
  const {getServerUrl} = useConfig();

  return useQuery<ExternalAuthZENPDPConnectionSummary[]>({
    queryKey: [ResourceServerQueryKeys.EXTERNAL_AUTHZEN_PDP_CONNECTIONS],
    queryFn: async (): Promise<ExternalAuthZENPDPConnectionSummary[]> => {
      const serverUrl = getServerUrl();

      const response: {data: ExternalAuthZENPDPConnectionSummary[]} = await http.request({
        url: `${serverUrl}/connections/external-authzen-pdp`,
        method: 'GET',
      } as unknown as Parameters<typeof http.request>[0]);

      return response.data;
    },
  });
}
