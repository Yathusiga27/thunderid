// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {fireEvent, render, screen, waitFor} from '@thunderid/test-utils';
import {type ReactNode, useEffect} from 'react';
import {beforeEach, describe, expect, it, vi} from 'vitest';
import ConnectionDetailPage from '../ConnectionDetailPage';

const updateMock = vi.fn().mockResolvedValue({});
const updateResetMock = vi.fn();
const refetchMock = vi.fn().mockResolvedValue({});
const deleteMock = vi.fn((_id: string, opts: {onSuccess: () => void}) => opts.onSuccess());
const navigateMock = vi.fn();
const updateMutationState = {isPending: false, isError: false};
const usagesQueryState: {
  data?: {totalResults: number | null; count: number; summary: Record<string, number>; usages: unknown[]};
  isLoading: boolean;
  isError: boolean;
  error: Error | null;
} = {
  data: {totalResults: 0, count: 0, summary: {}, usages: []},
  isLoading: false,
  isError: false,
  error: null,
};

const ATTR_CONFIG = {
  userTypeResolution: {default: 'employee'},
  userTypeAttributeMappings: [
    {userType: 'employee', attributes: [{externalAttribute: 'email', localAttribute: 'mail'}]},
  ],
};

const CONNECTION = {
  id: 'g1',
  type: 'google',
  name: 'Google',
  clientId: 'cid',
  clientSecret: '******',
  redirectUri: 'https://id.acme.io/oauth/callback/google',
  scopes: ['openid'],
  attributeConfiguration: ATTR_CONFIG,
};

const TWILIO_CONNECTION = {
  id: 'tw1',
  type: 'twilio',
  name: 'Twilio',
  accountSid: 'AC00000000000000000000000000000000',
  authToken: '******',
  senderId: '+15005550006',
};

const OIDC_CONNECTION = {
  id: 'oidc1',
  type: 'oidc',
  name: 'Acme Workforce OIDC',
  clientId: 'cid',
  clientSecret: '******',
  authorizationEndpoint: 'https://idp.example.com/authorize',
  tokenEndpoint: 'https://idp.example.com/token',
  redirectUri: 'https://id.acme.io/oauth/callback/oidc',
};

const AUTHZEN_PDP_CONNECTION = {
  id: 'pdp1',
  type: 'external-authzen-pdp',
  name: 'External AuthZEN PDP',
  endpoint: 'https://pdp.example.com/.well-known/authzen-configuration',
  timeoutMs: 500,
  retryCount: 1,
  subjectProperties: 'email department',
  subjectPropertyMappings: 'department: dept',
};

const mockParams: {type: string; id: string} = {type: 'google', id: 'g1'};
const mockConn: {data: Record<string, unknown>} = {data: CONNECTION};

vi.mock('react-router', async (importOriginal) => ({
  ...(await importOriginal<typeof import('react-router')>()),
  useNavigate: () => navigateMock,
  useParams: () => mockParams,
}));
vi.mock('@thunderid/contexts', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@thunderid/contexts')>()),
  useConfig: () => ({getGateCallbackUrl: () => 'https://id.acme.io/gate/callback'}),
  useToast: () => ({showToast: vi.fn()}),
}));
vi.mock('@thunderid/components', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@thunderid/components')>()),
  SettingsCard: ({title, children}: {title: string; children: ReactNode}) => (
    <section aria-label={title}>{children}</section>
  ),
  UnsavedChangesBar: ({onSave, saveLabel}: {onSave: () => void; saveLabel: string}) => (
    <button type="button" data-testid="save-bar" onClick={onSave}>
      {saveLabel}
    </button>
  ),
}));

vi.mock('../../api/useConnection', () => ({
  default: () => ({data: mockConn.data, isLoading: false, isError: false, refetch: refetchMock}),
}));
vi.mock('../../api/useConnectionInstances', () => ({default: () => ({data: [], isLoading: false})}));
vi.mock('../../api/useUpdateConnection', () => ({
  default: () => ({mutateAsync: updateMock, reset: updateResetMock, ...updateMutationState}),
}));
vi.mock('../../api/useDeleteConnection', () => ({default: () => ({mutate: deleteMock, isPending: false})}));
vi.mock('../../api/useGetConnectionUsages', () => ({
  default: () => usagesQueryState,
}));

vi.mock('../../components/ConnectionForm', () => ({
  default: function StubConnectionForm({
    onFieldChange,
    nameError,
  }: {
    onFieldChange: (name: string, value: string) => void;
    nameError?: string | null;
  }) {
    return (
      <div data-testid="stub-connection-form">
        <button type="button" data-testid="edit-client-id" onClick={() => onFieldChange('clientId', 'changed')}>
          edit
        </button>
        <button type="button" data-testid="edit-name" onClick={() => onFieldChange('name', 'Renamed')}>
          edit name
        </button>
        {nameError && <div data-testid="stub-name-error">{nameError}</div>}
      </div>
    );
  },
}));

vi.mock('../../components/AttributeMappingSection', () => ({
  default: function StubAttributeMappingSection({onChange}: {onChange: (c: unknown, v: boolean) => void}) {
    useEffect(() => {
      onChange(ATTR_CONFIG, true);
    }, [onChange]);
    return <div data-testid="stub-attribute-mapping" />;
  },
}));

vi.mock('../../components/SubjectMappingSection', () => ({
  default: function StubSubjectMappingSection({
    values,
    onChange,
  }: {
    values: {subjectProperties?: string; subjectPropertyMappings?: string};
    onChange: (field: 'subjectProperties' | 'subjectPropertyMappings', value: string) => void;
  }) {
    return (
      <div data-testid="stub-subject-mapping">
        <span>{values.subjectProperties}</span>
        <button
          type="button"
          data-testid="edit-subject-mapping"
          onClick={() => onChange('subjectProperties', 'email riskScore')}
        >
          edit subject mapping
        </button>
      </div>
    );
  },
}));

describe('ConnectionDetailPage', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockParams.type = 'google';
    mockParams.id = 'g1';
    mockConn.data = CONNECTION;
    updateMutationState.isPending = false;
    updateMutationState.isError = false;
    usagesQueryState.data = {totalResults: 0, count: 0, summary: {}, usages: []};
    usagesQueryState.isLoading = false;
    usagesQueryState.isError = false;
    usagesQueryState.error = null;
  });

  it('renders the general tab with quick-copy and the credentials form', () => {
    render(<ConnectionDetailPage />);
    expect(screen.getByTestId('connection-id-copy')).toBeInTheDocument();
    expect(screen.getByDisplayValue('g1')).toBeInTheDocument();
    expect(screen.getByText('Unique identifier for this connection.')).toBeInTheDocument();
    expect(screen.getByTestId('stub-connection-form')).toBeInTheDocument();
  });

  it('renders the danger-zone delete on the advanced tab', () => {
    render(<ConnectionDetailPage />);
    fireEvent.click(screen.getByTestId('connection-tab-advanced'));
    expect(screen.getByTestId('connection-delete-button')).toBeInTheDocument();
  });

  it('shows the Configured status under the name in the card style, with no redundant subtitle', () => {
    render(<ConnectionDetailPage />);
    expect(screen.getByText('Google')).toBeInTheDocument();
    expect(screen.getByText('Configured')).toBeInTheDocument();
    expect(screen.queryByText('Google connection')).not.toBeInTheDocument();
    expect(document.querySelector('.MuiChip-root')).not.toBeInTheDocument();
  });

  it('hides the save bar until a field is edited', () => {
    render(<ConnectionDetailPage />);
    expect(screen.queryByTestId('save-bar')).not.toBeInTheDocument();
    fireEvent.click(screen.getByTestId('edit-client-id'));
    expect(screen.getByTestId('save-bar')).toBeInTheDocument();
  });

  it('saves the merged payload, preserves stored attribute mappings, refetches, then clears the dirty bar', async () => {
    render(<ConnectionDetailPage />);
    fireEvent.click(screen.getByTestId('edit-client-id'));
    fireEvent.click(screen.getByTestId('save-bar'));

    expect(updateMock).toHaveBeenCalledTimes(1);
    const payload = updateMock.mock.calls[0][0] as {name: string; clientId: string; attributeConfiguration?: unknown};
    expect(payload).toMatchObject({name: 'Google', clientId: 'changed'});
    // General-tab-only edit must not wipe the stored attribute configuration.
    expect(payload.attributeConfiguration).toEqual(ATTR_CONFIG);

    await waitFor(() => expect(refetchMock).toHaveBeenCalledTimes(1));
    await waitFor(() => expect(screen.queryByTestId('save-bar')).not.toBeInTheDocument());
  });

  it('shows an inline name error on a 409 update conflict, and clears it when the name is edited', async () => {
    mockParams.type = 'oidc';
    mockParams.id = 'oidc1';
    mockConn.data = OIDC_CONNECTION;
    updateMock.mockRejectedValueOnce({response: {status: 409}});
    render(<ConnectionDetailPage />);
    fireEvent.click(screen.getByTestId('edit-client-id'));
    fireEvent.click(screen.getByTestId('save-bar'));

    await waitFor(() =>
      expect(screen.getByTestId('stub-name-error')).toHaveTextContent('A connection with this name already exists.'),
    );

    fireEvent.click(screen.getByTestId('edit-name'));
    expect(screen.queryByTestId('stub-name-error')).not.toBeInTheDocument();
  });

  it('resets a failed (settled) update mutation as soon as a field is edited', () => {
    updateMutationState.isError = true;
    render(<ConnectionDetailPage />);

    fireEvent.click(screen.getByTestId('edit-client-id'));

    expect(updateResetMock).toHaveBeenCalled();
  });

  it('does not reset a still-pending update mutation when a field is edited', () => {
    updateMutationState.isPending = true;
    render(<ConnectionDetailPage />);

    fireEvent.click(screen.getByTestId('edit-client-id'));

    expect(updateResetMock).not.toHaveBeenCalled();
  });

  it('deletes the connection and returns to the list', () => {
    render(<ConnectionDetailPage />);
    fireEvent.click(screen.getByTestId('connection-tab-advanced'));
    fireEvent.click(screen.getByTestId('connection-delete-button'));
    fireEvent.click(screen.getByTestId('connection-delete-confirm'));
    expect(deleteMock).toHaveBeenCalledWith('g1', expect.anything());
    expect(navigateMock).toHaveBeenCalledWith('/connections');
  });

  it('shows a connection usage check failure inside the delete dialog', () => {
    usagesQueryState.data = undefined;
    usagesQueryState.isError = true;
    usagesQueryState.error = new Error('raw usage failure');

    render(<ConnectionDetailPage />);
    fireEvent.click(screen.getByTestId('connection-tab-advanced'));
    fireEvent.click(screen.getByTestId('connection-delete-button'));

    expect(screen.getByText('Failed to check connection usage. Please try again.')).toBeInTheDocument();
    expect(screen.getByTestId('connection-delete-confirm')).toBeDisabled();
  });

  it('SMS vendor: hides the attribute-mapping tab and save omits attributeConfiguration', () => {
    mockParams.type = 'twilio';
    mockParams.id = 'tw1';
    mockConn.data = TWILIO_CONNECTION;
    render(<ConnectionDetailPage />);

    expect(screen.getByTestId('connection-tab-general')).toBeInTheDocument();
    expect(screen.queryByTestId('connection-tab-attributes')).not.toBeInTheDocument();

    fireEvent.click(screen.getByTestId('edit-client-id'));
    fireEvent.click(screen.getByTestId('save-bar'));

    expect(updateMock).toHaveBeenCalledTimes(1);
    const payload = updateMock.mock.calls[0][0] as Record<string, unknown>;
    expect(payload).toMatchObject({
      name: 'Twilio',
      accountSid: 'AC00000000000000000000000000000000',
      senderId: '+15005550006',
    });
    expect('attributeConfiguration' in payload).toBe(false);
  });

  it('External AuthZEN PDP: shows subject mapping in its own tab and saves it with the connection', () => {
    mockParams.type = 'external-authzen-pdp';
    mockParams.id = 'pdp1';
    mockConn.data = AUTHZEN_PDP_CONNECTION;
    render(<ConnectionDetailPage />);

    expect(screen.getByTestId('connection-tab-subject-mapping')).toBeInTheDocument();
    fireEvent.click(screen.getByTestId('connection-tab-subject-mapping'));
    expect(screen.getByTestId('stub-subject-mapping')).toHaveTextContent('email department');

    fireEvent.click(screen.getByTestId('edit-subject-mapping'));
    fireEvent.click(screen.getByTestId('save-bar'));

    expect(updateMock).toHaveBeenCalledTimes(1);
    const payload = updateMock.mock.calls[0][0] as Record<string, unknown>;
    expect(payload).toMatchObject({
      name: 'External AuthZEN PDP',
      endpoint: 'https://pdp.example.com/.well-known/authzen-configuration',
      subjectProperties: 'email riskScore',
      subjectPropertyMappings: 'department: dept',
    });
    expect('resourceType' in payload).toBe(false);
    expect('resourceId' in payload).toBe(false);
  });
});
