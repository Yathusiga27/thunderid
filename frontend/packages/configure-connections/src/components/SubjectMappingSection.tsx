// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {SettingsCard} from '@thunderid/components';
import {useGetUserType, useGetUserTypes} from '@thunderid/configure-user-types';
import {
  Autocomplete,
  Box,
  Button,
  FormControl,
  FormLabel,
  IconButton,
  MenuItem,
  Select,
  Stack,
  TextField,
  Typography,
} from '@wso2/oxygen-ui';
import {Plus, Route, Trash2} from '@wso2/oxygen-ui-icons-react';
import {type JSX, useMemo, useState} from 'react';
import {useTranslation} from 'react-i18next';
import {flattenUserTypeAttributes} from '../utils/attributeConfiguration';
import {parseKeyValuePairs, sanitizeKeyValuePart} from '../utils/keyValuePairs';

export interface SubjectMappingValues {
  subjectProperties?: string;
  subjectPropertyMappings?: string;
  subjectAttributeMappings?: SubjectAttributeMappingGroupValue[];
}

interface SubjectMappingSectionProps {
  values: SubjectMappingValues;
  onChange: <K extends keyof SubjectMappingValues>(field: K, value: NonNullable<SubjectMappingValues[K]>) => void;
}

export interface SubjectAttributeMappingGroupValue {
  userType: string;
  attributes: SubjectAttributeMappingValue[];
}

export interface SubjectAttributeMappingValue {
  attribute: string;
  pdpAttribute?: string;
}

interface SubjectAttributeRow {
  key: number;
  attribute: string;
  pdpAttribute: string;
}

interface SubjectMappingGroup {
  key: number;
  userType: string;
  rows: SubjectAttributeRow[];
}

interface SubjectRowsState {
  groups: SubjectMappingGroup[];
  seq: number;
  syncedProperties: string;
  syncedMappings: string;
  syncedAttributeMappings: string;
}

const BUILT_IN_OPTIONAL_SUBJECT_ATTRIBUTES = ['groups', 'ouId'];
const DEFAULT_SUBJECT_FIELDS = ['Subject ID'];

function parseSubjectProperties(value: string | undefined): string[] {
  return (value ?? '')
    .split(/\s+/)
    .map((attribute) => attribute.trim())
    .filter((attribute) => attribute !== '');
}

function uniqueValues(values: string[]): string[] {
  return values.filter((value, index, all) => value.trim() !== '' && all.indexOf(value) === index);
}

function buildRowsFromValues(
  subjectProperties: string | undefined,
  subjectPropertyMappings: string | undefined,
  fromSeq: number,
): {rows: SubjectAttributeRow[]; seq: number} {
  const mappings = parseKeyValuePairs(subjectPropertyMappings ?? '');
  const mappedByAttribute = new Map(mappings.map((mapping) => [mapping.name, mapping.value]));
  const attributes = uniqueValues([
    ...parseSubjectProperties(subjectProperties),
    ...mappings.map((mapping) => mapping.name).filter((name) => name.trim() !== ''),
  ]);
  const rows =
    attributes.length > 0
      ? attributes.map((attribute, index) => ({
          key: fromSeq + index + 1,
          attribute,
          pdpAttribute: mappedByAttribute.get(attribute) ?? '',
        }))
      : [{key: fromSeq + 1, attribute: '', pdpAttribute: ''}];
  return {rows, seq: fromSeq + rows.length};
}

function buildSubjectRows(
  subjectProperties: string | undefined,
  subjectPropertyMappings: string | undefined,
  subjectAttributeMappings: SubjectAttributeMappingGroupValue[] | undefined,
  defaultUserType: string,
  fromSeq: number,
): SubjectRowsState {
  if (subjectAttributeMappings && subjectAttributeMappings.length > 0) {
    let seq = fromSeq;
    const groups = subjectAttributeMappings.map((group) => {
      const rows =
        group.attributes.length > 0
          ? group.attributes.map((attribute) => {
              seq += 1;
              return {
                key: seq,
                attribute: attribute.attribute,
                pdpAttribute: attribute.pdpAttribute ?? '',
              };
            })
          : [{key: seq + 1, attribute: '', pdpAttribute: ''}];
      seq += group.attributes.length > 0 ? 0 : 1;
      seq += 1;
      return {key: seq, userType: group.userType, rows};
    });
    return {
      groups,
      seq,
      syncedProperties: subjectProperties ?? '',
      syncedMappings: subjectPropertyMappings ?? '',
      syncedAttributeMappings: canonicalSubjectAttributeMappings(subjectAttributeMappings),
    };
  }
  const {rows, seq} = buildRowsFromValues(subjectProperties, subjectPropertyMappings, fromSeq);
  return {
    groups: [{key: seq + 1, userType: defaultUserType, rows}],
    seq: seq + 1,
    syncedProperties: subjectProperties ?? '',
    syncedMappings: subjectPropertyMappings ?? '',
    syncedAttributeMappings: canonicalSubjectAttributeMappings(subjectAttributeMappings),
  };
}

function serializeSubjectProperties(groups: SubjectMappingGroup[]): string {
  return uniqueValues(groups.flatMap((group) => group.rows.map((row) => row.attribute.trim()))).join(' ');
}

function serializeSubjectMappings(groups: SubjectMappingGroup[]): string {
  const mappings = new Map<string, string>();
  for (const group of groups) {
    for (const row of group.rows) {
      const attribute = row.attribute.trim();
      const pdpAttribute = row.pdpAttribute.trim();
      if (attribute !== '' && pdpAttribute !== '') {
        mappings.set(attribute, pdpAttribute);
      }
    }
  }
  return [...mappings.entries()].map(([attribute, pdpAttribute]) => `${attribute}: ${pdpAttribute}`).join(', ');
}

function serializeSubjectAttributeMappings(groups: SubjectMappingGroup[]): SubjectAttributeMappingGroupValue[] {
  return groups
    .map((group) => ({
      userType: group.userType,
      attributes: group.rows
        .map((row) => ({
          attribute: row.attribute.trim(),
          pdpAttribute: row.pdpAttribute.trim(),
        }))
        .filter((row) => row.attribute !== '')
        .map((row) => ({
          attribute: row.attribute,
          ...(row.pdpAttribute !== '' ? {pdpAttribute: row.pdpAttribute} : {}),
        })),
    }))
    .filter((group) => group.userType.trim() !== '' || group.attributes.length > 0);
}

function canonicalSubjectAttributeMappings(
  subjectAttributeMappings: SubjectAttributeMappingGroupValue[] | undefined,
): string {
  return JSON.stringify(subjectAttributeMappings ?? []);
}

function withStoredOption(options: string[], stored: string): string[] {
  return stored.trim() !== '' && !options.includes(stored) ? [...options, stored] : options;
}

function hasUnusedUserType(groups: SubjectMappingGroup[], userTypeNames: string[]): boolean {
  const used = new Set(groups.map((group) => group.userType).filter((userType) => userType.trim() !== ''));
  return userTypeNames.some((name) => !used.has(name));
}

function SubjectMappingGroupEditor({
  group,
  userTypeNames,
  otherUsedUserTypes,
  userTypeIdByName,
  canRemove,
  onUserTypeChange,
  onAddRow,
  onRemoveRow,
  onUpdateRow,
  onRemoveGroup,
}: {
  group: SubjectMappingGroup;
  userTypeNames: string[];
  otherUsedUserTypes: string[];
  userTypeIdByName: Map<string, string>;
  canRemove: boolean;
  onUserTypeChange: (userType: string) => void;
  onAddRow: () => void;
  onRemoveRow: (rowKey: number) => void;
  onUpdateRow: (rowKey: number, part: 'attribute' | 'pdpAttribute', value: string) => void;
  onRemoveGroup: () => void;
}): JSX.Element {
  const {t} = useTranslation('connections');
  const userTypeDetail = useGetUserType(userTypeIdByName.get(group.userType));
  const userAttributes = useMemo(
    () => flattenUserTypeAttributes(userTypeDetail.data?.schema),
    [userTypeDetail.data?.schema],
  );
  const subjectAttributeOptions = useMemo(
    () => [...new Set([...BUILT_IN_OPTIONAL_SUBJECT_ATTRIBUTES, ...userAttributes])].sort(),
    [userAttributes],
  );
  const userTypeOptions = useMemo(() => {
    const usedElsewhere = new Set(otherUsedUserTypes);
    return withStoredOption(
      userTypeNames.filter((name) => !usedElsewhere.has(name)),
      group.userType,
    );
  }, [group.userType, otherUsedUserTypes, userTypeNames]);
  const singleUserType = userTypeNames.length === 1;
  const lastRow = group.rows[group.rows.length - 1];
  const lastRowIsEmpty = lastRow?.attribute.trim() === '' && lastRow?.pdpAttribute.trim() === '';

  return (
    <Box
      sx={{border: '1px solid', borderColor: 'divider', borderRadius: 2, p: 2}}
      data-testid={`subject-mapping-group-${group.key}`}
    >
      {(!singleUserType || canRemove) && (
        <Stack direction="row" spacing={2} alignItems="flex-end" sx={{mb: 2}}>
          {!singleUserType && (
            <FormControl sx={{minWidth: 220}}>
              <FormLabel sx={{mb: 0.75}} htmlFor={`subject-mapping-group-user-type-${group.key}`}>
                {t('subjectMapping.attributes.userType.label', 'User type')}
              </FormLabel>
              <Select
                id={`subject-mapping-group-user-type-${group.key}`}
                displayEmpty
                value={group.userType}
                onChange={(event) => onUserTypeChange(event.target.value)}
                renderValue={(value) =>
                  value ? value : t('subjectMapping.attributes.userType.placeholder', 'Select a user type')
                }
                data-testid={`subject-mapping-group-user-type-select-${group.key}`}
              >
                {userTypeOptions.map((name) => (
                  <MenuItem key={name} value={name}>
                    {name}
                  </MenuItem>
                ))}
              </Select>
            </FormControl>
          )}
          <Box sx={{flex: 1}} />
          {canRemove && (
            <Button
              variant="text"
              color="error"
              size="small"
              startIcon={<Trash2 size={16} />}
              onClick={onRemoveGroup}
              data-testid={`subject-mapping-group-remove-${group.key}`}
            >
              {t('subjectMapping.mappings.remove', 'Remove')}
            </Button>
          )}
        </Stack>
      )}

      <Stack direction="column" spacing={1.5}>
        <Stack direction="row" spacing={1.5}>
          <Typography variant="caption" color="text.secondary" fontWeight={600} sx={{flex: 1}}>
            {t('subjectMapping.mappings.thunderIdAttribute', 'ThunderID Attribute')}
          </Typography>
          <Typography variant="caption" color="text.secondary" fontWeight={600} sx={{flex: 1}}>
            {t('subjectMapping.mappings.pdpAttributeOptional', 'PDP Attribute (optional)')}
          </Typography>
          <Box sx={{width: 40}} />
        </Stack>

        {group.rows.map((row, index) => {
          const isOnlyEmptyRow =
            group.rows.length === 1 && row.attribute.trim() === '' && row.pdpAttribute.trim() === '';
          return (
            <Stack key={row.key} direction="row" spacing={1.5} alignItems="center">
              <Autocomplete
                fullWidth
                freeSolo
                options={subjectAttributeOptions}
                inputValue={row.attribute}
                onInputChange={(_event, nextValue) => onUpdateRow(row.key, 'attribute', nextValue)}
                renderInput={(params) => (
                  <TextField
                    {...params}
                    id={`subject-mapping-attribute-${group.key}-${index + 1}`}
                    placeholder={t('subjectMapping.mappings.thunderIdPlaceholder', 'e.g. email')}
                    inputProps={{
                      ...params.inputProps,
                      'aria-label': t('subjectMapping.mappings.thunderIdAttribute', 'ThunderID Attribute'),
                    }}
                  />
                )}
              />
              <TextField
                fullWidth
                id={`subject-mapping-pdp-attribute-${group.key}-${index + 1}`}
                value={row.pdpAttribute}
                onChange={(event) => onUpdateRow(row.key, 'pdpAttribute', event.target.value)}
                slotProps={{
                  input: {
                    'aria-label': t('subjectMapping.mappings.pdpAttribute', 'PDP Attribute'),
                  },
                }}
              />
              {isOnlyEmptyRow ? (
                <Box sx={{width: 40}} />
              ) : (
                <IconButton
                  onClick={() => onRemoveRow(row.key)}
                  aria-label={t('form.keyValue.remove', 'Remove')}
                  data-testid={`subject-mapping-remove-${group.key}-${index + 1}`}
                >
                  <Trash2 size={16} />
                </IconButton>
              )}
            </Stack>
          );
        })}

        <Box>
          <Button
            variant="text"
            size="small"
            startIcon={<Plus size={16} />}
            onClick={onAddRow}
            disabled={lastRowIsEmpty}
            data-testid={`subject-mapping-add-${group.key}`}
          >
            {t('subjectMapping.mappings.add', 'Add Mapping')}
          </Button>
        </Box>
      </Stack>
    </Box>
  );
}

export default function SubjectMappingSection({values, onChange}: SubjectMappingSectionProps): JSX.Element {
  const {t} = useTranslation('connections');
  const userTypesQuery = useGetUserTypes();
  const userTypeList = useMemo(() => userTypesQuery.data?.types ?? [], [userTypesQuery.data]);
  const userTypeNames = useMemo(() => userTypeList.map((type) => type.name), [userTypeList]);
  const userTypeIdByName = useMemo(() => new Map(userTypeList.map((type) => [type.name, type.id])), [userTypeList]);
  const defaultUserType = userTypeList[0]?.name ?? '';
  const [state, setState] = useState<SubjectRowsState>(() =>
    buildSubjectRows(
      values.subjectProperties,
      values.subjectPropertyMappings,
      values.subjectAttributeMappings,
      defaultUserType,
      0,
    ),
  );

  if (
    values.subjectProperties !== state.syncedProperties ||
    (values.subjectPropertyMappings ?? '') !== state.syncedMappings ||
    canonicalSubjectAttributeMappings(values.subjectAttributeMappings) !== state.syncedAttributeMappings
  ) {
    setState(
      buildSubjectRows(
        values.subjectProperties,
        values.subjectPropertyMappings,
        values.subjectAttributeMappings,
        defaultUserType,
        state.seq,
      ),
    );
  }

  if (
    defaultUserType !== '' &&
    state.groups.length === 1 &&
    state.groups[0].userType === '' &&
    state.syncedProperties === (values.subjectProperties ?? '') &&
    state.syncedMappings === (values.subjectPropertyMappings ?? '') &&
    state.syncedAttributeMappings === canonicalSubjectAttributeMappings(values.subjectAttributeMappings)
  ) {
    setState((prev) => ({
      ...prev,
      groups: prev.groups.map((group, index) => (index === 0 ? {...group, userType: defaultUserType} : group)),
    }));
  }

  const iconBox = (icon: JSX.Element): JSX.Element => (
    <Box
      sx={{
        width: 30,
        height: 30,
        borderRadius: 1.5,
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
        bgcolor: 'action.hover',
        color: 'primary.main',
      }}
    >
      {icon}
    </Box>
  );

  const commit = (groups: SubjectMappingGroup[], seq: number): void => {
    const subjectProperties = serializeSubjectProperties(groups);
    const subjectPropertyMappings = serializeSubjectMappings(groups);
    const subjectAttributeMappings = serializeSubjectAttributeMappings(groups);
    setState({
      groups,
      seq,
      syncedProperties: subjectProperties,
      syncedMappings: subjectPropertyMappings,
      syncedAttributeMappings: canonicalSubjectAttributeMappings(subjectAttributeMappings),
    });
    onChange('subjectProperties', subjectProperties);
    onChange('subjectPropertyMappings', subjectPropertyMappings);
    onChange('subjectAttributeMappings', subjectAttributeMappings);
  };

  const updateGroupType = (groupKey: number, userType: string): void => {
    const groups = state.groups.map((group) => (group.key === groupKey ? {...group, userType} : group));
    commit(groups, state.seq);
  };

  const addGroup = (): void =>
    setState((prev) => ({
      ...prev,
      groups: [
        ...prev.groups,
        {key: prev.seq + 1, userType: '', rows: [{key: prev.seq + 2, attribute: '', pdpAttribute: ''}]},
      ],
      seq: prev.seq + 2,
    }));

  const removeGroup = (groupKey: number): void => {
    const groups = state.groups.filter((group) => group.key !== groupKey);
    commit(groups.length > 0 ? groups : [{key: state.seq + 1, userType: '', rows: []}], state.seq + 1);
  };

  const addRow = (groupKey: number): void =>
    setState((prev) => ({
      ...prev,
      groups: prev.groups.map((group) =>
        group.key === groupKey
          ? {...group, rows: [...group.rows, {key: prev.seq + 1, attribute: '', pdpAttribute: ''}]}
          : group,
      ),
      seq: prev.seq + 1,
    }));

  const removeRow = (groupKey: number, rowKey: number): void => {
    const groups = state.groups.map((group) => {
      if (group.key !== groupKey) {
        return group;
      }
      const rows = group.rows.filter((row) => row.key !== rowKey);
      return {
        ...group,
        rows: rows.length > 0 ? rows : [{key: state.seq + 1, attribute: '', pdpAttribute: ''}],
      };
    });
    commit(groups, state.seq + 1);
  };

  const updateRow = (groupKey: number, rowKey: number, part: 'attribute' | 'pdpAttribute', value: string): void => {
    const groups = state.groups.map((group) =>
      group.key === groupKey
        ? {
            ...group,
            rows: group.rows.map((row) =>
              row.key === rowKey
                ? {...row, [part]: sanitizeKeyValuePart(value, part === 'attribute' ? 'name' : 'value')}
                : row,
            ),
          }
        : group,
    );
    commit(groups, state.seq);
  };

  const selectedAttributes = uniqueValues([
    ...DEFAULT_SUBJECT_FIELDS,
    ...state.groups.flatMap((group) => group.rows.map((row) => row.attribute.trim())),
  ]);
  const showAddUserType = hasUnusedUserType(state.groups, userTypeNames);

  return (
    <Stack direction="column" spacing={3} data-testid="subject-mapping-section">
      <SettingsCard
        title={t('subjectMapping.attributes.title', 'Subject attribute mapping')}
        description={t(
          'subjectMapping.attributes.description',
          'Choose the additional user attributes this PDP needs and optionally rename them for the AuthZEN request.',
        )}
        titleIcon={iconBox(<Route size={16} />)}
      >
        <Stack direction="column" spacing={3}>
          <Stack direction="column" spacing={1}>
            <Typography variant="caption" color="text.secondary" fontWeight={600}>
              {t('subjectMapping.selected.title', 'Selected attributes')}
            </Typography>
            <Stack direction="row" spacing={1} flexWrap="wrap" useFlexGap>
              {selectedAttributes.map((field) => (
                <Box
                  key={field}
                  sx={{
                    px: 1.25,
                    py: 0.5,
                    borderRadius: 999,
                    bgcolor: 'action.hover',
                    color: 'text.secondary',
                  }}
                >
                  <Typography variant="caption">{field}</Typography>
                </Box>
              ))}
            </Stack>
          </Stack>

          <Box>
            <FormLabel>{t('subjectMapping.mappings.label', 'Additional attributes')}</FormLabel>
            <Stack direction="column" spacing={2} sx={{mt: 1}}>
              {state.groups.map((group) => (
                <SubjectMappingGroupEditor
                  key={group.key}
                  group={group}
                  userTypeNames={userTypeNames}
                  otherUsedUserTypes={state.groups
                    .filter((other) => other.key !== group.key)
                    .map((other) => other.userType)
                    .filter((userType) => userType.trim() !== '')}
                  userTypeIdByName={userTypeIdByName}
                  canRemove={state.groups.length > 1}
                  onUserTypeChange={(userType) => updateGroupType(group.key, userType)}
                  onAddRow={() => addRow(group.key)}
                  onRemoveRow={(rowKey) => removeRow(group.key, rowKey)}
                  onUpdateRow={(rowKey, part, value) => updateRow(group.key, rowKey, part, value)}
                  onRemoveGroup={() => removeGroup(group.key)}
                />
              ))}

              {showAddUserType && (
                <Box>
                  <Button
                    variant="text"
                    color="primary"
                    size="small"
                    startIcon={<Plus size={16} />}
                    onClick={addGroup}
                    data-testid="subject-mapping-add-user-type"
                  >
                    {t('subjectMapping.mappings.addUserType', 'Add User Type')}
                  </Button>
                </Box>
              )}

              <Typography variant="caption" color="text.secondary">
                {t(
                  'subjectMapping.mappings.hint',
                  'Select extra user attributes to include in the PDP request. The PDP attribute is optional and is only needed when the PDP expects a different name.',
                )}
              </Typography>
            </Stack>
          </Box>
        </Stack>
      </SettingsCard>
    </Stack>
  );
}
