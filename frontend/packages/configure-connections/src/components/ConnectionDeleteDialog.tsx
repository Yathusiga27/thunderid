// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {getErrorMessage} from '@thunderid/utils';
import {
  Alert,
  Button,
  CircularProgress,
  Dialog,
  DialogActions,
  DialogContent,
  DialogContentText,
  DialogTitle,
  List,
  ListItem,
  ListItemText,
  Typography,
} from '@wso2/oxygen-ui';
import type {JSX} from 'react';
import {useTranslation} from 'react-i18next';
import useGetConnectionUsages from '../api/useGetConnectionUsages';
import type {ConnectionType} from '../models/connection';

const MAX_VISIBLE_USAGES = 5;

interface ConnectionDeleteDialogProps {
  open: boolean;
  connectionType: ConnectionType;
  connectionId: string;
  connectionName: string;
  isPending: boolean;
  /** Resolved error message from a failed delete, rendered inline. The dialog stays open on failure. */
  error?: string | null;
  onConfirm: () => void;
  onClose: () => void;
}

export default function ConnectionDeleteDialog({
  open,
  connectionType,
  connectionId,
  connectionName,
  isPending,
  error = null,
  onConfirm,
  onClose,
}: ConnectionDeleteDialogProps): JSX.Element {
  const {t} = useTranslation('connections');

  const usagesQuery = useGetConnectionUsages(connectionType, connectionId || undefined, open);
  const {data: usagesData, isLoading: isLoadingUsages, isError: isUsagesError, error: usagesError} = usagesQuery;

  const usagesKnown = usagesData !== undefined && usagesData.totalResults !== null;
  const blockingUsages = usagesData?.usages.filter((usage) => usage.behaviorOnDelete === 'restrict') ?? [];
  const hasBlockingUsages = usagesKnown && blockingUsages.length > 0;
  const visibleBlocking = blockingUsages.slice(0, MAX_VISIBLE_USAGES);
  const hiddenBlockingCount = blockingUsages.length - visibleBlocking.length;
  const usagesErrorMessage = isUsagesError
    ? getErrorMessage(
        usagesError,
        t,
        'delete.usages.error',
        'Failed to check connection usage. Please try again.',
      )
    : null;

  return (
    <Dialog open={open} onClose={isPending ? undefined : onClose} maxWidth="sm" fullWidth>
      <DialogTitle>{t('delete.title')}</DialogTitle>
      <DialogContent>
        <DialogContentText sx={{mb: 2}}>{t('delete.message', {name: connectionName})}</DialogContentText>

        {isLoadingUsages ? (
          <Alert severity="info" icon={<CircularProgress size={16} />}>
            {t('delete.usages.loading')}
          </Alert>
        ) : isUsagesError ? (
          <Alert severity="error">{usagesErrorMessage}</Alert>
        ) : hasBlockingUsages ? (
          <Alert severity="error">
            <Typography variant="body2" sx={{mb: 1}}>
              {t('delete.blocking.title')}
            </Typography>
            <List dense disablePadding>
              {visibleBlocking.map((usage) => (
                <ListItem key={usage.id} disableGutters sx={{py: 0}}>
                  <ListItemText primary={<Typography variant="body2">{usage.displayName}</Typography>} />
                </ListItem>
              ))}
              {hiddenBlockingCount > 0 && (
                <ListItem disableGutters sx={{py: 0}}>
                  <ListItemText
                    primary={
                      <Typography variant="body2" color="text.secondary">
                        {t('delete.usages.more', {count: hiddenBlockingCount})}
                      </Typography>
                    }
                  />
                </ListItem>
              )}
            </List>
          </Alert>
        ) : null}

        {error && (
          <Alert severity="error" sx={{mt: 2}}>
            {error}
          </Alert>
        )}
      </DialogContent>
      <DialogActions>
        <Button onClick={onClose} disabled={isPending}>
          {t('common:actions.cancel')}
        </Button>
        <Button
          onClick={onConfirm}
          color="error"
          variant="contained"
          disabled={isPending || isLoadingUsages || isUsagesError || hasBlockingUsages}
          data-testid="connection-delete-confirm"
        >
          {t('common:actions.delete')}
        </Button>
      </DialogActions>
    </Dialog>
  );
}
