// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {SettingsCard} from '@thunderid/components';
import {FormControl, FormHelperText, FormLabel, MenuItem, Select, Stack, TextField} from '@wso2/oxygen-ui';
import type {JSX} from 'react';
import {useTranslation} from 'react-i18next';
import useExternalAuthZENPDPConnections from '../../api/useExternalAuthZENPDPConnections';
import type {ResourceServer} from '../../models/resource-server';

interface AdvancedTabProps {
  resourceServer: ResourceServer;
  identifier: string;
  authorizationEngine: string;
  externalPDPConnectionId: string;
  onIdentifierChange: (value: string) => void;
  onAuthorizationEngineChange: (value: string) => void;
  onExternalPDPConnectionChange: (value: string) => void;
}

export default function AdvancedTab({
  resourceServer,
  identifier,
  authorizationEngine,
  externalPDPConnectionId,
  onIdentifierChange,
  onAuthorizationEngineChange,
  onExternalPDPConnectionChange,
}: AdvancedTabProps): JSX.Element {
  const {t} = useTranslation();
  const externalPDPConnections = useExternalAuthZENPDPConnections();
  const externalPDPOptionPrefix = 'external_authzen_pdp:';
  const authorizationEngineValue =
    authorizationEngine === 'external_authzen_pdp' && externalPDPConnectionId
      ? `${externalPDPOptionPrefix}${externalPDPConnectionId}`
      : authorizationEngine ?? 'rbac';
  const hasSelectedExternalPDP =
    authorizationEngine === 'external_authzen_pdp' &&
    externalPDPConnectionId &&
    !(externalPDPConnections.data ?? []).some((connection) => connection.id === externalPDPConnectionId);

  return (
    <Stack spacing={3}>
      <SettingsCard
        title={t('resourceServers:edit.advanced.identifier.title', 'Configurations')}
        description={
          resourceServer.type === 'MCP'
            ? t(
                'resourceServers:edit.advanced.identifier.descriptionMcp',
                'Configuration settings for this MCP server.',
              )
            : t(
                'resourceServers:edit.advanced.identifier.description',
                'Configuration settings for this resource server.',
              )
        }
      >
        <FormControl fullWidth>
          <FormLabel htmlFor="resource-server-identifier">
            {t('resourceServers:edit.advanced.identifier.label', 'Identifier (Audience)')}
          </FormLabel>
          <TextField
            id="resource-server-identifier"
            value={identifier}
            onChange={(e) => onIdentifierChange(e.target.value)}
            fullWidth
            size="small"
            placeholder={
              resourceServer.type === 'MCP'
                ? t('resourceServers:edit.advanced.identifier.placeholderMcp', 'https://mcp.example.com')
                : t('resourceServers:edit.advanced.identifier.placeholder', 'https://api.example.com')
            }
            helperText={
              resourceServer.type === 'MCP'
                ? t(
                    'resourceServers:edit.advanced.identifier.hintMcp',
                    'A unique value that identifies this MCP server. When set as an URI, enables RFC 8707 resource indicator support in OAuth2 authorization requests.',
                  )
                : t(
                    'resourceServers:edit.advanced.identifier.hint',
                    'A unique value that identifies this resource server. When set as an URI, enables RFC 8707 resource indicator support in OAuth2 authorization requests.',
                  )
            }
            disabled={resourceServer.isReadOnly}
          />
        </FormControl>
        <FormControl fullWidth sx={{mt: 3}}>
          <FormLabel htmlFor="resource-server-authorization-engine">
            {t('resourceServers:edit.advanced.authorizationEngine.label', 'Authorization engine')}
          </FormLabel>
          <Select
            id="resource-server-authorization-engine"
            value={authorizationEngineValue}
            onChange={(event) => {
              const value = event.target.value;
              if (value.startsWith(externalPDPOptionPrefix)) {
                onAuthorizationEngineChange('external_authzen_pdp');
                onExternalPDPConnectionChange(value.slice(externalPDPOptionPrefix.length));
                return;
              }
              onAuthorizationEngineChange(value);
              onExternalPDPConnectionChange('');
            }}
            size="small"
            disabled={Boolean(resourceServer.isReadOnly) || externalPDPConnections.isLoading}
          >
            <MenuItem value="rbac">
              {t('resourceServers:edit.advanced.authorizationEngine.option.rbac', 'RBAC')}
            </MenuItem>
            {hasSelectedExternalPDP && (
              <MenuItem value={`${externalPDPOptionPrefix}${externalPDPConnectionId}`}>
                {t('resourceServers:edit.advanced.authorizationEngine.option.selectedExternalPDP', 'Selected PDP')}
              </MenuItem>
            )}
            {(externalPDPConnections.data ?? []).map((connection) => (
              <MenuItem key={connection.id} value={`${externalPDPOptionPrefix}${connection.id}`}>
                {connection.name}
              </MenuItem>
            ))}
          </Select>
          <FormHelperText>
            {externalPDPConnections.error
              ? t(
                  'resourceServers:edit.advanced.authorizationEngine.loadExternalPDPError',
                  'Failed to load external PDP connections.',
                )
              : t(
                  'resourceServers:edit.advanced.authorizationEngine.hint',
                  'Choose RBAC or a configured external PDP connection for this resource server.',
                )}
          </FormHelperText>
        </FormControl>
      </SettingsCard>
    </Stack>
  );
}
