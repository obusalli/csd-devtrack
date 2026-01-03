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
import type { Project } from '../../../types';

interface ProjectsData {
  projects: Project[];
  projectsCount: number;
}

const ProjectsPage: React.FC = () => {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const { request } = useGraphQL();
  const [data, setData] = useState<ProjectsData | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    const loadData = async () => {
      try {
        setLoading(true);
        const result = await request<ProjectsData>(`
          query {
            projects { id name type componentCount gitBranch gitDirty runningCount }
            projectsCount
          }
        `);
        setData(result);
        setError(null);
      } catch (err) {
        setError(err instanceof Error ? err.message : 'Failed to load projects');
      } finally {
        setLoading(false);
      }
    };
    loadData();
  }, [request]);

  if (loading) {
    return (
      <CSDLayoutPage title={t('projects.title')}>
        <CSDBox sx={{ p: 3, textAlign: 'center' }}>
          <CSDTypography>{t('common.loading')}</CSDTypography>
        </CSDBox>
      </CSDLayoutPage>
    );
  }

  if (error) {
    return (
      <CSDLayoutPage title={t('projects.title')}>
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
      title={t('projects.title')}
      actions={
        <CSDButton
          variant="contained"
          startIcon={<CSDIcon>add</CSDIcon>}
        >
          {t('projects.addProject')}
        </CSDButton>
      }
    >
      <CSDPaper>
        <CSDTable>
          <CSDTableHead>
            <CSDTableRow>
              <CSDTableCell>{t('projects.name')}</CSDTableCell>
              <CSDTableCell>{t('projects.type')}</CSDTableCell>
              <CSDTableCell>{t('projects.components')}</CSDTableCell>
              <CSDTableCell>{t('projects.branch')}</CSDTableCell>
              <CSDTableCell>{t('projects.status')}</CSDTableCell>
              <CSDTableCell align="right">{t('common.actions')}</CSDTableCell>
            </CSDTableRow>
          </CSDTableHead>
          <CSDTableBody>
            {data?.projects?.map((project) => (
              <CSDTableRow
                key={project.id}
                hover
                sx={{ cursor: 'pointer' }}
                onClick={() => navigate(`/devtrack/projects/${project.id}`)}
              >
                <CSDTableCell>
                  <CSDStack direction="row" alignItems="center" spacing={1}>
                    <CSDIcon color="primary">folder</CSDIcon>
                    <CSDTypography fontWeight="medium">{project.name}</CSDTypography>
                  </CSDStack>
                </CSDTableCell>
                <CSDTableCell>{project.type}</CSDTableCell>
                <CSDTableCell>{project.componentCount}</CSDTableCell>
                <CSDTableCell>
                  <CSDStack direction="row" alignItems="center" spacing={1}>
                    <CSDIcon fontSize="small">git_branch</CSDIcon>
                    <CSDTypography variant="body2">{project.gitBranch || '-'}</CSDTypography>
                  </CSDStack>
                </CSDTableCell>
                <CSDTableCell>
                  <CSDChip
                    size="small"
                    label={project.gitDirty ? t('projects.dirty') : t('projects.clean')}
                    color={project.gitDirty ? 'warning' : 'success'}
                  />
                </CSDTableCell>
                <CSDTableCell align="right">
                  <CSDButton size="small" onClick={(e) => { e.stopPropagation(); }}>
                    <CSDIcon>more_vert</CSDIcon>
                  </CSDButton>
                </CSDTableCell>
              </CSDTableRow>
            ))}
          </CSDTableBody>
        </CSDTable>
        {(!data?.projects || data.projects.length === 0) && (
          <CSDBox sx={{ p: 4, textAlign: 'center' }}>
            <CSDTypography color="text.secondary">{t('projects.noProjects')}</CSDTypography>
          </CSDBox>
        )}
      </CSDPaper>
    </CSDLayoutPage>
  );
};

export default ProjectsPage;
