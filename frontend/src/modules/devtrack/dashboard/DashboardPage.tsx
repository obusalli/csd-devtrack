import React, { useEffect, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import {
  CSDLayoutPage,
  CSDStatsGrid,
  CSDStatCard,
  CSDPaper,
  CSDTypography,
  CSDStack,
  CSDBox,
  CSDIcon,
  CSDButton,
} from 'csd_core/UI';
import { useTranslation } from '../../../translations/TranslationContext';
import { useGraphQL } from '../../../shared/hooks';
import type { Project, SessionSummary } from '../../../types';

interface DashboardData {
  projects: Project[];
  projectsCount: number;
  sessions: SessionSummary[];
  sessionsCount: number;
}

const DashboardPage: React.FC = () => {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const { request } = useGraphQL();
  const [data, setData] = useState<DashboardData | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    const loadData = async () => {
      try {
        setLoading(true);
        const result = await request<DashboardData>(`
          query {
            projects { id name type componentCount gitBranch gitDirty }
            projectsCount
            sessions { id name projectName state messageCount lastActiveAt }
            sessionsCount
          }
        `);
        setData(result);
        setError(null);
      } catch (err) {
        setError(err instanceof Error ? err.message : 'Failed to load dashboard');
      } finally {
        setLoading(false);
      }
    };
    loadData();
  }, [request]);

  const stats = [
    {
      title: t('dashboard.projects'),
      value: data?.projectsCount ?? 0,
      icon: 'folder',
      color: 'primary' as const,
      onClick: () => navigate('/devtrack/projects'),
    },
    {
      title: t('dashboard.sessions'),
      value: data?.sessionsCount ?? 0,
      icon: 'chat',
      color: 'secondary' as const,
      onClick: () => navigate('/devtrack/sessions'),
    },
    {
      title: t('dashboard.activeProcesses'),
      value: data?.sessions?.filter(s => s.state === 'running').length ?? 0,
      icon: 'play_circle',
      color: 'success' as const,
    },
  ];

  if (loading) {
    return (
      <CSDLayoutPage title={t('dashboard.title')}>
        <CSDBox sx={{ p: 3, textAlign: 'center' }}>
          <CSDTypography>{t('common.loading')}</CSDTypography>
        </CSDBox>
      </CSDLayoutPage>
    );
  }

  if (error) {
    return (
      <CSDLayoutPage title={t('dashboard.title')}>
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
    <CSDLayoutPage title={t('dashboard.title')}>
      <CSDStack spacing={3}>
        {/* Stats Grid */}
        <CSDStatsGrid>
          {stats.map((stat, index) => (
            <CSDStatCard
              key={index}
              title={stat.title}
              value={stat.value}
              icon={<CSDIcon>{stat.icon}</CSDIcon>}
              color={stat.color}
              onClick={stat.onClick}
            />
          ))}
        </CSDStatsGrid>

        {/* Recent Sessions */}
        <CSDPaper sx={{ p: 2 }}>
          <CSDStack direction="row" justifyContent="space-between" alignItems="center" sx={{ mb: 2 }}>
            <CSDTypography variant="h6">{t('dashboard.recentActivity')}</CSDTypography>
            <CSDButton size="small" onClick={() => navigate('/devtrack/sessions')}>
              {t('common.actions')}
            </CSDButton>
          </CSDStack>
          {data?.sessions?.slice(0, 5).map((session) => (
            <CSDBox
              key={session.id}
              sx={{
                p: 1.5,
                borderBottom: '1px solid',
                borderColor: 'divider',
                cursor: 'pointer',
                '&:hover': { bgcolor: 'action.hover' },
                '&:last-child': { borderBottom: 'none' },
              }}
              onClick={() => navigate(`/devtrack/sessions/${session.id}`)}
            >
              <CSDStack direction="row" justifyContent="space-between" alignItems="center">
                <CSDStack>
                  <CSDTypography variant="body1" fontWeight="medium">
                    {session.name}
                  </CSDTypography>
                  <CSDTypography variant="body2" color="text.secondary">
                    {session.projectName} - {session.messageCount} messages
                  </CSDTypography>
                </CSDStack>
                <CSDBox
                  sx={{
                    px: 1,
                    py: 0.5,
                    borderRadius: 1,
                    bgcolor: session.state === 'running' ? 'success.light' :
                             session.state === 'waiting' ? 'warning.light' : 'grey.200',
                    color: session.state === 'running' ? 'success.dark' :
                           session.state === 'waiting' ? 'warning.dark' : 'grey.700',
                  }}
                >
                  <CSDTypography variant="caption">
                    {t(`sessions.${session.state}`)}
                  </CSDTypography>
                </CSDBox>
              </CSDStack>
            </CSDBox>
          ))}
          {(!data?.sessions || data.sessions.length === 0) && (
            <CSDTypography color="text.secondary" sx={{ p: 2, textAlign: 'center' }}>
              {t('sessions.noSessions')}
            </CSDTypography>
          )}
        </CSDPaper>
      </CSDStack>
    </CSDLayoutPage>
  );
};

export default DashboardPage;
