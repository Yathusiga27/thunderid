// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useMutation, useQueryClient, type UseMutationResult} from '@tanstack/react-query';
import {useConfig, useToast} from '@thunderid/contexts';
import {useThunderID} from '@thunderid/react';
import {useTranslation} from 'react-i18next';
import ConnectionQueryKeys from '../constants/query-keys';
import type {ConnectionType} from '../models/connection';

/**
 * Delete a connection instance (DELETE /connections/{type}/{id}).
 *
 * Failures are not toasted here — the caller surfaces them inline in the confirm dialog it
 * passes `onConfirm`/`isPending` to, keeping the dialog open so the user can see why it failed.
 */
export default function useDeleteConnection(type: ConnectionType): UseMutationResult<void, Error, string> {
  const {http} = useThunderID();
  const {getServerUrl} = useConfig();
  const queryClient: ReturnType<typeof useQueryClient> = useQueryClient();
  const {t} = useTranslation('connections');
  const {showToast} = useToast();

  return useMutation<void, Error, string>({
    mutationFn: async (id: string): Promise<void> => {
      const serverUrl: string = getServerUrl();
      await http.request({
        url: `${serverUrl}/connections/${type}/${id}`,
        method: 'DELETE',
        headers: {
          'Content-Type': 'application/json',
        },
      } as unknown as Parameters<typeof http.request>[0]);
    },
    onSuccess: () => {
      queryClient.invalidateQueries({queryKey: [ConnectionQueryKeys.CONNECTIONS]}).catch(() => {
        // Ignore invalidation errors
      });
      queryClient.invalidateQueries({queryKey: [ConnectionQueryKeys.CONNECTION_INSTANCES, type]}).catch(() => {
        // Ignore invalidation errors
      });
      showToast(t('delete.success'), 'success');
    },
  });
}
