import React, { useEffect, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import {
  CSDLayoutPage,
  CSDPaper,
  CSDTypography,
  CSDStack,
  CSDBox,
  CSDIcon,
  CSDButton,
  CSDTable,
  CSDTableHead,
  CSDTableBody,
  CSDTableRow,
  CSDTableCell,
  CSDChip,
} from 'csd_core/UI';
import { useTranslation } from '../../../translations/TranslationContext';
import { useGraphQL } from '../../../shared/hooks';
import { formatRelativeTime } from '../../../shared/utils';
import type { SessionSummary } from '../../../types';

interface SessionsData {
  sessions: SessionSummary[];
  sessionsCount: number;
}

const SessionsPage: React.FC = () => {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const { request } = useGraphQL();
  const [data, setData] = useState<SessionsData | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    const loadData = async () => {
      try {
        setLoading(true);
        const result = await request<SessionsData>(`
          query {
            sessions { id name projectId projectName workDir state messageCount createdAt lastActiveAt }
            sessionsCount
          }
        `);
        setData(result);
        setError(null);
      } catch (err) {
        setError(err instanceof Error ? err.message : 'Failed to load sessions');
      } finally {
        setLoading(false);
      }
    };
    loadData();
  }, [request]);

  const getStateColor = (state: string): 'success' | 'warning' | 'error' | 'default' => {
    switch (state) {
      case 'running': return 'success';
      case 'waiting': return 'warning';
      case 'error': return 'error';
      default: return 'default';
    }
  };

  if (loading) {
    return (
      <CSDLayoutPage title={t('sessions.title')}>
        <CSDBox sx={{ p: 3, textAlign: 'center' }}>
          <CSDTypography>{t('common.loading')}</CSDTypography>
        </CSDBox>
      </CSDLayoutPage>
    );
  }

  if (error) {
    return (
      <CSDLayoutPage title={t('sessions.title')}>
        <CSDBox sx={{ p: 3, textAlign: 'center' }}>
          <CSDTypography color="error">{error}</CSDTypography>
          <CSDButton onClick={() => window.location.reload()} sx={{ mt: 2 }}>
            {t('common.retry')}
          </CSDButton>
        </CSDBox>
      </CSDLayoutPage>
    );
  }

  return (
    <CSDLayoutPage
      title={t('sessions.title')}
      actions={
        <CSDButton
          variant="contained"
          startIcon={<CSDIcon>add</CSDIcon>}
        >
          {t('sessions.newSession')}
        </CSDButton>
      }
    >
      <CSDPaper>
        <CSDTable>
          <CSDTableHead>
            <CSDTableRow>
              <CSDTableCell>{t('sessions.name')}</CSDTableCell>
              <CSDTableCell>{t('sessions.project')}</CSDTableCell>
              <CSDTableCell>{t('sessions.state')}</CSDTableCell>
              <CSDTableCell>{t('sessions.messages')}</CSDTableCell>
              <CSDTableCell>{t('sessions.lastActive')}</CSDTableCell>
              <CSDTableCell align="right">{t('common.actions')}</CSDTableCell>
            </CSDTableRow>
          </CSDTableHead>
          <CSDTableBody>
            {data?.sessions?.map((session) => (
              <CSDTableRow
                key={session.id}
                hover
                sx={{ cursor: 'pointer' }}
                onClick={() => navigate(`/devtrack/sessions/${session.id}`)}
              >
                <CSDTableCell>
                  <CSDStack direction="row" alignItems="center" spacing={1}>
                    <CSDIcon color="secondary">chat</CSDIcon>
                    <CSDTypography fontWeight="medium">{session.name}</CSDTypography>
                  </CSDStack>
                </CSDTableCell>
                <CSDTableCell>
                  <CSDTypography variant="body2">{session.projectName || '-'}</CSDTypography>
                </CSDTableCell>
                <CSDTableCell>
                  <CSDChip
                    size="small"
                    label={t(`sessions.${session.state}`)}
                    color={getStateColor(session.state)}
                  />
                </CSDTableCell>
                <CSDTableCell>{session.messageCount}</CSDTableCell>
                <CSDTableCell>
                  <CSDTypography variant="body2" color="text.secondary">
                    {formatRelativeTime(session.lastActiveAt)}
                  </CSDTypography>
                </CSDTableCell>
                <CSDTableCell align="right">
                  <CSDStack direction="row" spacing={1} justifyContent="flex-end">
                    <CSDButton
                      size="small"
                      variant="outlined"
                      startIcon={<CSDIcon>terminal</CSDIcon>}
                      onClick={(e) => {
                        e.stopPropagation();
                        navigate(`/devtrack/sessions/${session.id}?terminal=true`);
                      }}
                    >
                      {t('sessions.openTerminal')}
                    </CSDButton>
                  </CSDStack>
                </CSDTableCell>
              </CSDTableRow>
            ))}
          </CSDTableBody>
        </CSDTable>
        {(!data?.sessions || data.sessions.length === 0) && (
          <CSDBox sx={{ p: 4, textAlign: 'center' }}>
            <CSDTypography color="text.secondary">{t('sessions.noSessions')}</CSDTypography>
          </CSDBox>
        )}
      </CSDPaper>
    </CSDLayoutPage>
  );
};

export default SessionsPage;
